package httpp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/thinkonmay/thinkshare-daemon/persistent/gRPC/packet"
	"github.com/thinkonmay/thinkshare-daemon/utils/app"
	"github.com/thinkonmay/thinkshare-daemon/utils/libvirt"
	"github.com/thinkonmay/thinkshare-daemon/utils/log"
)

const (
	_new_timeout = 5       // second
	_use_timeout = 10 * 60 // minutes
)

var (
	now = func() int { return int(time.Now().Unix()) }
)

type deployment struct {
	cancel               chan bool
	starttime, timestamp int
}
type GRPCclient struct {
	logger          []string
	worker_info     func() *packet.WorkerInfor
	recv_session    func(*packet.WorkerSession, chan bool, chan bool) (*packet.WorkerSession, error)
	closed_sesssion func(*packet.WorkerSession) error
	Mux             *http.ServeMux

	pending    map[string]*deployment
	keepalives map[string]*deployment

	mut *sync.Mutex

	done bool
}

func InitHttppServer() (ret *GRPCclient, err error) {
	ret = &GRPCclient{
		done:       false,
		pending:    map[string]*deployment{},
		keepalives: map[string]*deployment{},
		mut:        &sync.Mutex{},
		Mux:        http.NewServeMux(),

		logger: []string{},
		worker_info: func() *packet.WorkerInfor {
			return &packet.WorkerInfor{}
		},

		recv_session: func(*packet.WorkerSession, chan bool, chan bool) (*packet.WorkerSession, error) {
			return nil, fmt.Errorf("handler not configured")
		},
		closed_sesssion: func(ws *packet.WorkerSession) error {
			return fmt.Errorf("handler not configured")
		},
	}
	get_current_position := func(id string) int {
		type pos = struct {
			*deployment
			id string
		}

		order := []pos{}
		ret.mut.Lock()
		defer ret.mut.Unlock()
		for k, v := range ret.pending {
			order = append(order, pos{
				deployment: v,
				id:         k,
			})
		}

		slices.SortFunc(order, func(a pos, b pos) int {
			return a.starttime - b.starttime
		})

		for i, v := range order {
			if v.id == id {
				return i
			}
		}

		return -1
	}

	ret.wrapper("ping",
		func(conn string) ([]byte, error) {
			return []byte("pong"), nil
		})
	ret.wrapper("info",
		func(conn string) ([]byte, error) {
			return json.Marshal(ret.worker_info())
		})
	ret.wrapper("log",
		func(conn string) ([]byte, error) {
			return []byte(strings.Join(ret.logger, "\n")), nil
		})
	ret.wrapper("_log",
		func(conn string) ([]byte, error) {
			log.PushLog(conn)
			return []byte{}, nil
		})
	ret.wrapper("_used",
		func(conn string) ([]byte, error) {
			msg := ""
			if err := json.Unmarshal([]byte(conn), &msg); err != nil {
				return nil, err
			}

			ret.mut.Lock()
			keepalive, found := ret.keepalives[msg]
			ret.mut.Unlock()
			if !found {
				return nil, fmt.Errorf("_used session not found")
			}

			log.PushLog("receive _used signal for session %s", msg)
			keepalive.timestamp = now() - _use_timeout
			return []byte("{}"), nil
		})
	ret.wrapper("_use",
		func(conn string) ([]byte, error) {
			msg := ""
			if err := json.Unmarshal([]byte(conn), &msg); err != nil {
				return nil, err
			}

			ret.mut.Lock()
			keepalive, found := ret.keepalives[msg]
			ret.mut.Unlock()
			if !found {
				return nil, fmt.Errorf("_use session not found")
			}

			keepalive.timestamp = now()
			return []byte("{}"), nil
		})
	ret.wrapper("_new",
		func(conn string) ([]byte, error) {
			msg := &packet.WorkerSession{}
			if err := json.Unmarshal([]byte(conn), msg); err != nil {
				return nil, err
			}

			ret.mut.Lock()
			deployment, found := ret.pending[msg.Id]
			ret.mut.Unlock()
			if !found {
				return nil, fmt.Errorf("_new session not found")
			}

			deployment.timestamp = now()
			return json.Marshal(map[string]int{"position": get_current_position(msg.Id)})
		})
	ret.wrapper("new",
		func(conn string) ([]byte, error) {
			msg := &packet.WorkerSession{}
			if err := json.Unmarshal([]byte(conn), msg); err != nil {
				return nil, err
			}

			deployment, keepalive :=
				&deployment{
					cancel:    make(chan bool, 4096),
					timestamp: now(),
					starttime: now(),
				}, &deployment{
					cancel:    make(chan bool, 4096),
					timestamp: now(),
					starttime: now(),
				}

			keepaliveid := app.GetKeepaliveID(msg)
			ret.mut.Lock()
			ret.pending[msg.Id] = deployment
			ret.keepalives[keepaliveid] = keepalive
			ret.mut.Unlock()
			running := true
			defer func() {
				running = false
				deployment.cancel <- true
				ret.mut.Lock()
				delete(ret.pending, msg.Id)
				ret.mut.Unlock()
			}()
			go func() {
				for running {
					time.Sleep(time.Second)
					if now()-deployment.timestamp > _new_timeout {
						deployment.cancel <- true
						return
					}
				}
			}()
			go func() {
				for {
					time.Sleep(time.Minute)
					timeout := now() - keepalive.timestamp
					if timeout > _use_timeout {
						keepalive.cancel <- true
						return
					}
				}
			}()

			if resp, err := ret.recv_session(msg, deployment.cancel, keepalive.cancel); err == nil {
				b, _ := json.Marshal(resp)
				return b, nil
			} else {
				return nil, err
			}
		})
	ret.wrapper("closed",
		func(conn string) ([]byte, error) {
			msg := &packet.WorkerSession{}
			if err = json.Unmarshal([]byte(conn), msg); err != nil {
				return nil, err
			}

			return []byte("{}"), ret.closed_sesssion(msg)
		})

	exe, _ := os.Executable()
	path, _ := filepath.Abs(filepath.Dir(exe))
	fileserver := http.FileServerFS(os.DirFS(path))
	ret.Mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.PushLog("receive %s request on file %s", r.Method, r.URL.Path)
		if r.Method == "GET" {
			fileserver.ServeHTTP(w, r)
			return
		}

		result := []byte("success")
		err := (error)(nil)
		defer func() {
			if err != nil {
				w.WriteHeader(400)
				w.Write([]byte(err.Error()))
			} else {
				w.WriteHeader(200)
				w.Write(result)
			}
		}()
		if r.Method == "DELETE" {
			err = os.Remove(fmt.Sprintf("%s%s", path, r.URL.Path))
		} else if r.Method == "PUT" {
			temppath := fmt.Sprintf("%s/temp%s", path, r.URL.Path)
			if err = os.MkdirAll(filepath.Dir(temppath), 0777); err != nil {
			} else if result, err = exec.Command("mv",
				fmt.Sprintf("%s%s", path, r.URL.Path),
				temppath,
			).CombinedOutput(); err != nil {
				err = fmt.Errorf("%s", string(result))
			} else {
				result = []byte(fmt.Sprintf("/temp%s", r.URL.Path))
			}
		} else if r.Method == "POST" {
			if result, err = exec.Command("qemu-img",
				"info", fmt.Sprintf("%s%s", path, r.URL.Path),
				"--output", "json",
			).CombinedOutput(); err != nil {
				err = fmt.Errorf("%s", string(result))
			}
		}
	})
	ret.wrapper("allocate",
		func(conn string) ([]byte, error) {
			Allocate := struct {
				ID   string `json:"id"`
				Base string `json:"base"`
				Size int    `json:"size"`
			}{}

			if err = json.Unmarshal([]byte(conn), &Allocate); err != nil {
				return nil, err
			} else if err = uuid.Validate(Allocate.ID); err != nil {
				return nil, err
			} else if Allocate.Base == "os" {
				if err = libvirt.
					NewVolume(fmt.Sprintf("%s/%s.qcow2", path, Allocate.Base)).
					PushChainID(Allocate.ID, Allocate.Size); err != nil {
					return nil, err
				}
			} else if _, err := os.Stat(fmt.Sprintf("%s/%s.qcow2", path, Allocate.Base)); err == nil {
				if result, err := exec.Command("cp",
					fmt.Sprintf("%s/%s.qcow2", path, Allocate.Base),
					fmt.Sprintf("%s/child/%s.qcow2", path, Allocate.ID)).CombinedOutput(); err != nil {
					return nil, fmt.Errorf("%s", string(result))
				}
			} else {
				return nil, err
			}
			return []byte("success"), nil
		})
	ret.wrapper("import",
		func(conn string) ([]byte, error) {
			msg := &struct {
				From string `json:"from"`
				Path string `json:"path"`
			}{}

			if err = json.Unmarshal([]byte(conn), msg); err != nil {
				return nil, err
			}
			var targeturl *url.URL
			if exist, err := exists(msg.Path); err != nil {
				return nil, err
			} else if exist {
				return nil, fmt.Errorf("file path exist %s", msg.Path)
			} else if targeturl, err = url.Parse(msg.From); err != nil {
				return nil, err
			}

			req, err := http.NewRequest("PUT", msg.From, bytes.NewReader([]byte{}))
			if err != nil {
				return nil, err
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				return nil, err
			} else if body, err := io.ReadAll(resp.Body); err != nil {
				return nil, err
			} else if resp.StatusCode != 200 {
				return nil, fmt.Errorf("%s", string(body))
			} else {
				targeturl.Path = string(body)
			}

			_tempurl := targeturl.String()
			tempfile := fmt.Sprintf("%s/%s", os.TempDir(), uuid.NewString())
			defer os.Remove(tempfile)
			result, err := exec.Command("curl",
				"--progress-bar", "--limit-rate", "60M",
				"-o", tempfile,
				_tempurl).CombinedOutput()
			if err != nil {
				return nil, fmt.Errorf("failed to download file %s", string(result))
			}

			result, err = exec.Command("mv",
				tempfile,
				fmt.Sprintf("%s%s", path, msg.Path)).CombinedOutput()
			if err != nil {
				return nil, fmt.Errorf("failed to move file %s", string(result))
			}

			req, err = http.NewRequest("DELETE", _tempurl, bytes.NewReader([]byte{}))
			if err != nil {
				return nil, err
			}
			http.DefaultClient.Do(req)
			return []byte("success"), nil
		})

	return ret, nil
}

func (ret *GRPCclient) wrapper(url string, fun func(content string) ([]byte, error)) {
	log.PushLog("registering url handler on %s", url)
	ret.Mux.HandleFunc("/"+url, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "*")
		if r.Method == "OPTIONS" {
			return
		}

		b, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(503)
			w.Write([]byte(err.Error()))
			return
		}

		resp, err := fun(string(b))
		if err != nil {
			log.PushLog("request failed %s %s => %s", url, string(b), err.Error())
			w.WriteHeader(503)
			w.Write([]byte(err.Error()))
			return
		}

		w.WriteHeader(200)
		w.Write(resp)
	})
}

func (client *GRPCclient) Stop() {
	client.done = true
}

func (grpc *GRPCclient) Log(source string, level string, log string) {
	grpc.logger = append(grpc.logger, fmt.Sprintf("%s %s %s: %s", time.Now().Format(time.DateTime), source, level, log))
}

func (grpc *GRPCclient) Infor(fun func() *packet.WorkerInfor) {
	grpc.worker_info = fun
}
func (grpc *GRPCclient) RecvSession(fun func(*packet.WorkerSession, chan bool, chan bool) (*packet.WorkerSession, error)) {
	grpc.recv_session = fun
}
func (grpc *GRPCclient) ClosedSession(fun func(*packet.WorkerSession) error) {
	grpc.closed_sesssion = fun
}

func exists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}
