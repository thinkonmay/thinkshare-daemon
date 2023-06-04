package update

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/thinkonmay/thinkshare-daemon/credential"
)

func Update() {
	out, err := exec.Command("go", "version").Output()
	if err != nil {
		panic(err)
	}
	fmt.Printf("go version %s",string(out))
	out, err  = exec.Command("dotnet",  "--list-sdks").Output()
	if err != nil {
		panic(err)
	}
	fmt.Printf("dotnet version %s",string(out))
	out, err  = exec.Command("git", "--version").Output()
	if err != nil {
		panic(err)
	}
	fmt.Printf("git version %s",string(out))
	out, err  = exec.Command("gcc", "--version" ).Output()
	if err != nil {
		panic(err)
	}
	fmt.Printf("gcc version %s",string(out))
	out, err  = exec.Command("gst-inspect-1.0", "--version").Output()
	if err != nil {
		panic(err)
	}
	fmt.Printf("gstreamer version %s",string(out))

	currentCommitHash, err := exec.Command("git", "rev-parse", "HEAD").Output()
	if err == nil {
		fmt.Printf("current commit hash: %s \n", currentCommitHash)
	} else if currentCommitHash == nil {
		fmt.Println("you are not using git, please download git to have auto update")
	} else if strings.Contains(string(currentCommitHash), "fatal") {
		fmt.Println("you did not clone this repo, please use clone")
	}



	desiredCommitHash := credential.Secrets.Daemon.Commit
	if desiredCommitHash != string(currentCommitHash) {
		fmt.Println("daemon is not in sync, restarting")
		exec.Command("git", "pull").Output()
		exec.Command("git", "checkout" , desiredCommitHash).Output()
		os.Exit(0)
	}
}