package signaling

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/thinkonmay/thinkshare-daemon/utils/signaling/protocol"
	"github.com/thinkonmay/thinkshare-daemon/utils/signaling/validator"
)

type Signalling struct {
	waitLine map[string]protocol.Tenant
	mut      *sync.Mutex

	handlers  []protocol.ProtocolHandler
	validator validator.Validator
}

func InitSignallingServer(handlers []protocol.ProtocolHandler,
	provider validator.Validator,
) *Signalling {
	signaling := Signalling{
		waitLine:  make(map[string]protocol.Tenant),
		mut:       &sync.Mutex{},
		validator: provider,
		handlers:  handlers,
	}

	go func() { // remove exited tenant from waiting like
		for {
			var rev []string
			signaling.mut.Lock()
			for index, wait := range signaling.waitLine {
				if wait.IsExited() {
					fmt.Printf("tenant exited\n")
					rev = append(rev, index)
				}
			}
			signaling.mut.Unlock()
			for _, i := range rev {
				signaling.removeTenant(i)
			}
			time.Sleep(100 * time.Millisecond)
		}
	}()
	go func() { // discard message from waiting like
		for {
			time.Sleep(100 * time.Millisecond)
			signaling.mut.Lock()
			for _, t := range signaling.waitLine {
				if t.Peek() {
					bytes, _ := json.Marshal(t.Receive())
					fmt.Printf("discarded packet from waiting tenant: %s \n", string(bytes))
				}
			}
			signaling.mut.Unlock()
		}
	}()

	for _, handler := range signaling.handlers { // handle new tenant
		handler.OnTenant(func(token string, tent protocol.Tenant) error {
			signaling.addTenant(token, tent) // add tenant to queue

			// get all keys from current waiting line
			keys := make([]string, 0, len(signaling.waitLine))
			for k := range signaling.waitLine {
				keys = append(keys, k)
			}

			// validate every tenant in queue
			pairs, new_queue := signaling.validator.Validate(keys)

			// move tenant from waiting line to pair queue
			for _, v := range pairs {
				pair := Pair{A: nil, B: nil}
				for _, v2 := range keys {
					if v2 == v.PeerA && pair.B == nil {
						pair.B = signaling.waitLine[v2]
					} else if v2 == v.PeerB && pair.A == nil {
						pair.A = signaling.waitLine[v2]
					}
				}

				if pair.A == nil || pair.B == nil {
					continue
				}

				pair.handlePair()
			}

			// remove tenant in old queue if not exist in new queue
			for _, k := range keys {
				rm := true
				for _, n := range new_queue {
					if n == k {
						rm = false
					}
				}

				if rm {
					signaling.removeTenant(k)
				}
			}

			return nil
		})
	}

	return &signaling
}
