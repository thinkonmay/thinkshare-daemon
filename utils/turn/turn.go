package turn

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/pion/turn/v4"
)

const (
	realm = "thinkmay.net"
)

type TurnServerConfig struct {
	Path string `json:"path"`
}
type TurnServer struct {
	sessions map[string]*TurnSession
	Mux      *http.ServeMux
	mut      *sync.Mutex
	sync     bool
}
type TurnRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	PublicIP string `json:"publicip"`
	Port     int    `json:"port"`
	MaxPort  int    `json:"max"`
	MinPort  int    `json:"min"`
}
type TurnSession struct {
	TurnRequest
	server *turn.Server
}

func NewTurnServer(config TurnServerConfig) (*TurnServer, error) {
	server := &TurnServer{
		sessions: map[string]*TurnSession{},
		mut:      &sync.Mutex{},
		sync:     true,
	}

	go func() {
		_sessions := []TurnRequest{}
		if bytes, err := os.ReadFile(config.Path); err == nil {
			json.Unmarshal(bytes, _sessions)

			for _, ss := range _sessions {
				ts, err := setupTurn(ss)
				if err != nil {
					continue
				}

				server.sessions[ss.Username] = &TurnSession{
					TurnRequest: ss,
					server:      ts,
				}
			}
		}

		for server.sync {
			time.Sleep(time.Second)

			_sessions = []TurnRequest{}
			server.mut.Lock()
			for _, ss := range server.sessions {
				_sessions = append(_sessions, ss.TurnRequest)
			}
			server.mut.Unlock()

			bytes, err := json.MarshalIndent(_sessions, "", "	")
			if err == nil {
				os.WriteFile(config.Path, bytes, 777)
			}
		}
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("/open", func(w http.ResponseWriter, r *http.Request) {
		data, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(400)
			w.Write([]byte(err.Error()))
		}

		body := TurnRequest{}
		if err := json.Unmarshal(data, &body); err != nil {
			w.WriteHeader(400)
			w.Write([]byte(err.Error()))
			return
		}

		server.mut.Lock()
		defer server.mut.Unlock()

		ts, err := setupTurn(body)
		if err != nil {
			w.WriteHeader(400)
			w.Write([]byte(err.Error()))
			return
		}

		fmt.Printf("Started a new turn server %v\n",body)
		server.sessions[body.Username] = &TurnSession{
			TurnRequest: body,
			server:      ts,
		}

		w.WriteHeader(200)
		w.Write([]byte("success"))
	})

	mux.HandleFunc("/close", func(w http.ResponseWriter, r *http.Request) {
		data, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(400)
			w.Write([]byte(err.Error()))
			return
		}

		body := struct {
			Username string `json:"username"`
		}{}
		if err := json.Unmarshal(data, &body); err != nil {
			w.WriteHeader(400)
			w.Write([]byte(err.Error()))
			return
		}

		server.mut.Lock()
		defer server.mut.Unlock()

		if session, ok := server.sessions[body.Username]; ok {
			session.server.Close()
			w.WriteHeader(200)
			w.Write([]byte("success"))
		} else {
			w.WriteHeader(404)
			w.Write([]byte(fmt.Sprintf("username %s not found")))
		}
	})

	server.Mux = mux
	return server, nil
}

type TurnClient struct {
	Addr string
}

func (client *TurnClient) Open(req TurnRequest) error {
	data, err := json.Marshal(req)
	if err != nil {
		return err
	}

	resp, err := http.Post(fmt.Sprintf("http://%s/new", client.Addr), "application/json", bytes.NewReader(data))
	if err != nil {
		return err
	} else if resp.StatusCode != 200 {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}

		return fmt.Errorf(string(body))
	}

	return nil
}

func (client *TurnClient) Close(username string) error {
	data, err := json.Marshal(TurnRequest{
		Username: username,
	})
	if err != nil {
		return err
	}

	resp, err := http.Post(fmt.Sprintf("http://%s/new", client.Addr), "application/json", bytes.NewReader(data))
	if err != nil {
		return err
	} else if resp.StatusCode != 200 {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}

		return fmt.Errorf(string(body))
	}

	return nil
}

func setupTurn(req TurnRequest) (t *turn.Server, err error) {
	udpListener, err := net.ListenPacket("udp4", "0.0.0.0:"+strconv.Itoa(req.Port))
	if err != nil {
		return nil, fmt.Errorf("Failed to create TURN server listener: %s", err)
	}

	usersMap := map[string][]byte{}
	usersMap[req.Username] = turn.GenerateAuthKey(req.Username, realm, req.Password)
	return turn.NewServer(turn.ServerConfig{
		Realm: realm,
		AuthHandler: func(username string, realm string, srcAddr net.Addr) ([]byte, bool) {
			if key, ok := usersMap[username]; ok {
				return key, true
			}
			return nil, false
		},
		PacketConnConfigs: []turn.PacketConnConfig{
			{
				PacketConn: udpListener,
				RelayAddressGenerator: &turn.RelayAddressGeneratorPortRange{
					RelayAddress: net.ParseIP(req.PublicIP), // Claim that we are listening on IP passed by user (This should be your Public IP)
					Address:      "0.0.0.0",                 // But actually be listening on every interface
					MinPort:      uint16(req.MinPort),
					MaxPort:      uint16(req.MaxPort),
				},
			},
		},
	})
}
