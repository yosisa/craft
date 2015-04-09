package rpc

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
)

type atomicWriter struct {
	c       chan chan string
	wg      sync.WaitGroup
	rs      []io.ReadCloser
	m       sync.Mutex
	closed  chan struct{}
	maxHold time.Duration
	maxLine int
}

func newAtomicWriter(w io.Writer) *atomicWriter {
	aw := &atomicWriter{
		c:       make(chan chan string),
		closed:  make(chan struct{}),
		maxHold: 10 * time.Millisecond,
		maxLine: 20,
	}
	go aw.write(w)
	return aw
}

func (w *atomicWriter) write(out io.Writer) {
	for c := range w.c {
		timeout := time.After(w.maxHold)
	INNER:
		for i := 0; i < w.maxLine; i++ {
			select {
			case s, ok := <-c:
				if !ok {
					break INNER
				}
				fmt.Fprintln(out, s)
			case <-timeout:
				break INNER
			}
		}
	}
	close(w.closed)
}

func (w *atomicWriter) read(prefix string, r io.ReadCloser) {
	w.wg.Add(1)
	defer w.wg.Done()
	w.m.Lock()
	w.rs = append(w.rs, r)
	w.m.Unlock()

	c := make(chan string)
	defer close(c)
	s := bufio.NewScanner(r)
	var line string
	for s.Scan() {
		line = prefix + s.Text()
	SCANNED:
		w.c <- c
		c <- line
		for s.Scan() {
			line = prefix + s.Text()
			select {
			case c <- line:
			case <-time.After(w.maxHold):
				goto SCANNED
			}
		}
		break
	}
	if err := s.Err(); err != nil && !strings.Contains(err.Error(), "use of closed network connection") {
		log.WithField("error", err).Error("Failed to read log stream")
	}
}

func (w *atomicWriter) wait() {
	w.wg.Wait()
	close(w.c)
	<-w.closed
}

func (w *atomicWriter) close() {
	w.m.Lock()
	defer w.m.Unlock()
	for _, r := range w.rs {
		r.Close()
	}
}

func ShortHostname(s string, omitPort bool) string {
	host, port, err := net.SplitHostPort(s)
	if err != nil {
		host = s
	}
	if net.ParseIP(host) == nil {
		// hostname (not an IP address)
		parts := strings.SplitN(host, ".", 2)
		host = parts[0]
	}
	if omitPort || port == "" {
		return host
	}
	return net.JoinHostPort(host, port)
}
