package filter

import (
	"errors"
	"regexp"
	"strings"

	"github.com/yosisa/craft/rpc"
)

var (
	ErrInvalidSyntax  = errors.New("Error invalid syntax")
	ErrStackRemaining = errors.New("Error stack remaining")
)

type Evaluator interface {
	Eval(*rpc.Capability) bool
}

type not struct {
	e Evaluator
}

func (e *not) Eval(cap *rpc.Capability) bool {
	return !e.e.Eval(cap)
}

type and struct {
	l Evaluator
	r Evaluator
}

func (e *and) Eval(cap *rpc.Capability) bool {
	return e.l.Eval(cap) && e.r.Eval(cap)
}

type or struct {
	l Evaluator
	r Evaluator
}

func (e *or) Eval(cap *rpc.Capability) bool {
	return e.l.Eval(cap) || e.r.Eval(cap)
}

type agent struct {
	re *regexp.Regexp
}

func (e *agent) Eval(cap *rpc.Capability) bool {
	return e.re.MatchString(cap.Agent)
}

type label struct {
	name  string
	value string
}

func (e *label) Eval(cap *rpc.Capability) bool {
	v, ok := cap.Labels[e.name]
	return ok && v == e.value
}

func (p *parser) pushStack(e Evaluator) {
	p.stack = append(p.stack, e)
}

func (p *parser) popStack() (e Evaluator) {
	n := len(p.stack)
	e, p.stack = p.stack[n-1], p.stack[:n-1]
	return
}

func (p *parser) Agent(s string) {
	re, err := regexp.Compile(s)
	if err != nil {
		p.err = err
	}
	p.pushStack(&agent{re})
}

func (p *parser) Label(s string) {
	parts := strings.SplitN(s, ":", 2)
	p.pushStack(&label{name: parts[0], value: parts[1]})
}

func (p *parser) Not() {
	p.pushStack(&not{p.popStack()})
}

func (p *parser) And() {
	p.pushStack(&and{p.popStack(), p.popStack()})
}

func (p *parser) Or() {
	p.pushStack(&or{p.popStack(), p.popStack()})
}

func Parse(s string) (Evaluator, error) {
	p := parser{Buffer: s}
	p.Init()
	if err := p.Parse(); err != nil {
		return nil, ErrInvalidSyntax
	}
	p.Execute()
	if p.err != nil {
		return nil, p.err
	}
	if len(p.stack) > 1 {
		return nil, ErrStackRemaining
	}
	return p.stack[0], nil
}
