package cmd

import (
	"fmt"
	"testing"
)

func TestFindPID(t *testing.T) {
	pid,found := findPreviousPID(60000)
	fmt.Println(pid,found)
}
