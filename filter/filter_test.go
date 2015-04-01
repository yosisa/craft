package filter

import (
	"testing"

	"github.com/yosisa/craft/rpc"
)

func TestFilter(t *testing.T) {
	testcase := []struct {
		expr     string
		cap      *rpc.Capability
		expected bool
	}{
		{"A@name1", &rpc.Capability{Agent: "name1"}, true},
		{"not A@name1", &rpc.Capability{Agent: "name1"}, false},
		{"not A@name1 or A@name1", &rpc.Capability{Agent: "name1"}, true},
		{"not (A@name1 or A@name1)", &rpc.Capability{Agent: "name1"}, false},
		{"A@^api- and A@-dev$", &rpc.Capability{Agent: "api-dev"}, true},
		{"A@^api- and A@-dev$", &rpc.Capability{Agent: "api-stg"}, false},
		{"A@^api-(dev|stg)-[0-9]+$", &rpc.Capability{Agent: "api-dev-10"}, true},
		{"A@^api-(dev|stg)-[0-9]+$", &rpc.Capability{Agent: "api-stg-10"}, true},
		{"A@^api-(dev|stg)-[0-9]+$", &rpc.Capability{Agent: "api-prd-10"}, false},
	}
	for _, c := range testcase {
		e, err := Parse(c.expr)
		if err != nil {
			t.Fatalf("expr %s %v", c.expr, err)
		}
		if r := e.Eval(c.cap); r != c.expected {
			t.Fatalf("expr %s expected %v, but %v", c.expr, c.expected, r)
		}
	}
}

func TestInvalidFilter(t *testing.T) {
	for _, expr := range []string{
		"name1",
		"A@name[1",
		"A@name(1",
	} {
		if _, err := Parse(expr); err == nil {
			t.Fatalf("%s must be error", expr)
		}
	}
}
