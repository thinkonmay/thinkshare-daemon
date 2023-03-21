package childprocess

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/thinkonmay/thinkshare-daemon/log"
)

type ProcessID int

type Event struct {
	raised bool
}

func NewEvent() *Event {
	return &Event{
		raised: false,
	}
}
func (eve *Event) Wait() {
	for {
		if eve.raised {
			return
		} else {
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func (eve *Event) IsInvoked() bool {
	return bool(eve.raised)
}
func (eve *Event) Raise() {
	eve.raised = true
}

type ChildProcess struct {
	cmd *exec.Cmd

	shutdown *Event
	done     *Event
}
type ChildProcesses struct {
	ready bool
	count int
	mutex sync.Mutex
	procs map[ProcessID]*ChildProcess

	ExitEvent chan ProcessID
}

func NewChildProcessSystem() *ChildProcesses {
	ret := ChildProcesses{
		ExitEvent: make(chan ProcessID),

		procs: make(map[ProcessID]*ChildProcess),
		mutex: sync.Mutex{},
		count: 0,
		ready: true,
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

func (procs *ChildProcesses) handleProcess(id ProcessID) {
	proc := procs.procs[id]

	processname := proc.cmd.Args[0]
	stdoutIn, _ := proc.cmd.StdoutPipe()
	stderrIn, _ := proc.cmd.StderrPipe()
	stdinOut, _ := proc.cmd.StdinPipe()
	go func() {
		for {
			time.Sleep(1 * time.Second)
			_, err := stdinOut.Write([]byte("\n"))
			if err != nil {
				return
			}
		}
	}()

	_log := make([]byte, 0)
	for _, i := range proc.cmd.Args {
		_log = append(_log, append([]byte(i), []byte(" ")...)...)
	}
	log.PushLog("starting %s : %s\n", processname, string(_log))
	proc.cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow: true,
	}
	err := proc.cmd.Start()
	if err != nil {
		log.PushLog("error init process %s\n", err.Error())
		return
	}

	go procs.copyAndCapture(processname, stdoutIn)
	go procs.copyAndCapture(processname, stderrIn)

	go func() {
		proc.shutdown.Wait()
		if !proc.done.IsInvoked() {
			proc.done.Raise()
			proc.cmd.Process.Kill()
		}
	}()
	go func() {
		proc.cmd.Wait()
		if !proc.done.IsInvoked() {
			proc.done.Raise()
		}
	}()

	proc.done.Wait()
}

func (procs *ChildProcesses) NewChildProcess(cmd *exec.Cmd) ProcessID {
	if !procs.ready {
		return ProcessID(-1)
	}

	if cmd == nil {
		return -1
	}

	procs.mutex.Lock()
	defer func() {
		procs.mutex.Unlock()
		procs.count++
	}()

	id := ProcessID(procs.count)
	procs.procs[id] = &ChildProcess{
		cmd:      cmd,
		shutdown: NewEvent(),
		done:     NewEvent(),
	}

	go func() {
		log.PushLog("process %s, process id %d booting up\n", cmd.Args[0], int(id))
		procs.handleProcess(id)
		procs.ExitEvent <- id
	}()

	return ProcessID(procs.count)
}

func (procs *ChildProcesses) CloseAll() {
	procs.mutex.Lock()
	defer procs.mutex.Unlock()

	procs.ready = false
	for _, proc := range procs.procs {
		proc.shutdown.Raise()
	}
}

func (procs *ChildProcesses) CloseID(ID ProcessID) {
	procs.mutex.Lock()
	defer procs.mutex.Unlock()

	proc := procs.procs[ID]
	if proc == nil {
		return
	}

	log.PushLog("force terminate process name %s, process id %d \n", proc.cmd.Args[0], int(ID))
	proc.shutdown.Raise()
}

func (procs *ChildProcesses) WaitID(ID ProcessID) {
	for {
		id := <-procs.ExitEvent
		if id == ID {
			log.PushLog("process name %s with id %d exited \n", procs.procs[ID].cmd.Args[0], int(ID))
			return
		} else {
			procs.ExitEvent <- id
			time.Sleep(10 * time.Millisecond)
		}
	}
}



func (procs *ChildProcesses) copyAndCapture(process string, r io.Reader) {
	procname := strings.Split(process,"\\")
	prefix := []byte(fmt.Sprintf("Child process (%s): ", procname[len(procname)-1]))
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
			if len(line) < 2 {
				continue
			}
			log.PushLog(fmt.Sprintf("%s%s",prefix,line))
			if err != nil {
				return
			}
		}
	}
}
