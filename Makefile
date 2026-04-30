BINARY := chaintui

.PHONY: all build clean

all: build

build:
	go build -o $(BINARY) ./cmd

clean:
	rm -f $(BINARY)
