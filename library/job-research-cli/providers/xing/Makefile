.PHONY: build test lint install clean

build:
	go build -o bin/xing-pp-cli ./cmd/xing-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/xing-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/xing-pp-mcp ./cmd/xing-pp-mcp

install-mcp:
	go install ./cmd/xing-pp-mcp

build-all: build build-mcp
