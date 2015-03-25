package rpc

import (
	"testing"

	. "gopkg.in/check.v1"
)

func Test(t *testing.T) { TestingT(t) }

type ExLinkSuite struct{}

var _ = Suite(&ExLinkSuite{})

func (s *ExLinkSuite) TestEnv(c *C) {
	l := ExLink{Name: "api", Exposed: "80/tcp", Addr: "192.168.1.1", Port: 80}
	c.Assert(l.Env(), DeepEquals, map[string]string{
		"API_PORT":              "tcp://192.168.1.1:80",
		"API_PORT_80_TCP":       "tcp://192.168.1.1:80",
		"API_PORT_80_TCP_ADDR":  "192.168.1.1",
		"API_PORT_80_TCP_PORT":  "80",
		"API_PORT_80_TCP_PROTO": "tcp",
	})
}
