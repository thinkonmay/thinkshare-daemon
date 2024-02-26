package childprocess

import (
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/thinkonmay/thinkshare-daemon/utils/log"
)

type ProcessID int64
func (id ProcessID)Valid()bool{
	return id > 0
}

const (
	InvalidProcID = -1
	NullProcID = -2
)


type ProcessLog struct {
	ID ProcessID 
	Log string
	LogType string
}


type ChildProcess struct {
	cmd *exec.Cmd
	force_closed bool
}
type ChildProcesses struct {
	mutex sync.Mutex
	procs map[ProcessID]*ChildProcess


	LogChan chan ProcessLog
}

func NewChildProcessSystem() *ChildProcesses {
	ret := ChildProcesses{
		LogChan: make(chan ProcessLog,1024),
		procs: make(map[ProcessID]*ChildProcess),
		mutex: sync.Mutex{},
	}

	return &ret
}



func (procs *ChildProcesses) NewChildProcess(cmd *exec.Cmd, hidewnd bool) (ProcessID,error) {
	procs.mutex.Lock()
	defer procs.mutex.Unlock()

	if cmd == nil {
		return InvalidProcID,fmt.Errorf("nil cmd input")
	}

	id := ProcessID(time.Now().UnixMilli())
	procs.procs[id] = &ChildProcess{
		cmd:      cmd,
	}

	log.PushLog("process %s, process id %d booting up", cmd.Args[0], int(id))
	procs.handleProcess(id,hidewnd)
	go func ()  {
		for {
			procs.WaitID(id);
			if procs.procs[id].force_closed {
				log.PushLog("process id %d closed",id)
				return
			}

			log.PushLog("process id %d closed, revoking",id)
			procs.procs[id] = &ChildProcess{
				cmd:      exec.Command(cmd.Args[0],cmd.Args[1:]...),
				force_closed: false,
			}
			procs.handleProcess(id,hidewnd)
		}
	}()
	return id,nil
}

func (procs *ChildProcesses) CloseAll() {
	for id,_ := range procs.procs {
		procs.CloseID(id);
	}
}

func (procs *ChildProcesses) CloseID(ID ProcessID) error {
	proc,err := procs.filterprocess(ID)
	if err != nil {
		return err
	}
	log.PushLog("force terminate process name %s, process id %d", proc.cmd.Args[0], int(ID))
	proc.force_closed = true
	return proc.cmd.Process.Kill()
}

func (procs *ChildProcesses) WaitID(ID ProcessID) error {
	proc,err := procs.filterprocess(ID)
	if err != nil {
		return err
	}
	proc.cmd.Process.Wait()
	return nil;
}

func (procs *ChildProcesses) RevokeID(ID ProcessID) error {
	proc,err := procs.filterprocess(ID)
	if err != nil {
		return err
	}
	proc.cmd.Process.Wait()
	return nil;
}

func (procs *ChildProcesses)filterprocess(ID ProcessID) (*ChildProcess,error) {
	procs.mutex.Lock()
	proc := procs.procs[ID]
	procs.mutex.Unlock()

	if proc == nil {
		return nil,fmt.Errorf("no such ProcessID")
	} else if proc.cmd == nil{
		return nil,fmt.Errorf("attempting to wait null process")
	} else if proc.cmd.Process == nil {
		return nil,fmt.Errorf("attempting to wait null process")
	}

	return proc,nil
}
















func (procs *ChildProcesses) handleProcess(id ProcessID, hidewnd bool) {
	proc := procs.procs[id]
	if proc == nil {
		return
	}


	processname := proc.cmd.Args[0]
	stdoutIn, _ := proc.cmd.StdoutPipe()
	stderrIn, _ := proc.cmd.StderrPipe()
	
	log.PushLog("starting %s : %s", processname, strings.Join(proc.cmd.Args, " "))
	proc.cmd.SysProcAttr = &syscall.SysProcAttr{ HideWindow: hidewnd, }
	err := proc.cmd.Start()
	if err != nil {
		log.PushLog("error init process %s", err.Error())
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
			return
		}

		if n < 1 {
			continue
		}

		lines := strings.Split(string(buf[:n]),"\n")
		for _,line := range lines {
			sublines := strings.Split(line,"\r")
			for _,subline := range sublines {
				if len(subline) == 0 {
					continue
				}

				procs.LogChan <- ProcessLog{
					Log: subline,
					LogType: logtype,
					ID: id,
				}
			}
		}
	}
}
