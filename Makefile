PROJECT_NAME = claude-wrapper
VERSION = $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME = $(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS = -ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)"
INSTALL_PATH = /usr/local/bin

.PHONY: build build-linux build-all test lint install clean run deploy deploy-patch deploy-minor deploy-major release release-patch release-minor release-major

build:
	go build $(LDFLAGS) -o bin/$(PROJECT_NAME) .

build-linux:
	@mkdir -p dist
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o dist/$(PROJECT_NAME)-linux-amd64 .

build-all: build-linux

test:
	go test -v -race -cover ./...

lint:
	go vet ./...
	gofmt -s -w .

install: build
	sudo install -m 755 bin/$(PROJECT_NAME) $(INSTALL_PATH)/$(PROJECT_NAME)

clean:
	rm -rf bin/ dist/
	go clean

run: build
	./bin/$(PROJECT_NAME)

deploy:
	./scripts/deploy.sh

deploy-patch:
	./scripts/deploy.sh patch

deploy-minor:
	./scripts/deploy.sh minor

deploy-major:
	./scripts/deploy.sh major

release:
	./scripts/release.sh

release-patch:
	./scripts/release.sh patch

release-minor:
	./scripts/release.sh minor

release-major:
	./scripts/release.sh major
