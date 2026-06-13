.PHONY: build test lint install clean

build:
	go build -o bin/linkedin-pp-cli ./cmd/linkedin-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/linkedin-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/linkedin-pp-mcp ./cmd/linkedin-pp-mcp

install-mcp:
	go install ./cmd/linkedin-pp-mcp

build-all: build build-mcp
