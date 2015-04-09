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
		{"L@env:prd", &rpc.Capability{Labels: map[string]string{"env": "prd"}}, true},
		{"L@env:dev", &rpc.Capability{Labels: map[string]string{"env": "prd"}}, false},
		{"L@role:api", &rpc.Capability{Labels: map[string]string{"env": "prd"}}, false},
		{"L@env:prd or L@role:db", &rpc.Capability{Labels: map[string]string{"role": "db"}}, true},
		{"L@env:prd and L@role:db", &rpc.Capability{Labels: map[string]string{"role": "db"}}, false},
		{"L@env:prd and L@role:db", &rpc.Capability{Labels: map[string]string{"env": "prd", "role": "db"}}, true},
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
		"L@env",
		"L@env:",
		"L@:prd",
		"L@:",
		"L@env:key:val",
	} {
		if _, err := Parse(expr); err == nil {
			t.Fatalf("%s must be error", expr)
		}
	}
}
