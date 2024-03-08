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
	InvalidProcID = ProcessID(-1)
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
	start time.Time
}
type ChildProcesses struct {
	mutex sync.Mutex
	procs map[ProcessID]*ChildProcess


	logger func (process,log string)  
}

func NewChildProcessSystem(fun func(process,log string)) *ChildProcesses {
	ret := ChildProcesses{
		logger: fun,
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
	proc := &ChildProcess{
		cmd:      cmd,
		start: time.Now(),
	}

	log.PushLog("process %s, process id %d booting up", cmd.Args[0], int(id))
	procs.procs[id] = proc
	err := procs.handleProcess(id,hidewnd)
	if err != nil {
		return InvalidProcID,err
	}

	go func ()  {
		for {
			procs.WaitID(id);
			if procs.procs[id].force_closed {
				log.PushLog("process id %d closed",id)
				return
			}

			log.PushLog("process id %d closed, revoking",id)
			procs.procs[id].cmd = exec.Command(cmd.Args[0],cmd.Args[1:]...)
			procs.handleProcess(id,hidewnd)
			time.Sleep(time.Second)
		}
	}()


	procs.procs[id].force_closed = true
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
















func (procs *ChildProcesses) handleProcess(id ProcessID, hidewnd bool) error {
	proc := procs.procs[id]
	if proc == nil {
		return fmt.Errorf("no such process")
	}


	processname := proc.cmd.Args[0]
	stdoutIn, _ := proc.cmd.StdoutPipe()
	stderrIn, _ := proc.cmd.StderrPipe()
	
	log.PushLog("starting %s : %s", processname, strings.Join(proc.cmd.Args, " "))
	proc.cmd.SysProcAttr = &syscall.SysProcAttr{ HideWindow: hidewnd, }
	err := proc.cmd.Start()
	if err != nil {
		return fmt.Errorf("error init process %s", err.Error())
	}

	go procs.copyAndCapture(processname,"stdout" ,stdoutIn)
	go procs.copyAndCapture(processname,"stderr" ,stderrIn)
	return nil
}


func (procs *ChildProcesses) copyAndCapture(process, logtype string, r io.Reader) {
	for {
		buf, err := io.ReadAll(r)
		if err != nil {
			return
		}

		sublines := []string{}
		lines := strings.Split(string(buf),"\n")
		for _,line := range lines {
			sublines = strings.Split(line,"\r")
		}
		for _,subline := range sublines {
			if len(subline) == 0 {
				continue
			}

			procs.logger(process,subline)
		}
	}
}
