package update

import (
	"fmt"
	"os/exec"
	"strings"
)

func Update() {
	out, err := exec.Command("go", "version").Output()
	out, err  = exec.Command("dotnet",  "--list-sdks").Output()
	out, err  = exec.Command("git", "--version").Output()
	out, err  = exec.Command("gcc", "--version" ).Output()
	out, err  = exec.Command("gst-inspect-1.0", "--version").Output()

	fmt.Printf("%v\n",out)

	commitHash, err := exec.Command("git", "rev-parse", "HEAD").Output()
	if err == nil {
		fmt.Printf("current commit hash: %s \n", commitHash)
	} else if commitHash == nil {
		fmt.Println("you are not using git, please download git to have auto update")
	} else if strings.Contains(string(commitHash), "fatal") {
		fmt.Println("you did not clone this repo, please use clone")
	}





}