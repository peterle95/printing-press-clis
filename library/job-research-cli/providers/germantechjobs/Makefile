.PHONY: build test lint install clean

build:
	go build -o bin/germantechjobs-pp-cli ./cmd/germantechjobs-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/germantechjobs-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/germantechjobs-pp-mcp ./cmd/germantechjobs-pp-mcp

install-mcp:
	go install ./cmd/germantechjobs-pp-mcp

build-all: build build-mcp
