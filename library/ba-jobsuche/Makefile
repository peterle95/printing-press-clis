.PHONY: build test lint install clean

build:
	go build -o bin/ba-jobsuche-pp-cli ./cmd/ba-jobsuche-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/ba-jobsuche-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/ba-jobsuche-pp-mcp ./cmd/ba-jobsuche-pp-mcp

install-mcp:
	go install ./cmd/ba-jobsuche-pp-mcp

build-all: build build-mcp
