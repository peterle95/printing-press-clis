.PHONY: build test vet fmt tidy install clean

build:
	go build -o ./bin/tr ./cmd/tr

test:
	go test ./...

vet:
	go vet ./...

fmt:
	go fmt ./...

tidy:
	go mod tidy

install:
	go install ./cmd/tr

clean:
	rm -rf ./bin

