package ws

import (
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/thinkonmay/thinkremote-rtchub/signalling/gRPC/packet"
	"github.com/thinkonmay/thinkshare-daemon/utils/log"
	"github.com/thinkonmay/thinkshare-daemon/utils/signaling/protocol"
)

type WebSocketServer struct {
	fun  protocol.OnTenantFunc
	auth func(*http.Request) bool
	path string

	mapid map[string]*HttpTenant
	mut   *sync.Mutex
}

func (server *WebSocketServer) OnTenant(fun protocol.OnTenantFunc) {
	server.fun = fun
}

func (wsserver *WebSocketServer) HandleHttpSignaling(w http.ResponseWriter, r *http.Request) {
	if !wsserver.auth(r) {
		w.WriteHeader(401)
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
		log.PushLog("New signaling tenant %s", token)
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
				log.PushLog("error authenticate session")
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

func InitSignallingHttp(path string, auth func(*http.Request) bool) *WebSocketServer {
	wsserver := &WebSocketServer{
		mapid: map[string]*HttpTenant{},
		fun:  func(protocol.Tenant) error { return nil },
		auth: auth,
		path: path,
		mut:  &sync.Mutex{},
	}
	http.HandleFunc(path, wsserver.HandleHttpSignaling)
	return wsserver
}
