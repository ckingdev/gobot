// Package handlers provides several pre-baked gobot.Handlers for convenience.
package handlers

import (
	"os"
	"io/ioutil"
	"strings"

	"euphoria.io/heim/proto"
	"github.com/cpalone/gobot"
	"github.com/dustin/go-humanize"
	"encoding/json"
)

// NotifyHandler responds to a send-event starting with "!tell"
type NotifyHandler struct{}

// HandleIncoming satisfies the Handler interface.
func (h *NotifyHandler) HandleIncoming(r *gobot.Room, p *proto.Packet) (*proto.Packet, error) {
	r.Logger.Debugln("Checking for tell command...")

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

	results := read(payload.Sender.Name)
	if *results != "" {
		if _, err := r.SendText(&payload.ID, *results); err != nil {
			return nil, err
		}
	}

	if !strings.HasPrefix(payload.Content, "!tell @") {
		return nil, nil
	}
	data := strings.SplitN(payload.Content, " ", 3)
	notice := Notice{
		Time: payload.UnixTime,
		Sender: payload.Sender.Name,
		Receiver: strings.Replace(data[1], "@", "", 1),
		Message: data[2],
	}
	go notice.write()

	return nil, nil
}

// Run is a no-op- the SampleHandler does not need to run continuously, only in
// response to an incoming packet.
func (h *NotifyHandler) Run(r *gobot.Room) {
	return
}

// Stop is also a no-op- there is nothing to stop.
func (h *NotifyHandler) Stop(r *gobot.Room) {
	return
}

const notice_directory = "notices/"

type Notice struct{
	Message		string 	`json:"message"`
	Sender 		string	`json:"sender"`
	Receiver 	string	`json:"receiver"`
	Time	proto.Time	`json:"timestamp"`
}

func (n *Notice) display() (*string) {
	timeago := humanize.Time(n.Time.StdTime())
	formatted := "[" + timeago + "] @" + n.Sender + " said: " + n.Message + "\n"
	return &formatted
}

func (notice *Notice) write() {
	if _, err := os.Stat(notice_directory); os.IsNotExist(err) {
		err = os.MkdirAll(notice_directory, 0711)
		check(err)
	}

	f, err := os.OpenFile(notice_directory + notice.Receiver, os.O_WRONLY | os.O_CREATE, 0644);
	check(err)
	defer f.Close()

	fs, err := f.Stat()
	check(err)

	bytes, _ := json.Marshal(&notice)
	if fs.Size() == 0 {
		f.WriteString("[")
		f.Write(bytes)
		f.WriteString("]")
	} else {
		f.Seek(-1, os.SEEK_END)
		f.WriteString(",")
		f.Write(bytes)
		f.WriteString("]")
	}

	f.Sync()
}

func read(receiver string) (*string) {
	file := notice_directory + receiver
	if _, err := os.Stat(file); os.IsNotExist(err) {
		return nil
	}

	buff, err := ioutil.ReadFile(file)
	check(err)

	var notices []Notice
	json.Unmarshal(buff, &notices)

//	err = os.Remove(file)
//	check(err)

	results := ""
	for _, notice := range notices {
		results += *notice.display()
	}

	return &results
}

func check(e error) {
	if e != nil {
		panic(e)
	}
}