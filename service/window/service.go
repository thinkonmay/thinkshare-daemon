package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/judwhite/go-svc"
	"github.com/thinkonmay/thinkshare-daemon/service/cmd"
	"github.com/thinkonmay/thinkshare-daemon/utils/discord"
	"github.com/thinkonmay/thinkshare-daemon/utils/log"
	"github.com/thinkonmay/thinkshare-daemon/utils/media"
	win "golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/eventlog"
	"golang.org/x/sys/windows/svc/mgr"
)

const (
	name = "thinkmay-remote-desktop"
	desc = "high performance remote desktop by Thinkmay"
)

// program implements svc.Service
type program struct {
	exit chan os.Signal
}

func main() {
	prg := &program{
		exit: make(chan os.Signal, 16),
	}

	if len(os.Args) == 4 {
		var err error
		if os.Args[1] == "discord" {
			app_id   := os.Args[2]
			activity := os.Args[3]
			if app_id != "" && app_id != "undefined" && activity != "" && activity != "undefined" {
				err = discord.StartSession(app_id, activity)
				if err != nil {
					panic(err)
				}
			}
			return
		}
	}
	if len(os.Args) == 2 {
		var err error
		if os.Args[1] == "start" {
			err = installService(name, desc)
			if err != nil {
				panic(err)
			}
			err = startService(name)
			if err != nil {
				panic(err)
			}
			return
		} else if os.Args[1] == "stop" {
			err = controlService(name, win.Stop, win.Stopped)
			if err != nil {
				panic(err)
			}
			err = removeService(name)
			if err != nil {
				panic(err)
			}
			return
		} else if os.Args[1] == "display" {
			media.ActivateVirtualDriver()
			_, id, err := media.StartVirtualDisplay(1920, 1080)
			if err != nil {
				panic(err)
			}

			defer media.RemoveVirtualDisplay(id)
			<-make(chan bool)
		}
	}

	if _, err := os.Stat("./.git"); errors.Is(err, os.ErrNotExist) {
		if err := AutoUpdate(); err != nil {
			fmt.Println(err.Error())
		}
	} else {
		if err := svc.Run(prg); err != nil {
			log.PushLog(err.Error())
		}
	}
}

func AutoUpdate() error {
	exec.Command("git", "clone", "https://github.com/thinkonmay/thinkmay", "thinkmay").Run()
	update := exec.Command("git", "pull")
	update.Dir = "./thinkmay"
	update.Run()
	update = exec.Command("git", "reset","--hard")
	update.Dir = "./thinkmay"
	update.Run()
	update = exec.Command("git", "reset","--hard")
	update.Dir = "./thinkmay/binary"
	update.Run()
	update = exec.Command("git", "submodule", "update", "--init", "binary")
	update.Dir = "./thinkmay"
	update.Run()
	final := exec.Command("./daemon.exe")
	final.Dir = "./thinkmay/binary"
	stderr,_ := final.StderrPipe()
	stdout,_ := final.StdoutPipe()
	final.Start()
	go func() {
		bytes := make([]byte, 4096)
		for {
			n, err := stdout.Read(bytes)
			if err != nil {
				break
			}

			fmt.Printf("%s",string(bytes[:n]))
		}
	}()
	go func() {
		bytes := make([]byte, 4096)
		for {
			n, err := stderr.Read(bytes)
			if err != nil {
				break
			}

			fmt.Printf("%s",string(bytes[:n]))
		}
	}()
	return final.Wait()
}

func (p *program) Init(env svc.Environment) error {
	if log_file, err := os.OpenFile("./thinkmay.log", os.O_RDWR|os.O_CREATE, 0755); err == nil {
		i := log.TakeLog(func(log string) {
			str := fmt.Sprintf("daemon.exe : %s", log)
			log_file.Write([]byte(fmt.Sprintf("%s\n", str)))
			fmt.Println(str)
		})
		defer log.RemoveCallback(i)
		defer log_file.Close()
		return nil
	} else {
		return err
	}
}

func (p *program) Start() error {
	// The Start method must not block, or Windows may assume your service failed
	// to start. Launch a Goroutine here to do something interesting/blocking.

	go cmd.Start(nil, p.exit)
	return nil
}

func (p *program) Stop() error {
	// The Stop method is invoked by stopping the Windows service, or by pressing Ctrl+C on the console.
	// This method may block, but it's a good idea to finish quickly or your process may be killed by
	// Windows during a shutdown/reboot. As a general rule you shouldn't rely on graceful shutdown.

	p.exit <- os.Interrupt
	<-p.exit
	log.PushLog("Stopped.")
	time.Sleep(3 * time.Second)
	return nil
}

func exePath() (string, error) {
	prog := os.Args[0]
	p, err := filepath.Abs(prog)
	if err != nil {
		return "", err
	}
	fi, err := os.Stat(p)
	if err == nil {
		if !fi.Mode().IsDir() {
			return p, nil
		}
		err = fmt.Errorf("%s is directory", p)
	}
	if filepath.Ext(p) == "" {
		p += ".exe"
		fi, err := os.Stat(p)
		if err == nil {
			if !fi.Mode().IsDir() {
				return p, nil
			}
			err = fmt.Errorf("%s is directory", p)
		}
	}
	return "", err
}

func installService(name, desc string) error {
	exepath, err := exePath()
	if err != nil {
		return err
	}
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	s, err := m.OpenService(name)
	if err == nil {
		s.Close()
		return fmt.Errorf("service %s already exists", name)
	}
	s, err = m.CreateService(name, exepath, mgr.Config{DisplayName: desc}, "is", "auto-started")
	if err != nil {
		return err
	}
	defer s.Close()
	err = eventlog.InstallAsEventCreate(name, eventlog.Error|eventlog.Warning|eventlog.Info)
	if err != nil {
		s.Delete()
		return fmt.Errorf("SetupEventLogSource() failed: %s", err)
	}
	return nil
}

func startService(name string) error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	s, err := m.OpenService(name)
	if err != nil {
		return fmt.Errorf("could not access service: %v", err)
	}
	defer s.Close()
	err = s.Start("is", "manual-started")
	if err != nil {
		return fmt.Errorf("could not start service: %v", err)
	}
	return nil
}

func removeService(name string) error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	s, err := m.OpenService(name)
	if err != nil {
		return fmt.Errorf("service %s is not installed", name)
	}
	defer s.Close()
	err = s.Delete()
	if err != nil {
		return err
	}
	err = eventlog.Remove(name)
	if err != nil {
		return fmt.Errorf("RemoveEventLogSource() failed: %s", err)
	}
	return nil
}

func controlService(name string, c win.Cmd, to win.State) error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	s, err := m.OpenService(name)
	if err != nil {
		return fmt.Errorf("could not access service: %v", err)
	}
	defer s.Close()
	status, err := s.Control(c)
	if err != nil {
		return fmt.Errorf("could not send control=%d: %v", c, err)
	}
	timeout := time.Now().Add(10 * time.Second)
	for status.State != to {
		if timeout.Before(time.Now()) {
			return fmt.Errorf("timeout waiting for service to go to state=%d", to)
		}
		time.Sleep(300 * time.Millisecond)
		status, err = s.Query()
		if err != nil {
			return fmt.Errorf("could not retrieve service status: %v", err)
		}
	}
	return nil
}
