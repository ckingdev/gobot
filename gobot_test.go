package gobot

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"euphoria.io/heim/proto"
	"github.com/Sirupsen/logrus"
	. "gopkg.in/check.v1"
)

type MockConn struct {
	outgoing chan *proto.Packet
	incoming chan *proto.Packet
}

func (c *MockConn) Connect(r *Room) error {
	return r.Ctx.Check("Connect")
}

func (c *MockConn) SendJSON(r *Room, msg interface{}) (string, error) {
	p, ok := msg.(*proto.Packet)
	if !ok {
		return "", fmt.Errorf("Could not assert message as packet.")
	}
	p.ID = strconv.Itoa(r.msgID)
	c.outgoing <- p
	return strconv.Itoa(r.msgID), nil
}

func (c *MockConn) ReceiveJSON(r *Room, p chan *proto.Packet) {
	select {
	case msg := <-c.incoming:
		p <- msg
	case <-r.Ctx.Done():
		return
	}
}

func (c *MockConn) Close() error {

	return nil
}

func Test(t *testing.T) { TestingT(t) }

type ConnSuite struct{}

var _ = Suite(&ConnSuite{})

// PongHandler responds to a send-event starting with "!ping" and returns a send
// command containing "pong!".
type PongHandler struct{}

// HandleIncoming satisfies the Handler interface.
func (ph *PongHandler) HandleIncoming(r *Room, p *proto.Packet) (*proto.Packet, error) {
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
func (ph *PongHandler) Run(r *Room) {
	return
}

func (ph *PongHandler) Stop(r *Room) {
	return
}

func BasicWSBot() (*Bot, error) {
	bcfg := BotConfig{
		Name:   "test",
		DbPath: "test.db",
	}
	b, err := NewBot(bcfg)
	if err != nil {
		return nil, err
	}
	rcfg := RoomConfig{
		Name:         "test",
		Password:     "",
		BotName:      "TestBot",
		AddlHandlers: []Handler{},
		Conn:         &WSConnection{},
	}
	b.AddRoom(rcfg)
	return b, nil
}

func BasicMockBot() (*Bot, *MockConn, error) {
	bcfg := BotConfig{
		Name:   "test",
		DbPath: "test.db",
	}
	b, err := NewBot(bcfg)
	if err != nil {
		return nil, nil, err
	}
	conn := &MockConn{
		outgoing: make(chan *proto.Packet),
		incoming: make(chan *proto.Packet),
	}
	rcfg := RoomConfig{
		Name:         "test",
		Password:     "",
		BotName:      "TestBot",
		AddlHandlers: []Handler{},
		Conn:         conn,
	}
	b.AddRoom(rcfg)
	return b, conn, nil
}

func MockBotWithPong() (*Bot, *MockConn, error) {
	bcfg := BotConfig{
		Name:   "test",
		DbPath: "test.db",
	}
	b, err := NewBot(bcfg)
	if err != nil {
		return nil, nil, err
	}
	conn := &MockConn{
		outgoing: make(chan *proto.Packet),
		incoming: make(chan *proto.Packet),
	}
	rcfg := RoomConfig{
		Name:         "test",
		Password:     "",
		BotName:      "TestBot",
		AddlHandlers: []Handler{&PongHandler{}},
		Conn:         conn,
	}
	b.AddRoom(rcfg)
	return b, conn, nil
}

func (s *ConnSuite) TestGoodConn(c *C) {
	b, err := BasicWSBot()
	defer b.Stop()
	c.Check(err, IsNil)
	c.Check(b.Rooms["test"].conn.Connect(b.Rooms["test"]), Equals, nil)
	pchan := make(chan *proto.Packet)
	go b.Rooms["test"].conn.ReceiveJSON(b.Rooms["test"], pchan)
	p := <-pchan
	c.Check(p, Not(Equals), nil)
}

func (s *ConnSuite) TestSoak(c *C) {
	b, err := BasicWSBot()
	defer b.Stop()
	c.Check(err, IsNil)
	go b.Rooms["test"].Run()
	time.Sleep(time.Second * 3)
}

type BotSuite struct{}

var _ = Suite(&BotSuite{})

func (s *BotSuite) TestNew(c *C) {
	b, err := BasicWSBot()
	defer b.Stop()
	c.Check(err, IsNil)
	c.Check(b, NotNil)
}

func (s *BotSuite) TestRun(c *C) {
	logrus.Debugln("1")
	b, _, err := BasicMockBot()
	defer b.Stop()
	logrus.Debugln("2")
	c.Check(err, IsNil)
	logrus.Debugln("3")
	c.Check(b, NotNil)
	go b.RunAllRooms()
	logrus.Debugln("4")
	time.Sleep(time.Second)
	logrus.Debugln("Stopping...")
}

func (s *BotSuite) TestPingHandle(c *C) {
	b, conn, err := BasicMockBot()
	c.Check(err, IsNil)
	c.Check(b, NotNil)
	defer b.Stop()
	go b.Rooms["test"].Run()
	time.Sleep(time.Second)
	packet := proto.Packet{
		Type: proto.PingEventType,
	}
	t := time.Now().Unix()
	payload := proto.PingCommand{
		UnixTime: proto.Time(time.Unix(t, 0)),
	}
	marshalled, err := json.Marshal(payload)
	packet.Data.UnmarshalJSON(marshalled)

	conn.incoming <- &packet
	msg := <-conn.outgoing
	p, err := msg.Payload()
	c.Check(err, IsNil)
	pingreply, ok := p.(*proto.PingReply)
	c.Check(ok, Equals, true)
	c.Check(time.Time(pingreply.UnixTime).Unix(), Equals, t)
}

func (s *BotSuite) TestPongHandler(c *C) {
	b, conn, err := MockBotWithPong()
	c.Check(err, IsNil)
	defer b.Stop()
	go b.RunAllRooms()
	packet := proto.Packet{
		Type: proto.SendEventType,
	}
	payload := proto.SendEvent{
		Content: "!ping",
	}
	marshalled, err := json.Marshal(payload)
	packet.Data.UnmarshalJSON(marshalled)

	conn.incoming <- &packet
	msg := <-conn.outgoing
	c.Check(msg.Type, Equals, proto.SendType)
	p, err := msg.Payload()
	c.Check(err, IsNil)
	reply, ok := p.(*proto.SendCommand)
	c.Check(ok, Equals, true)
	c.Check(reply.Content, Equals, "pong!")
}

func (s *BotSuite) TestFailedConnect(c *C) {
	b, _, err := BasicMockBot()
	c.Check(err, IsNil)
	c.Check(b, NotNil)
	defer b.Stop()
	ctrl := b.Rooms["test"].Ctx.Breakpoint("Connect")
	testErr := fmt.Errorf("Test error: failed to connect.")
	go func() {
		<-ctrl
		ctrl <- testErr
	}()
	go b.RunAllRooms()
	time.Sleep(time.Second)
	c.Check(b.Rooms["test"].Ctx.Alive(), Equals, false)
}

func (s *BotSuite) TestFailedConnectOnce(c *C) {
	b, err := BasicWSBot()
	c.Check(err, IsNil)
	c.Check(b, NotNil)
	defer b.Stop()
	ctrl := b.Rooms["test"].Ctx.Breakpoint("connectOnce", 0)
	testErr := fmt.Errorf("Test error: failed to connect.")
	go func() {
		<-ctrl
		ctrl <- testErr
	}()
	go b.RunAllRooms()
	time.Sleep(time.Second)
	c.Check(b.Rooms["test"].Ctx.Alive(), Equals, true)
}

func (s *BotSuite) TestFailedSendJSON(c *C) {
	b, err := BasicWSBot()
	c.Check(err, IsNil)
	c.Check(b, NotNil)
	defer b.Stop()
	ctrl := b.Rooms["test"].Ctx.Breakpoint("SendJSON")
	testErr := fmt.Errorf("Test error: failed to send.")
	go func() {
		<-ctrl
		ctrl <- testErr
	}()
	go b.RunAllRooms()
	b.Rooms["test"].SendText(nil, "test!")
	time.Sleep(time.Second)
	c.Check(b.Rooms["test"].Ctx.Alive(), Equals, false)
}

func (s *BotSuite) TestSendNoConnect(c *C) {
	b, err := BasicWSBot()
	c.Check(err, IsNil)
	c.Check(b, NotNil)
	defer b.Stop()
	ctrl := b.Rooms["test"].Ctx.Breakpoint("SendJSON")
	testErr := fmt.Errorf("Test error: failed to send.")
	go func() {
		<-ctrl
		ctrl <- testErr
	}()
	go b.Rooms["test"].sendLoop()
	b.Rooms["test"].Ctx.WaitGroup().Add(1)
	_, err = b.Rooms["test"].sendAuth()
	if err != nil {
		panic(err)
	}
	c.Check(b.Rooms["test"].Ctx.Alive(), Equals, true)
}

func (s *BotSuite) TestSendBadConnect(c *C) {
	b, err := BasicWSBot()
	c.Check(err, IsNil)
	c.Check(b, NotNil)
	defer b.Stop()
	ctrl := b.Rooms["test"].Ctx.Breakpoint("Connect")
	testErr := fmt.Errorf("Test error: failed to connect.")
	go func() {
		<-ctrl
		ctrl <- testErr
	}()
	go b.Rooms["test"].sendLoop()
	b.Rooms["test"].Ctx.WaitGroup().Add(1)
	_, err = b.Rooms["test"].sendAuth()
	if err != nil {
		panic(err)
	}
	time.Sleep(time.Second)
	c.Check(b.Rooms["test"].Ctx.Alive(), Equals, false)
}
