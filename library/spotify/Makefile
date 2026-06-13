.PHONY: build test fmt tidy

build:
	mkdir -p bin
	go build -o ./bin/spotify-pp-cli ./cmd/spotify-pp-cli

test:
	go test ./...

fmt:
	go fmt ./...

tidy:
	go mod tidy
