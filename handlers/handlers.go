// Package handlers provides several pre-baked gobot.Handlers for convenience.
package handlers

import (
	"fmt"
	"strings"
	"time"

	"euphoria.io/heim/proto"
	"github.com/cpalone/gobot"
)

// PongHandler responds to a send-event starting with "!ping" and returns a send
// command containing "pong!".
type PongHandler struct{}

// HandleIncoming satisfies the Handler interface.
func (ph *PongHandler) HandleIncoming(r *gobot.Room, p *proto.Packet) (*proto.Packet, error) {
	r.Logger.Debugln("Checking for ping command...")
	if p.Type != proto.SendEventType {
		return nil, nil
	}
	raw, err := p.Payload()
	if err != nil {
		return nil, err
	}
	payload, ok := raw.(*proto.SendEvent)
	if !ok {
		r.Logger.Warningln("Unable to assert packet as SendEvent.")
		return nil, err
	}
	if !strings.HasPrefix(payload.Content, "!ping") {
		return nil, nil
	}
	if strings.Contains(payload.Content, "@") && !strings.HasPrefix(payload.Content, "!ping @"+r.BotName) {
		return nil, nil
	}
	r.Logger.Debugln("Sending !ping reply...")
	if _, err := r.SendText(&payload.ID, "pong!"); err != nil {
		return nil, err
	}
	return nil, nil
}

// Run is a no-op- the PongHandler does not need to run continuously, only in
// response to an incoming packet.
func (ph *PongHandler) Run(r *gobot.Room) {
	return
}

// Stop is also a no-op- there is nothing to stop.
func (ph *PongHandler) Stop(r *gobot.Room) {
	return
}

// UptimeHandler records the time when the bot goes up and responds to commands
// with the duration the bot has been up.
type UptimeHandler struct {
	t0 time.Time
}

// Run simply records the time.
func (u *UptimeHandler) Run(r *gobot.Room) {
	u.t0 = time.Now()
}

// HandleIncoming checks incoming commands for !uptime or !uptime @[BotName] and
// responds with the duration the bot has been up.
func (u *UptimeHandler) HandleIncoming(r *gobot.Room, p *proto.Packet) (*proto.Packet, error) {
	if p.Type != proto.SendEventType {
		return nil, nil
	}
	raw, err := p.Payload()
	if err != nil {
		return nil, err
	}
	payload, ok := raw.(*proto.SendEvent)
	if !ok {
		r.Logger.Warningln("Unable to assert packet as SendEvent.")
		return nil, err
	}
	if !strings.HasPrefix(payload.Content, "!uptime") {
		return nil, nil
	}
	if payload.Content != "!uptime" && payload.Content != "!uptime @"+r.BotName {
		return nil, nil
	}
	uptime := time.Since(u.t0)
	days := int(uptime.Hours()) / 24
	hours := int(uptime.Hours()) % 24
	minutes := int(uptime.Minutes()) % 60
	seconds := int(uptime.Seconds()) % 60
	if _, err := r.SendText(&payload.ID, fmt.Sprintf(
		"This bot has been up for %dd %dh %dm %.3fs.",
		days, hours, minutes, seconds)); err != nil {
		return nil, err
	}
	return nil, nil
}

// Stop is a no-op.
func (u *UptimeHandler) Stop(r *gobot.Room) {
	return
}

// HelpHandler stores a short help message and a long help message and responds
// with them to !help and !help @[BotName], respectively.
type HelpHandler struct {
	ShortDesc string
	LongDesc  string
}

// Run is a no-op.
func (h *HelpHandler) Run(r *gobot.Room) {
	return
}

// Stop is a no-op.
func (h *HelpHandler) Stop(r *gobot.Room) {
	return
}

// HandleIncoming checks incoming SendEvents for help commands and responds
// appropriately.
func (h *HelpHandler) HandleIncoming(r *gobot.Room, p *proto.Packet) (*proto.Packet, error) {
	if p.Type != proto.SendEventType {
		return nil, nil
	}
	raw, err := p.Payload()
	if err != nil {
		return nil, err
	}
	payload, ok := raw.(*proto.SendEvent)
	if !ok {
		r.Logger.Warningln("Unable to assert packet as SendEvent.")
		return nil, err
	}
	if !strings.HasPrefix(payload.Content, "!help") {
		return nil, nil
	}
	if payload.Content != "!help @"+r.BotName && payload.Content != "!help" {
		return nil, nil
	}
	if payload.Content == "!help" {
		if _, err := r.SendText(&payload.ID, h.ShortDesc); err != nil {
			return nil, err
		}
		return nil, nil
	}
	if _, err := r.SendText(&payload.ID, h.LongDesc); err != nil {
		return nil, err
	}
	return nil, nil
}
