package rpc

import "testing"

func TestShortHostname(t *testing.T) {
	data := []struct {
		addr     string
		omitPort bool
		expected string
	}{
		{"foo.example.com:7300", false, "foo:7300"},
		{"foo.example.com:7300", true, "foo"},
		{"localhost:7300", true, "localhost"},
		{"localhost", false, "localhost"},
		{"127.0.0.1:7300", false, "127.0.0.1:7300"},
		{"127.0.0.1:7300", true, "127.0.0.1"},
	}
	for i, test := range data {
		v := ShortHostname(test.addr, test.omitPort)
		if v != test.expected {
			t.Errorf("index %d failed: got %s, expected %s", i, v, test.expected)
		}
	}
}
