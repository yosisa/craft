SOURCES := $(shell find . -name '*.go')
BIN := craft

all: deps $(BIN)

$(BIN): filter/parser.peg.go $(SOURCES)
	go build

deps:
	go get github.com/pointlander/peg
	go get -t ./...

test: deps
	go test ./...

clean:
	-rm $(BIN)

.peg.peg.go:
	peg -switch -inline $<

.SUFFIXES: .peg .peg.go

.PHONY: all deps test clean
