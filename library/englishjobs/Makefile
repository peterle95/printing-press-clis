.PHONY: build test lint install clean

build:
	go build -o bin/englishjobs-pp-cli ./cmd/englishjobs-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/englishjobs-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/englishjobs-pp-mcp ./cmd/englishjobs-pp-mcp

install-mcp:
	go install ./cmd/englishjobs-pp-mcp

build-all: build build-mcp
