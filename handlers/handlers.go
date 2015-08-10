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
	if strings.Contains(payload.Content, "@") && !strings.HasPrefix(payload.Content, "!ping @" + r.BotName) {
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

type UptimeHandler struct {
	t0 time.Time
}

func (u *UptimeHandler) Run(r *gobot.Room) {
	u.t0 = time.Now()
}

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
	if payload.Content != "!uptime" && payload.Content != "!uptime @" + r.BotName {
		return nil, nil
	}
	uptime := time.Since(u.t0)
	if _, err := r.SendText(&payload.ID, fmt.Sprintf("This bot has been up for %v hours.", uptime.Hours())); err != nil {
		return nil, err
	}
	return nil, nil
}

func (u *UptimeHandler) Stop(r *gobot.Room) {
	return
}

type HelpHandler struct {
	ShortDesc string
	LongDesc string
}

func (h *HelpHandler) Run(r *gobot.Room) {
	return
}

func (h *HelpHandler) Stop(r *gobot.Room) {
	return
}

func (h *HelpHandler)HandleIncoming(r *gobot.Room, p *proto.Packet) (*proto.Packet, error) {
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
	if payload.Content != "!help @" + r.BotName && payload.Content != "!help" {
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
