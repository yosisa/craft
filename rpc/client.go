package rpc

import (
	"log"
	"net/rpc"

	"github.com/yosisa/craft/mux"
)

func PullImage(addrs []string, image string) error {
	req := PullImageRequest{Image: image}
	p := newProgress()
	go p.show()
	CallAll(addrs, func(c *rpc.Client, addr string) (interface{}, error) {
		go func() {
			sc, err := mux.DialTimeout("tcp", addr, chanNewStream, dialTimeout)
			if err != nil {
				log.Print(err)
				return
			}
			p.add(sc, addr)
		}()
		var resp Empty
		err := c.Call("Docker.PullImage", req, &resp)
		return nil, err
	})
	p.wait()
	return nil
}
