package main

import (
	"os"
	"strconv"

	"github.com/thinkonmay/thinkshare-daemon/utils/turn"
)

func main() {
	port, err := strconv.ParseInt(os.Getenv("PORT"), 10, 64)
	if err != nil {
		panic(err)
	}

	turn.NewTurnServer(turn.TurnServerConfig{
		Port: int(port),
		Path: os.Getenv("PATH"),
	})
}
