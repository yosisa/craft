package rpc

import (
	"encoding/json"
	"io"
	"log"
	"net"
	"os"

	"github.com/nsf/termbox-go"
)

type jsonMessage struct {
	ID       string
	Status   string
	Progress string
}

func showProgress(c net.Conn) {
	if err := termbox.Init(); err != nil {
		io.Copy(os.Stdout, c)
		return
	}
	defer termbox.Close()

	var ids []string
	progress := make(map[string]string)
	dec := json.NewDecoder(c)
	for {
		var msg jsonMessage
		if err := dec.Decode(&msg); err != nil {
			if err != io.EOF {
				log.Print(err)
			}
			return
		}
		if _, ok := progress[msg.ID]; !ok {
			ids = append(ids, msg.ID)
		}
		progress[msg.ID] = msg.Status + " " + msg.Progress

		termbox.Clear(termbox.ColorDefault, termbox.ColorDefault)
		for y, id := range ids {
			msg := id + " " + progress[id]
			for x := 0; x < len(msg); x++ {
				termbox.SetCell(x, y, rune(msg[x]), termbox.ColorDefault, termbox.ColorDefault)
			}
		}
		termbox.Flush()
	}
}
