package rpc

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"sync"

	"github.com/nsf/termbox-go"
)

type jsonMessage struct {
	ID       string
	Status   string
	Progress string
}

type progressItem struct {
	c        net.Conn
	addr     string
	ids      []string
	progress map[string]string
	update   func()
}

func (p *progressItem) start() {
	dec := json.NewDecoder(p.c)
	for {
		var msg jsonMessage
		if err := dec.Decode(&msg); err != nil {
			if err != io.EOF {
				log.Print(err)
			}
			return
		}
		if _, ok := p.progress[msg.ID]; !ok && msg.ID != "" {
			p.ids = append(p.ids, msg.ID)
		}
		p.progress[msg.ID] = msg.Status + " " + msg.Progress
		p.update()
	}
}

type progress struct {
	items        []*progressItem
	wg           sync.WaitGroup
	once         sync.Once
	drawRequests chan struct{}
	termboxUsed  bool
}

func newProgress() *progress {
	return &progress{drawRequests: make(chan struct{}, 1)}
}

func (p *progress) add(c net.Conn, addr string) {
	pi := &progressItem{
		c:        c,
		addr:     addr,
		progress: make(map[string]string),
		update:   p.update,
	}
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		pi.start()
	}()
	p.items = append(p.items, pi)
}

func (p *progress) update() {
	select {
	case p.drawRequests <- struct{}{}:
	default:
	}
}

func (p *progress) show() {
	for _ = range p.drawRequests {
		p.once.Do(func() {
			if err := termbox.Init(); err == nil {
				p.termboxUsed = true
			}
		})
		if !p.termboxUsed {
			continue
		}
		_, maxRows := termbox.Size()
		rpi := maxRows / len(p.items)
		termbox.Clear(termbox.ColorDefault, termbox.ColorDefault)
		var row int
		for _, pi := range p.items {
			writeLine(row, fmt.Sprintf("[%s] %s", pi.addr, pi.progress[""]))
			row++
			for i, n := 0, 0; i < len(pi.ids) && n < rpi; i++ {
				id := pi.ids[i]
				if !strings.Contains(pi.progress[id], "complete") &&
					!strings.Contains(pi.progress[id], "Already exists") {
					writeLine(row, "  "+id+" "+pi.progress[id])
					row++
					n++
				}
			}
		}
		termbox.Flush()
	}
}

func (p *progress) wait() {
	p.wg.Wait()
	close(p.drawRequests)
	if p.termboxUsed {
		termbox.Close()
	}
}

func writeLine(row int, s string) {
	for col := 0; col < len(s); col++ {
		termbox.SetCell(col, row, rune(s[col]), termbox.ColorDefault, termbox.ColorDefault)
	}
}
