package handlers

import (
	"strings"

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
	payload, ok := raw.(*proto.SendEvent)
	if !ok {
		r.Logger.Warningln("Unable to assert packet as SendEvent.")
		return nil, err
	}
	if !strings.HasPrefix(payload.Content, "!ping") {
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
