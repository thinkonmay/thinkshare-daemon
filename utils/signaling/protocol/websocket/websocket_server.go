package ws

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/thinkonmay/thinkremote-rtchub/signalling/gRPC/packet"
	"github.com/thinkonmay/thinkshare-daemon/utils/signaling/protocol"
)

type WebSocketServer struct {
	fun  protocol.OnTenantFunc
	auth func(string) (*string, bool)
	path string

	mapid map[string]*HttpTenant
	mut   *sync.Mutex
}

func (server *WebSocketServer) OnTenant(fun protocol.OnTenantFunc) {
	server.fun = fun
}
func (server *WebSocketServer) HandleForward(w http.ResponseWriter, r *http.Request) bool {
	target := r.URL.Query().Get("target")
	ip, remove_target := server.auth(target)
	if target == "" || ip == nil {
		return false
	}

	q := r.URL.Query()
	if remove_target {
		q.Del("target")
	}

	pre, err := url.Parse(*ip)
	if err != nil {
		return false
	}

	clone := url.URL{
		Scheme:   pre.Scheme,
		Host:     pre.Host,
		Path:     r.URL.Path,
		RawQuery: q.Encode(),
	}
	req, err := http.NewRequest(r.Method, clone.String(), r.Body)
	if err != nil {
		w.WriteHeader(500)
		w.Write([]byte(err.Error()))
		return true
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		w.WriteHeader(500)
		w.Write([]byte(err.Error()))
		return true
	}

	for k, v := range resp.Header {
		if len(v) == 0 {
			continue
		}
		w.Header().Add(k, v[0])
	}

	b, _ := io.ReadAll(resp.Body)
	w.Write(b)
	return true
}

func (wsserver *WebSocketServer) HandleHttpSignaling(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "*")
	if r.Method == "OPTIONS" {
		return
	}

	if wsserver.HandleForward(w, r) {
		return
	}

	uniqueid := r.URL.Query().Get("uniqueid")
	token := r.URL.Query().Get("token")

	found := false
	wsserver.mut.Lock()
	for k := range wsserver.mapid {
		if k == uniqueid {
			found = true
		}
	}
	wsserver.mut.Unlock()

	if !found {
		tenant := NewWsTenant(uniqueid)
		wsserver.mut.Lock()
		wsserver.mapid[uniqueid] = tenant
		wsserver.mut.Unlock()
		go func() {
			defer func() {
				wsserver.mut.Lock()
				delete(wsserver.mapid, uniqueid)
				wsserver.mut.Unlock()
			}()

			err := wsserver.fun(protocol.Tenant{tenant, token})
			if err != nil {
				tenant.Exit()
				return
			}
			for {
				if tenant.IsExited() {
					break
				}
				time.Sleep(time.Millisecond * 100)
			}
		}()
	} else {
		wsserver.mut.Lock()
		tenant := wsserver.mapid[uniqueid]
		wsserver.mut.Unlock()

		b, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(400)
			w.Write([]byte(err.Error()))
			return
		}

		data := []packet.SignalingMessage{}
		err = json.Unmarshal(b, &data)
		if err != nil {
			w.WriteHeader(400)
			w.Write([]byte(err.Error()))
			return
		}

		if tenant.IsExited() {
			w.WriteHeader(400)
			w.Write([]byte("uniqueid not found"))
			return
		}

		for _, sm := range data {
			tenant.Incoming <- &sm
		}

		data = []packet.SignalingMessage{}
		for {
			if len(tenant.Outcoming) == 0 {
				break
			}
			out := <-tenant.Outcoming
			data = append(data, *out)
		}

		b, err = json.Marshal(data)
		if err != nil {
			w.WriteHeader(400)
			w.Write([]byte(err.Error()))
			return
		}

		w.WriteHeader(200)
		w.Write(b)
		return
	}

	w.WriteHeader(200)
	b, _ := json.Marshal([]packet.SignalingMessage{})
	w.Write(b)
}

func InitSignallingHttp(mux *http.ServeMux,path string) *WebSocketServer {
	wsserver := &WebSocketServer{
		mapid: map[string]*HttpTenant{},
		fun:   func(protocol.Tenant) error { return nil },
		auth:  func(s string) (*string, bool) { return nil, false },
		path:  path,
		mut:   &sync.Mutex{},
	}
	mux.HandleFunc(path, wsserver.HandleHttpSignaling)
	return wsserver
}

func (server *WebSocketServer) AuthHandler(auth func(string) (*string, bool)) {
	server.auth = auth
}
