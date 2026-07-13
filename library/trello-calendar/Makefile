.PHONY: build test test-race test-integration lint fmt vet check install clean build-mcp install-mcp build-all

build:
	go build -o bin/trello-calendar-pp-cli ./cmd/trello-calendar-pp-cli

test:
	go test ./...

test-race:
	go test -race ./...

test-integration:
	go test -tags=integration ./integration

lint:
	golangci-lint run

fmt:
	go fmt ./...

vet:
	go vet ./...

check: fmt test vet lint build-all

install:
	go install ./cmd/trello-calendar-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/trello-calendar-pp-mcp ./cmd/trello-calendar-pp-mcp

install-mcp:
	go install ./cmd/trello-calendar-pp-mcp

build-all: build build-mcp
