.PHONY: build test lint install clean

build:
	go build -o bin/remotive-jobs-pp-cli ./cmd/remotive-jobs-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/remotive-jobs-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/remotive-jobs-pp-mcp ./cmd/remotive-jobs-pp-mcp

install-mcp:
	go install ./cmd/remotive-jobs-pp-mcp

build-all: build build-mcp
