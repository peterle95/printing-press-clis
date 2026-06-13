.PHONY: build test lint install clean

build:
	go build -o bin/doctolib-pp-cli ./cmd/doctolib-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/doctolib-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/doctolib-pp-mcp ./cmd/doctolib-pp-mcp

install-mcp:
	go install ./cmd/doctolib-pp-mcp

build-all: build build-mcp
