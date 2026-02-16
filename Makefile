.PHONY: build test install clean

BINARY_NAME=claude-wrapper
INSTALL_PATH=/usr/local/bin

build:
	go build -o $(BINARY_NAME) .

test:
	go test -v -race -cover ./...

install: build
	sudo install -m 755 $(BINARY_NAME) $(INSTALL_PATH)/$(BINARY_NAME)

clean:
	rm -f $(BINARY_NAME)
	go clean

lint:
	go vet ./...
	gofmt -s -w .

run: build
	./$(BINARY_NAME)
