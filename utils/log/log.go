package log

import (
	"fmt"
	"sync"
)

var output  chan string
var handler map[int]func(log string)
var mut *sync.Mutex
var count = 0

func init() {
	output = make(chan string, 100)
	handler = map[int]func(log string){}
	mut = &sync.Mutex{}

	go func ()  {
		for {
			log := <-output

			mut.Lock()
			for _,v := range handler { v(log) }
			mut.Unlock()
		}
	}()
}

func TakeLog(fun func(log string)) int {
	mut.Lock()
	defer mut.Unlock()

	handler[count] = fun
	count++
	return count
}

func RemoveCallback(id int) {
	mut.Lock()
	defer mut.Unlock()

	delete(handler,id)
}

func PushLog(format string, a ...any) {
	output <-fmt.Sprintf(format,a...);
}