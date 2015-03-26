package rpc

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"

	"github.com/nsf/termbox-go"
)

type jsonMessage struct {
	ID       string
	Status   string
	Progress string
}

func showProgress(c net.Conn) {
	var ids []string
	progress := make(map[string]string)
	update := func(msg *jsonMessage) {
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

	first := true
	dec := json.NewDecoder(c)
	for {
		var msg jsonMessage
		if err := dec.Decode(&msg); err != nil {
			if err != io.EOF {
				log.Print(err)
			}
			return
		}
		if first {
			first = false
			if err := termbox.Init(); err == nil {
				defer termbox.Close()
			} else {
				update = func(msg *jsonMessage) {
					fmt.Fprintln(c, msg.Progress)
				}
			}
		}
		update(&msg)
	}
}
