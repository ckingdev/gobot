// Package main provides some sample code for setting up a bot.
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

//	b.Rooms["test"].Handlers = append(b.Rooms["test"].Handlers, &handlers.PongHandler{})
	b.Rooms["testing"].Handlers = []gobot.Handler{&handlers.PongHandler{}}
	b.RunAllRooms()
}
