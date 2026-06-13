.PHONY: build test lint install clean

build:
	go build -o bin/arbeitnow-jobs-pp-cli ./cmd/arbeitnow-jobs-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/arbeitnow-jobs-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/arbeitnow-jobs-pp-mcp ./cmd/arbeitnow-jobs-pp-mcp

install-mcp:
	go install ./cmd/arbeitnow-jobs-pp-mcp

build-all: build build-mcp
