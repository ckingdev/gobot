package main

import (
	"github.com/cpalone/gobot"
	"github.com/cpalone/gobot/config"
	"github.com/cpalone/gobot/handlers"
)

func main() {
	b, err := config.BotFromCfgFile("sample.yml")
	if err != nil {
		panic(err)
	}
	b.Rooms["test"].Handlers = []gobot.Handler{&handlers.PongHandler{}}
	b.Rooms["testing"].Handlers = []gobot.Handler{&handlers.PongHandler{}}
	b.RunAllRooms()
}
