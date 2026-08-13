BINARY := chaintui

.PHONY: all build test clean

all: build

build:
	go build -o $(BINARY) ./cmd

test:
	go test ./...

clean:
	rm -f $(BINARY)
