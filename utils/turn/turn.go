package turn

import (
	"os"
	"os/exec"

	"github.com/thinkonmay/thinkshare-daemon/utils/log"
)

type TurnServer struct {
    proc *os.Process
}

func SetupTurn() *TurnServer{
    turn := TurnServer{}
    go func() {
        for {
            cmd := exec.Command("./turn.exe")
            err := cmd.Start()
            if err != nil {
                log.PushLog("failed to start turn server %s",err.Error())
                return
            }

            turn.proc = cmd.Process
            turn.proc.Wait()
            turn.proc = nil
        }
    }()

    return &turn
}


func (turn *TurnServer)CloseTurn() {
    if turn.proc == nil {
        return
    }

    turn.proc.Kill()
}