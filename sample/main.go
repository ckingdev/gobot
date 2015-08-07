package main

import (
	"github.com/cpalone/gobot"
	"github.com/cpalone/gobot/handlers"
)

func main() {
	bcfg := gobot.BotConfig{
		Name:   "GoBot",
		DbPath: "GoBot.db",
	}
	b, err := gobot.NewBot(bcfg)
	if err != nil {
		panic(err)
	}
	rcfg := gobot.RoomConfig{
		Name:         "test",
		Password:     "",
		BotName:      "GoBot",
		AddlHandlers: []gobot.Handler{&handlers.PongHandler{}},
		Conn:         &gobot.WSConnection{},
	}
	b.AddRoom(rcfg)
	err = b.Rooms["test"].Run()
	if err != nil {
		panic(err)
	}
}
