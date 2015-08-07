package gobot

import (
	"encoding/json"
	"fmt"
	"strconv"

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
	logrus.SetLevel(logrus.WarnLevel)
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
	Rooms  map[string]*Room
	name   string
	ctx    scope.Context
	DB     *bolt.DB
	logger *logrus.Logger
	cmd    chan interface{}
}

// Room contains a connection to a euphoria room and uses Handlers to process
// packets and optionally reply to them. The DB member points to the same DB
// initialized by the parent Bot. The Ctx member is distinct from the Bot's ctx
// member.
type Room struct {
	name     string
	conn     Connection
	Ctx      scope.Context
	password string
	outbound chan *proto.Packet
	inbound  chan *proto.Packet
	handlers []Handler
	msgID    int
	botName  string
	Logger   *logrus.Logger
	DB       *bolt.DB
}

// BotConfig controls the configuration of a new Bot when it is created by the
// user.
type BotConfig struct {
	Name   string
	DbPath string
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
		Rooms:  rooms,
		name:   cfg.Name,
		ctx:    ctx,
		DB:     db,
		logger: logger,
		cmd:    cmd,
	}, nil
}

// RoomConfig controls the configuration of a new Room when it is added to a Bot.
type RoomConfig struct {
	Name         string
	Password     string
	AddlHandlers []Handler
	BotName      string
	Conn         Connection
}

// AddRoom adds a new Room to the bot with the given configuration. The context
// for this room is distinct from the Bot's context.
func (b *Bot) AddRoom(cfg RoomConfig) {
	ctx := scope.New()
	var handlers []Handler
	handlers = append(handlers, cfg.AddlHandlers...)
	logger := logrus.New()
	logger.Level = logrus.DebugLevel
	room := Room{
		name:     cfg.Name,
		password: cfg.Password,
		Ctx:      ctx,
		outbound: make(chan *proto.Packet, 5),
		inbound:  make(chan *proto.Packet, 5),
		botName:  cfg.BotName,
		msgID:    0,
		Logger:   logger,
		handlers: handlers,
		DB:       b.DB,
		conn:     cfg.Conn,
	}
	b.Rooms[room.name] = &room
}

func (r *Room) sendLoop() {
	for {
		select {
		case <-r.Ctx.Done():
			r.Ctx.WaitGroup().Done()
			return
		case msg := <-r.outbound:
			r.Logger.Debugf("Sending message of type %s...", msg.Type)
			if _, err := r.conn.SendJSON(r, msg); err != nil {
				logrus.Errorf("Error sending JSON: %s", err)
				r.Ctx.Terminate(err)
			}
		}
	}

}

func (r *Room) recvLoop() {
	for {
		pchan := make(chan *proto.Packet)
		go r.conn.ReceiveJSON(r, pchan)
		select {
		case <-r.Ctx.Done():
			close(pchan)
			r.Ctx.WaitGroup().Done()
			return
		case p := <-pchan:
			r.inbound <- p
		}
	}
}

func (r *Room) runHandlerIncoming(handler Handler, p proto.Packet) {
	defer r.Ctx.Done()
	retPacket, err := handler.HandleIncoming(r, &p)
	if err != nil {
		r.Ctx.Terminate(err)
		return
	}
	if retPacket != nil {
		r.outbound <- retPacket
	} else {
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
			return
		case p := <-r.inbound:
			r.Logger.Debugf("Dispatching packet of type %s", p.Type)
			if p.Type == proto.PingEventType {
				err := r.handlePing(p)
				if err != nil {
					r.Ctx.Terminate(err)
					return
				}
			}
			for _, handler := range r.handlers {
				r.runHandlerIncoming(handler, *p)
			}
		}
	}
}

func (r *Room) queuePayload(payload interface{}, pType proto.PacketType) string {
	msg, err := MakePacket(pType, payload)
	if err != nil {
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
	payload := proto.NickCommand{Name: r.botName}
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
	r.Ctx.WaitGroup().Add(1)
	go r.sendLoop()

	if err := r.conn.Connect(r); err != nil {
		r.Ctx.Terminate(err)
		r.Ctx.WaitGroup().Wait()
		return err
	}
	r.Ctx.WaitGroup().Add(1)
	go r.recvLoop()

	r.Ctx.WaitGroup().Add(1)
	go r.dispatcher()

	for _, handler := range r.handlers {
		go handler.Run(r)
	}

	<-r.Ctx.Done()
	return fmt.Errorf("Fatal error in room %s: %s", r.name, r.Ctx.Err())
}

// RunAllRooms runs room.Run() for all rooms registered with the bot. It will
// only return when all rooms are exited- common usage will be running this as
// a goroutine.
func (b *Bot) RunAllRooms() {
	errChan := make(chan error, len(b.Rooms))
	for _, room := range b.Rooms {
		b.ctx.WaitGroup().Add(1)
		go func(r *Room) {
			err := r.Run()
			errChan <- err
			b.ctx.WaitGroup().Done()
		}(room)
	}
	go func() {
		b.ctx.WaitGroup().Wait()
		close(errChan)
	}()
	for err := range errChan {
		b.logger.Errorf("%s", err)
	}
}

// Stop cancels a Room's context, closes the connection, and waits for all
// goroutines spawned by the Room to stop.
func (r *Room) Stop() error {
	r.Ctx.Cancel()
	if err := r.conn.Close(); err != nil {
		return err
	}
	r.Ctx.WaitGroup().Wait()
	return nil
}

// Stop runs Room.Stop() for all rooms registered with the bot, cancels the
// bot's context, and waits for all goroutines to exit before closing the DB.
func (b *Bot) Stop() {
	for _, room := range b.Rooms {
		if err := room.Stop(); err != nil {
			b.logger.Errorf("Error stopping room: %s", err)
		}
	}
	b.ctx.Cancel()
	b.ctx.WaitGroup().Wait()
	if err := b.DB.Close(); err != nil {
		b.logger.Errorf("Error closing database: %s", err)
	}
}
