package rpc

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/rpc"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"

	log "github.com/Sirupsen/logrus"
	"github.com/pierrec/lz4"
	"github.com/yosisa/craft/mux"
	"github.com/yosisa/throttle"
)

func AllocStream(c *rpc.Client, addr string) (id uint32, conn net.Conn, err error) {
	var resp AllocResponse
	if err = c.Call("StreamConn.Alloc", Empty{}, &resp); err != nil {
		return
	}
	id = resp.ID
	if conn, err = mux.DialTimeout("tcp", addr, chanNewStream, dialTimeout); err != nil {
		return
	}
	err = binary.Write(conn, binary.BigEndian, id)
	return
}

func ListContainers(addrs []string, all bool) (map[string]interface{}, error) {
	return CallAll(addrs, func(c *rpc.Client, addr string) (interface{}, error) {
		req := ListContainersRequest{All: all}
		var resp ListContainersResponse
		err := c.Call("Docker.ListContainers", req, &resp)
		return &resp, err
	})
}

func StartContainer(addrs []string, container string) error {
	_, err := CallAll(addrs, func(c *rpc.Client, addr string) (interface{}, error) {
		err := c.Call("Docker.StartContainer", container, &Empty{})
		if err == nil {
			fields := log.Fields{"agent": addr, "container": container}
			log.WithFields(fields).Info("Container started")
		} else {
			err = safeError(err)
		}
		return nil, err
	})
	return err
}

func StopContainer(addrs []string, container string, timeout uint) error {
	_, err := CallAll(addrs, func(c *rpc.Client, addr string) (interface{}, error) {
		req := StopContainerRequest{ID: container, Timeout: timeout}
		err := c.Call("Docker.StopContainer", req, &Empty{})
		if err == nil {
			fields := log.Fields{"agent": addr, "container": container}
			log.WithFields(fields).Info("Container stopped")
		} else {
			err = safeError(err)
		}
		return nil, err
	})
	return err
}

func RestartContainer(addrs []string, container string, timeout uint) error {
	_, err := CallAll(addrs, func(c *rpc.Client, addr string) (interface{}, error) {
		req := RestartContainerRequest{ID: container, Timeout: timeout}
		err := c.Call("Docker.RestartContainer", req, &Empty{})
		if err == nil {
			fields := log.Fields{"agent": addr, "container": container}
			log.WithFields(fields).Info("Container restarted")
		} else {
			err = safeError(err)
		}
		return nil, err
	})
	return err
}

func RemoveContainer(addrs []string, container string, force bool) error {
	_, err := CallAll(addrs, func(c *rpc.Client, addr string) (interface{}, error) {
		req := RemoveContainerRequest{ID: container, Force: force}
		err := c.Call("Docker.RemoveContainer", req, &Empty{})
		if err == nil {
			fields := log.Fields{"agent": addr, "container": container}
			log.WithFields(fields).Info("Container removed")
		} else {
			err = safeError(err)
		}
		return nil, err
	})
	return err
}

func PullImage(addrs []string, image string) error {
	p := newProgress()
	go p.show()
	_, err := CallAll(addrs, func(c *rpc.Client, addr string) (interface{}, error) {
		id, sc, err := AllocStream(c, addr)
		if err != nil {
			return nil, err
		}
		p.add(sc, addr)
		req := PullImageRequest{Image: image, StreamID: id}
		var resp Empty
		err = c.Call("Docker.PullImage", req, &resp)
		return nil, err
	})
	p.wait()
	return err
}

func Logs(addrs []string, container string, follow bool, tail string) error {
	dstout := newLogWriter(os.Stdout)
	dsterr := newLogWriter(os.Stderr)
	closed := make(chan struct{})
	if follow {
		go func() {
			sig := make(chan os.Signal)
			signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
			<-sig
			signal.Stop(sig)
			close(sig)
			close(closed)
			dstout.close()
			dsterr.close()
		}()
	}

	_, err := CallAll(addrs, func(c *rpc.Client, addr string) (interface{}, error) {
		oid, osc, err := AllocStream(c, addr)
		if err != nil {
			return nil, err
		}
		eid, esc, err := AllocStream(c, addr)
		if err != nil {
			return nil, err
		}
		go dstout.read(fmt.Sprintf("[%s] ", addr), osc)
		go dsterr.read(fmt.Sprintf("[%s] ", addr), esc)

		req := LogsRequest{
			Container:   container,
			OutStreamID: oid,
			ErrStreamID: eid,
			Follow:      follow,
			Tail:        tail,
		}
		select {
		// rpc call never return if follow is true
		case call := <-c.Go("Docker.Logs", req, &Empty{}, nil).Done:
			return nil, safeError(call.Error)
		case <-closed:
			return nil, nil
		}
	})
	dstout.wait()
	dsterr.wait()
	return err
}

func LoadImage(addrs []string, r io.Reader, compress bool, bwlimit uint64) error {
	n := int32(len(addrs))
	queue := make(chan net.Conn, len(addrs))
	ready := func() {
		if atomic.AddInt32(&n, -1) == 0 {
			close(queue)
		}
	}
	go func() {
		var ws []io.Writer
		for c := range queue {
			ws = append(ws, c)
			defer c.Close()
		}
		if len(ws) == 0 {
			return
		}
		w := io.MultiWriter(ws...)
		if bwlimit > 0 {
			n := int64(bwlimit / uint64(len(ws)))
			w = throttle.NewWriter(w, n, n)
		}
		if compress {
			w = lz4.NewWriter(w)
			defer w.(*lz4.Writer).Close()
		}
		io.Copy(w, r)
	}()

	_, err := CallAll(addrs, func(c *rpc.Client, addr string) (interface{}, error) {
		id, sc, err := AllocStream(c, addr)
		if err != nil {
			ready()
			return nil, err
		}
		queue <- sc
		ready()
		req := LoadImageRequest{StreamID: id, Compress: compress}
		err = c.Call("Docker.LoadImage", req, &Empty{})
		return nil, err
	})
	return err
}

func LoadImageUsingPipeline(addrs []string, r io.Reader, compress bool, bwlimit uint64) error {
	return loadImageUsingPipeline(addrs, r, compress, false, bwlimit)
}

func connectImagePipeline(addrs []string, r io.Reader, compressed bool) error {
	return loadImageUsingPipeline(addrs, r, false, compressed, 0)
}

func loadImageUsingPipeline(addrs []string, r io.Reader, compress, compressed bool, bwlimit uint64) error {
	next, rest := addrs[0], addrs[1:]
	c, err := Dial("tcp", next)
	if err != nil {
		return err
	}
	id, sc, err := AllocStream(c, next)
	if err != nil {
		return err
	}

	log.WithField("next", next).Info("Sending the image using pipeline")
	go func(w io.Writer) {
		defer sc.Close()
		if bwlimit > 0 {
			w = throttle.NewWriter(w, int64(bwlimit), int64(bwlimit))
		}
		if compress {
			w = lz4.NewWriter(w)
			defer w.(*lz4.Writer).Close()
		}
		io.Copy(w, r)
	}(sc)
	req := LoadImageRequest{StreamID: id, Compress: compress || compressed, Rest: rest}
	return c.Call("Docker.LoadImage", req, &Empty{})
}

type logWriter struct {
	c  chan string
	wg sync.WaitGroup
	rs []io.ReadCloser
	m  sync.Mutex
}

func newLogWriter(w io.Writer) *logWriter {
	lw := &logWriter{
		c: make(chan string),
	}
	go lw.write(w)
	return lw
}

func (l *logWriter) write(w io.Writer) {
	for s := range l.c {
		fmt.Fprintln(w, s)
	}
}

func (l *logWriter) read(prefix string, r io.ReadCloser) {
	l.m.Lock()
	l.rs = append(l.rs, r)
	l.m.Unlock()

	l.wg.Add(1)
	defer l.wg.Done()
	s := bufio.NewScanner(r)
	for s.Scan() {
		l.c <- prefix + s.Text()
	}
	if err := s.Err(); err != nil && !strings.Contains(err.Error(), "use of closed network connection") {
		log.WithField("error", err).Error("Failed to read log stream")
	}
}

func (l *logWriter) wait() {
	l.wg.Wait()
	close(l.c)
}

func (l *logWriter) close() {
	l.m.Lock()
	defer l.m.Unlock()
	for _, r := range l.rs {
		r.Close()
	}
}

func safeError(err error) error {
	if err != nil && strings.Contains(err.Error(), "No such container") {
		return nil
	}
	return err
}
