package childprocess

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/thinkonmay/thinkshare-daemon/utils/log"
)

type ProcessID int64

type ProcessLog struct {
	ID ProcessID 
	Log string
	LogType string
}


type ChildProcess struct {
	cmd *exec.Cmd
}
type ChildProcesses struct {
	mutex sync.Mutex
	procs map[ProcessID]*ChildProcess


	LogChan chan ProcessLog
}

func NewChildProcessSystem() *ChildProcesses {
	ret := ChildProcesses{
		procs: make(map[ProcessID]*ChildProcess),
		mutex: sync.Mutex{},
	}

	stdinSelf := os.Stdin
	go func() {
		for {
			time.Sleep(1 * time.Second)
			_, err := stdinSelf.Write([]byte("\n"))
			if err != nil {
				return
			}
		}
	}()

	return &ret
}



func (procs *ChildProcesses) NewChildProcess(cmd *exec.Cmd) (ProcessID,error) {
	procs.mutex.Lock()
	defer procs.mutex.Unlock()

	if cmd == nil {
		return -1,fmt.Errorf("nil cmd input")
	}

	id := ProcessID(time.Now().UnixMilli())
	procs.procs[id] = &ChildProcess{
		cmd:      cmd,
	}

	log.PushLog("process %s, process id %d booting up\n", cmd.Args[0], int(id))
	procs.handleProcess(id)
	return id,nil
}

func (procs *ChildProcesses) CloseAll() {
	procs.mutex.Lock()
	defer procs.mutex.Unlock()

	for id,_ := range procs.procs {
		procs.CloseID(id);
	}
}

func (procs *ChildProcesses) CloseID(ID ProcessID) error {
	procs.mutex.Lock()
	defer procs.mutex.Unlock()

	proc := procs.procs[ID]
	if proc == nil {
		return fmt.Errorf("no such ProcessID")
	}

	log.PushLog("force terminate process name %s, process id %d \n", proc.cmd.Args[0], int(ID))
	return proc.cmd.Process.Kill()
}

func (procs *ChildProcesses) WaitID(ID ProcessID) error {
	procs.mutex.Lock()
	defer procs.mutex.Unlock()

	proc := procs.procs[ID]
	if proc == nil {
		return fmt.Errorf("no such ProcessID")
	}

	proc.cmd.Process.Wait()
	return nil;
}

















func (procs *ChildProcesses) handleProcess(id ProcessID) {
	proc := procs.procs[id]
	if proc == nil {
		return
	}


	processname := proc.cmd.Args[0]
	stdoutIn, _ := proc.cmd.StdoutPipe()
	stderrIn, _ := proc.cmd.StderrPipe()
	
	log.PushLog("starting %s : %s\n", processname, strings.Join(proc.cmd.Args, " "))
	proc.cmd.SysProcAttr.HideWindow = true;
	err := proc.cmd.Start()
	if err != nil {
		log.PushLog("error init process %s\n", err.Error())
		return
	}

	go procs.copyAndCapture(id,"stdout" ,stdoutIn)
	go procs.copyAndCapture(id,"stderr" ,stderrIn)
}


func (procs *ChildProcesses) copyAndCapture(id ProcessID, logtype string, r io.Reader) {
	buf := make([]byte, 1024)
	for {
		n, err := r.Read(buf[:])
		if err != nil {
			// Read returns io.EOF at the end of file, which is not an error for us
			if err == io.EOF {
				err = nil
			}
			return
		}

		if n < 1 {
			continue
		}
		lines := strings.Split(string(buf[:n]),"\n")
		for _,line := range lines {
			procs.LogChan <- ProcessLog{
				Log: line,
				LogType: logtype,
				ID: id,
			}
		}
	}
}
