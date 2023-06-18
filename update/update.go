package update

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/thinkonmay/thinkshare-daemon/credential"
)

func Update() {
	out, err  := exec.Command("git", "--version").Output()
	if err != nil {
		panic(err)
	}
	fmt.Printf("git version %s",string(out))
	currentCommitHash,_ := exec.Command("git", "rev-parse", "HEAD").Output()
	if strings.Contains(string(currentCommitHash), "fatal") {
		fmt.Println("you did not clone this repo, please use clone")
		os.Exit(0)
	}

	desiredCommitHash := credential.Secrets.Daemon.Commit
	fmt.Printf("desired commit hash: %s\n",desiredCommitHash)
	fmt.Printf("current commit hash: %s\n",currentCommitHash)
	if !strings.Contains(string(currentCommitHash),desiredCommitHash) {
		fmt.Println("daemon is not in sync, restarting")
		exec.Command("git", "reset","--hard").Output()
		exec.Command("git", "pull").Output()
		exec.Command("git", "checkout" , desiredCommitHash).Output()
		exec.Command("git", "submodule" , "update","--init").Output()
	}
}