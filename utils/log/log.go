package log

import "fmt"

var output chan string

func init() {
	output = make(chan string, 100)
}

func TakeLog() string {
	return <-output
}

func PushLog(format string, a ...any) {
	output <-fmt.Sprintf(format,a...);
}