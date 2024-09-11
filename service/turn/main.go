package main

import (
	"fmt"
	"net/http"
	"os"
	"strconv"

	"github.com/thinkonmay/thinkshare-daemon/utils/turn"
)

func main() {
	port, err := strconv.ParseInt(os.Getenv("PORT"), 10, 64)
	if err != nil {
		panic(err)
	}

	server, err := turn.NewTurnServer(turn.TurnServerConfig{
		Path: os.Getenv("PATH"),
	})
	if err != nil {
		panic(err)
	}

	fmt.Printf("started turn http server on port %d\n", port)
	panic(http.ListenAndServe(fmt.Sprintf("0.0.0.0:%d", port), server.Mux))
}
