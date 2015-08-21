package config

import (
	"io/ioutil"

	"gopkg.in/yaml.v2"

	"github.com/cpalone/gobot"
	"github.com/cpalone/gobot/handlers"
)

type Config struct {
	Bot               gobot.BotConfig    `yaml:"Bot"`
	Rooms             []gobot.RoomConfig `yaml:"Rooms"`
	FollowBotProtocol bool               `yaml:"FollowBotProtocol"`
	ShortHelp         string             `yaml:"ShortHelp"`
	LongHelp          string             `yaml:"LongHelp"`
}

func configFromFile(path string) (*Config, error) {
	raw, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	c := &Config{}
	if err := yaml.Unmarshal(raw, c); err != nil {
		return nil, err
	}
	return c, nil
}

func botFromConfig(c *Config) (*gobot.Bot, error) {
	b, err := gobot.NewBot(c.Bot)
	if err != nil {
		return nil, err
	}
	if c.FollowBotProtocol {
		b.Logger.Debugln("Adding handlers for bot protocol...")
		for i, _ := range c.Rooms {
			c.Rooms[i].AddlHandlers = []gobot.Handler{
				&handlers.PongHandler{},
				&handlers.UptimeHandler{},
				&handlers.HelpHandler{LongDesc: c.LongHelp,
					ShortDesc: c.ShortHelp}}
		}
	}
	for _, roomCfg := range c.Rooms {
		roomCfg.Conn = &gobot.WSConnection{}
		b.AddRoom(roomCfg)
	}
	return b, nil
}

func BotFromCfgFile(path string) (*gobot.Bot, error) {
	cfg, err := configFromFile(path)
	if err != nil {
		return nil, err
	}
	b, err := botFromConfig(cfg)
	if err != nil {
		return nil, err
	}
	return b, nil
}
