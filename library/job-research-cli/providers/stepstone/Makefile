.PHONY: build test lint install clean

build:
	go build -o bin/stepstone-pp-cli ./cmd/stepstone-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/stepstone-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/stepstone-pp-mcp ./cmd/stepstone-pp-mcp

install-mcp:
	go install ./cmd/stepstone-pp-mcp

build-all: build build-mcp
