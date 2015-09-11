package gobot

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"euphoria.io/heim/proto"
	"github.com/gorilla/websocket"
)

// Connection is an interface primarily designed to allow for a mock connecting
// during testing.
type Connection interface {
	Connect(r *Room) error
	SendJSON(r *Room, msg interface{}) (string, error)
	ReceiveJSON(r *Room, p chan *proto.Packet)
	Close() error
}

// WSConnection is a type that satisfies the Connection interface and manages
// a websocket connection to a euphoria room.
type WSConnection struct {
	conn *websocket.Conn
	m    sync.Mutex
}

func (ws *WSConnection) connectOnce(r *Room, try int) error {
	r.Logger.Infof("Connecting to room %s...", r.RoomName)
	tlsConn, err := tls.Dial("tcp", "euphoria.io:443", &tls.Config{})
	if err != nil {
		return err
	}
	roomURL, err := url.Parse(fmt.Sprintf("wss://euphoria.io/room/%s/ws", r.RoomName))
	if err != nil {
		return err
	}
	wsConn, _, err := websocket.NewClient(tlsConn, roomURL, http.Header{}, 4096, 4096)
	if err != nil {
		return err
	}
	if err := r.Ctx.Check("connectOnce", try); err != nil {
		return err
	}
	ws.conn = wsConn
	if r.password != "" {
		if _, err := r.sendAuth(); err != nil {
			return err
		}
	}
	if _, err := r.sendNick(); err != nil {
		return err
	}
	return nil
}

func (ws *WSConnection) connectUnlocked(r *Room) error {
	if err := r.Ctx.Check("Connect"); err != nil {
		return err
	}
	err := ws.connectOnce(r, 0)
	if err == nil {
		return nil
	}
	r.Logger.Warningf("Error connecting on first try: %s", err)
	for count := 1; count < MAXRETRIES; count++ {
		err = ws.connectOnce(r, count)
		if err == nil {
			return nil
		}
		r.Logger.Warningf("Error connecting on retry #%s: %s", err)
		time.Sleep(time.Duration(count) * time.Second * 5)
	}
	r.Logger.Errorf("Error connecting to websocket: %s", err)
	return err
}

// Connect tries to connect to a euphoria room with multiple retries upon error.
func (ws *WSConnection) Connect(r *Room) error {
	ws.m.Lock()
	defer ws.m.Unlock()
	return ws.connectUnlocked(r)
}

// SendJSON sends a packet through the websocket connection.
func (ws *WSConnection) SendJSON(r *Room, msg interface{}) (string, error) {
	ws.m.Lock()
	defer ws.m.Unlock()
	if err := r.Ctx.Check("SendJSON"); err != nil {
		return "", err
	}
	if ws.conn == nil {
		if err := ws.connectUnlocked(r); err != nil {
			return "", err
		}
	}
	if err := ws.conn.WriteJSON(msg); err != nil {
		err = ws.connectUnlocked(r)
		if err != nil {
			return "", err
		}
		if err := ws.conn.WriteJSON(msg); err != nil {
			r.Logger.Warningf("Error writing JSON: %s", err)
			return "", err
		}
	}
	return strconv.Itoa(r.msgID), nil
}

// ReceiveJSON reads a message from the websocket and unmarshals it into the
// provided packet.
func (ws *WSConnection) ReceiveJSON(r *Room, p chan *proto.Packet) {
	ws.m.Lock()
	defer ws.m.Unlock()
	if ws.conn == nil {
		if err := ws.connectUnlocked(r); err != nil {
			r.Logger.Errorf("Error connecting to euphoria: %s", err)
			return
		}
	}
	var msg proto.Packet
	if err := ws.conn.ReadJSON(&msg); err != nil {
		r.Logger.Warningf("Error reading JSON, reconnecting: %s", err)
		if err := ws.connectUnlocked(r); err != nil {
			r.Logger.Errorf("Error reconnecting: %s", err)
		}
		if r.Ctx.Alive() {
			p <- nil
		}
		return
	}
	if r.Ctx.Alive() {
		p <- &msg
	}
}

func (ws *WSConnection) closeUnlocked() error {
	if ws.conn == nil {
		return nil
	}
	ws.conn.WriteControl(websocket.CloseMessage, []byte{}, time.Now().Add(time.Second))
	return ws.conn.Close()
}

// Close simply closes the websocket connection, if it is connected.
func (ws *WSConnection) Close() error {
	ws.m.Lock()
	defer ws.m.Unlock()
	return ws.closeUnlocked()
}
