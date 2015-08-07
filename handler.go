package gobot

import (
	"euphoria.io/heim/proto"
)

// Handler is an interface that processes incoming packets and optionally
// returns a packet to be sent by the bot.
//
// The Run method will be called when the Room is run. This allows a handler to
// maintain state and send packets that are not in response to an incoming
// packet.
type Handler interface {
	// HandleIncoming is called whenever a packet is received over the
	// connection to euphoria. It is passed a pointer to the calling Room and
	// a pointer to that incoming packet.
	HandleIncoming(r *Room, p *proto.Packet) (*proto.Packet, error)

	// Run is called whenever the Room the Handler is attached to has its Run
	// method called. It allows for a Handler to maintain its own state and
	// logic between incoming packets. Note, the access to the calling Room
	// means that the Handler can send packets that are not directly in response
	// to an incoming packet.
	Run(r *Room)

	// Stop is called whenever the Room the Handler is attached to has its Stop
	// method called. The Handler must not block and be available to receive
	// a signal from r.Ctx.Done() or check that r.Ctx.Alive() is false.
	Stop(r *Room)
}
