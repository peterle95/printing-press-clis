.PHONY: build test fmt tidy clean install

build:
	mkdir -p bin
	go build -o ./bin/mail-pp-cli ./cmd/mail-pp-cli

test:
	go test ./...

fmt:
	go fmt ./...

tidy:
	go mod tidy

install: build
	mkdir -p $(HOME)/go/bin
	cp ./bin/mail-pp-cli $(HOME)/go/bin/mail-pp-cli

clean:
	rm -rf bin
