package gobot

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"euphoria.io/heim/proto"
	"euphoria.io/heim/proto/snowflake"

	"euphoria.io/scope"
	"github.com/Sirupsen/logrus"
	"github.com/boltdb/bolt"
)

const (
	// MAXRETRIES is the number of times to retry a connection to euphoria.
	MAXRETRIES = 5
)

func init() {
	logrus.SetLevel(logrus.InfoLevel)
}

// MakePacket is a convenience function that takes a payload and a PacketType
// and returns a Packet. The message ID of the packet is NOT set.
func MakePacket(msgType proto.PacketType, payload interface{}) (*proto.Packet, error) {
	packet := &proto.Packet{
		Type: msgType}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	if err := packet.Data.UnmarshalJSON(data); err != nil {
		return nil, err
	}
	return packet, nil
}

// Bot is the highest level abstraction- it contains and controls multiple Room
// structs. Methods on Bot can add the bot to new rooms or, eventually, remove
// them.
//
// Bot exposes a bolt database for the use of the user. The basic bot does not
// use the database, so there is no chance of collisions in bucket names or
// key-value instances.
type Bot struct {
	Rooms   map[string]*Room
	BotName string
	ctx     scope.Context
	DB      *bolt.DB
	Logger  *logrus.Logger
	cmd     chan interface{}
}

// Room contains a connection to a euphoria room and uses Handlers to process
// packets and optionally reply to them. The DB member points to the same DB
// initialized by the parent Bot. The Ctx member is distinct from the Bot's ctx
// member.
type Room struct {
	RoomName string
	conn     Connection
	Ctx      scope.Context
	password string
	outbound chan *proto.Packet
	inbound  chan *proto.Packet
	Handlers []Handler
	msgID    int
	BotName  string
	Logger   *logrus.Logger
	DB       *bolt.DB
}

// BotConfig controls the configuration of a new Bot when it is created by the
// user.
type BotConfig struct {
	Name   string `yaml:"Name"`
	DbPath string `yaml:"DbPath"`
}

// NewBot creates a bot with the given configuration. It will create a bolt DB
// if it does not already exist at the specified location.
func NewBot(cfg BotConfig) (*Bot, error) {
	db, err := bolt.Open(cfg.DbPath, 0666, nil)
	if err != nil {
		return nil, err
	}
	ctx := scope.New()
	logger := logrus.New()
	logger.Level = logrus.DebugLevel
	cmd := make(chan interface{})
	rooms := make(map[string]*Room)
	return &Bot{
		Rooms:   rooms,
		BotName: cfg.Name,
		ctx:     ctx,
		DB:      db,
		Logger:  logger,
		cmd:     cmd,
	}, nil
}

// RoomConfig controls the configuration of a new Room when it is added to a Bot.
type RoomConfig struct {
	RoomName     string `yaml:"RoomName"`
	Password     string `yaml:"Password,omitempty"`
	AddlHandlers []Handler
	Conn         Connection
}

// AddRoom adds a new Room to the bot with the given configuration. The context
// for this room is distinct from the Bot's context.
func (b *Bot) AddRoom(cfg RoomConfig) {
	b.Logger.Debugf("%s", len(cfg.AddlHandlers))
	ctx := scope.New()
	logger := logrus.New()
	logger.Level = logrus.DebugLevel
	room := Room{
		RoomName: cfg.RoomName,
		password: cfg.Password,
		Ctx:      ctx,
		outbound: make(chan *proto.Packet, 5),
		inbound:  make(chan *proto.Packet, 5),
		BotName:  b.BotName,
		msgID:    0,
		Logger:   logger,
		Handlers: cfg.AddlHandlers,
		DB:       b.DB,
		conn:     cfg.Conn,
	}
	b.Rooms[room.RoomName] = &room
}

func (r *Room) sendLoop() {
	defer r.Ctx.WaitGroup().Done()
	for {
		select {
		case <-r.Ctx.Done():
			r.Logger.Debugln("sendLoop exiting...")
			return
		case msg := <-r.outbound:
			r.Logger.Debugf("Sending message of type %s...", msg.Type)
			if _, err := r.conn.SendJSON(r, msg); err != nil {
				logrus.Errorf("Error sending JSON, terminating room: %s", err)
				r.Ctx.Terminate(err)
				return
			}
		}
	}

}

func (r *Room) recvLoop() {
	defer r.Ctx.WaitGroup().Done()
	for {
		pchan := make(chan *proto.Packet)
		go r.conn.ReceiveJSON(r, pchan)
		select {
		case <-r.Ctx.Done():
			r.Logger.Debugln("recvLoop exiting...")
			close(pchan)
			return
		case p := <-pchan:
			if p != nil {
				r.inbound <- p
			}
		}
	}
}

func (r *Room) runHandlerIncoming(handler Handler, p proto.Packet) {
	// defer r.Ctx.WaitGroup().Done()
	r.Logger.Debugln("runHandlerIncoming")
	retPacket, err := handler.HandleIncoming(r, &p)
	if err != nil {
		r.Logger.Errorf("Error in handler, shutting down room: %s", err)
		r.Ctx.Terminate(err)
		return
	}
	if retPacket != nil {
		r.outbound <- retPacket
	}
}

func (r *Room) handlePing(p *proto.Packet) error {
	r.Logger.Debugln("Handling ping...")
	raw, err := p.Payload()
	if err != nil {
		return err
	}
	payload, ok := raw.(*proto.PingEvent)
	if !ok {
		return fmt.Errorf("Could not assert payload as *packet.PingEvent")
	}
	r.queuePayload(proto.PingReply{UnixTime: payload.UnixTime}, proto.PingReplyType)
	return nil
}

func (r *Room) dispatcher() {
	defer r.Ctx.WaitGroup().Done()
	for {
		select {
		case <-r.Ctx.Done():
			r.Logger.Debugln("dispatcher exiting...")
			return
		case p := <-r.inbound:
			r.Logger.Debugf("Dispatching packet of type %s", p.Type)
			if p.Type == proto.PingEventType {
				err := r.handlePing(p)
				if err != nil {
					r.Logger.Errorf("Error handling ping, shutting down room: %s", err)
					r.Ctx.Terminate(err)
					return
				}
			} else if p.Type == proto.ErrorReplyType {
				r.Logger.Errorf("Error packet received- full contents: %s", p)
				r.Ctx.Cancel()
				return
			} else if p.Error != "" {
				r.Logger.Errorf("Error in packet: %s", p.Error)
				r.Ctx.Cancel()
				return
			} else if p.Type == proto.BounceEventType {
				payload, err := p.Payload()
				if err != nil {
					r.Logger.Error("Could not extract payload.")
					r.Ctx.Cancel()
					return
				}
				bounce, ok := payload.(*proto.BounceEvent)
				if !ok {
					r.Logger.Error("Could not assert BounceEvent as such.")
					r.Ctx.Cancel()
					return
				}
				r.Logger.Errorf("Bounced: %s", bounce.Reason)
			} else if p.Type == proto.DisconnectEventType {
				payload, err := p.Payload()
				if err != nil {
					r.Logger.Error("Could not extract payload.")
					r.Ctx.Cancel()
					return
				}
				disc, ok := payload.(*proto.DisconnectEvent)
				if !ok {
					r.Logger.Error("Could not assert DisconnectEvent as such.")
					r.Ctx.Cancel()
					return
				}
				r.Logger.Errorf("disconnect-event received: reason: %s", disc.Reason)
				r.Ctx.Cancel()
				return
			}
			for _, handler := range r.Handlers {
				r.Logger.Debugln("Running handler...")
				r.runHandlerIncoming(handler, *p)
			}
		}
	}
}

func (r *Room) queuePayload(payload interface{}, pType proto.PacketType) string {
	msg, err := MakePacket(pType, payload)
	if err != nil {
		r.Logger.Errorf("Error making packet, shutting room down: %s", err)
		r.Ctx.Terminate(err)
		return ""
	}
	msg.ID = strconv.Itoa(r.msgID)
	r.msgID++
	go func() {
		r.outbound <- msg
	}()
	return msg.ID
}

func (r *Room) sendNick() (string, error) {
	payload := proto.NickCommand{Name: r.BotName}
	msgID := r.queuePayload(payload, proto.NickType)
	return msgID, nil
}

func (r *Room) sendAuth() (string, error) {
	payload := proto.AuthCommand{
		Type:     "passcode",
		Passcode: r.password,
	}
	msgID := r.queuePayload(payload, proto.AuthType)
	return msgID, nil
}

// SendText sends a text message to the server. The parent must be a Snowflake
// from a previously received message ID or nil for a root message.
func (r *Room) SendText(parent *snowflake.Snowflake, msg string) (string, error) {
	r.Logger.Debugf("Sending text message with text: %s", msg)

	payload := &proto.SendCommand{
		Content: msg,
	}
	if parent != nil {
		payload.Parent = *parent
	}
	r.Logger.Debugf("Putting message in outgoing queue with text: %s", msg)
	msgID := r.queuePayload(payload, proto.SendType)
	return msgID, nil

}

// Run starts up the necessary goroutines to send, receive, and dispatch packets
// to handlers.
func (r *Room) Run() error {
	go r.monitorLoop()

	r.Ctx.WaitGroup().Add(1)
	go r.sendLoop()

	if err := r.conn.Connect(r); err != nil {
		r.Ctx.Terminate(err)
		r.Logger.Errorf("Error on initial connect to room: %s", err)
		r.Ctx.WaitGroup().Wait()
		return err
	}
	r.Ctx.WaitGroup().Add(1)
	go r.recvLoop()

	r.Ctx.WaitGroup().Add(1)
	go r.dispatcher()

	for _, handler := range r.Handlers {
		go handler.Run(r)
	}
	<-r.Ctx.Done()
	r.Logger.Warnf("Room %s's context is finished.", r.RoomName)
	return fmt.Errorf("Fatal error in room %s: %s", r.RoomName, r.Ctx.Err())
}

// RunAllRooms runs room.Run() for all rooms registered with the bot. It will
// only return when all rooms are exited- common usage will be running this as
// a goroutine.
func (b *Bot) RunAllRooms() {
	go b.monitorLoop()
	errChan := make(chan error, len(b.Rooms))
	for _, room := range b.Rooms {
		b.ctx.WaitGroup().Add(1)
		go func(r *Room) {
			defer b.ctx.WaitGroup().Done()
			err := r.Run()
			r.Logger.Errorf("Error in room %s: %s", room.RoomName, err)
			if err = r.Stop(); err != nil {
				r.Logger.Errorf("Error stopping room %s: %s", room.RoomName, err)
			}
			errChan <- err
		}(room)
	}
	go func() {
		b.ctx.WaitGroup().Wait()
		b.Logger.Warnln("Bot waitgroup finished.")
		close(errChan)
	}()
	for err := range errChan {
		b.Logger.Errorf("%s", err)
	}
}

func (r *Room) monitorLoop() {
	for {
		<-time.After(120 * time.Second)
		if r.Ctx.Alive() {
			r.Logger.Debugln("Context is alive.")
		} else {
			r.Logger.Debugln("Context is dead.")
		}
	}
}

func (b *Bot) monitorLoop() {
	for {
		<-time.After(120 * time.Second)
		if b.ctx.Alive() {
			b.Logger.Debugln("Context is alive.")
		} else {
			b.Logger.Debugln("Context is dead.")
		}
	}
}

// Stop cancels a Room's context, closes the connection, and waits for all
// goroutines spawned by the Room to stop.
func (r *Room) Stop() error {
	r.Logger.Warningf("Room '%s' shutting down", r.RoomName)
	r.Ctx.Cancel()
	r.Logger.Debugln("Closing connection...")
	if err := r.conn.Close(); err != nil {
		r.Logger.Debugf("Error closing connection: %s", err)
		return err
	}
	r.Logger.Debugln("Waiting for graceful shutdown...")
	r.Ctx.WaitGroup().Wait()
	return nil
}

// Stop runs Room.Stop() for all rooms registered with the bot, cancels the
// bot's context, and waits for all goroutines to exit before closing the DB.
func (b *Bot) Stop() {
	for _, room := range b.Rooms {
		if err := room.Stop(); err != nil {
			b.Logger.Errorf("Error stopping room: %s", err)
		}
	}
	b.ctx.Cancel()
	b.ctx.WaitGroup().Wait()
	if err := b.DB.Close(); err != nil {
		b.Logger.Errorf("Error closing database: %s", err)
	}
}
