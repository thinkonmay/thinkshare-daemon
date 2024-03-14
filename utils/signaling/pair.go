package signaling

import (
	"encoding/json"
	"fmt"

	"github.com/thinkonmay/thinkremote-rtchub/signalling/gRPC/packet"
	"github.com/thinkonmay/thinkshare-daemon/utils/log"
	"github.com/thinkonmay/thinkshare-daemon/utils/signaling/protocol"
)

type Pair struct {
	A protocol.Tenant
	B protocol.Tenant
}

func (pair *Pair) handlePair() {
	log.PushLog("new pair")
	pair.B.Send(&packet.SignalingMessage{
		Type: packet.SignalingType_tSTART,
		Sdp:  nil,
		Ice:  nil,
	})
	pair.A.Send(&packet.SignalingMessage{
		Type: packet.SignalingType_tSTART,
		Sdp:  nil,
		Ice:  nil,
	})

	log.PushLog("trigger done")
	stop := make(chan bool, 2)
	go func() {
		for {
			msg := pair.B.Receive()
			if pair.A.IsExited() || pair.B.IsExited() || msg == nil {
				stop <- true
				return
			}

			bytes, _ := json.Marshal(msg)
			fmt.Printf("sending packet from peerB to peerA : %s \n", string(bytes))
			pair.A.Send(msg)
		}
	}()
	go func() {
		for {
			msg := pair.A.Receive()
			if pair.A.IsExited() || pair.B.IsExited() || msg == nil {
				stop <- true
				return
			}

			bytes, _ := json.Marshal(msg)
			fmt.Printf("sending packet from peerA to peerB : %s \n", string(bytes))
			pair.B.Send(msg)
		}
	}()
	go func() {
		<-stop
		log.PushLog("pair exited")
		if !pair.A.IsExited() {
			pair.A.Exit()
		}
		if !pair.B.IsExited() {
			pair.B.Exit()
		}
	}()

}
