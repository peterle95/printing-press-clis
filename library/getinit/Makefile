.PHONY: build test lint install clean

build:
	go build -o bin/getinit-pp-cli ./cmd/getinit-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/getinit-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/getinit-pp-mcp ./cmd/getinit-pp-mcp

install-mcp:
	go install ./cmd/getinit-pp-mcp

build-all: build build-mcp
