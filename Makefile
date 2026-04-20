.PHONY: build run test fmt lint clean

APP=prepump
BUILD_DIR=.

build:
	go build -o $(BUILD_DIR)/$(APP) ./cmd/prepump

run:
	go run ./cmd/prepump

run-dev:
	go run ./cmd/prepump -top=20 -workers=4 -no-live-stats

test:
	go test ./...

test-verbose:
	go test -v ./...

fmt:
	go fmt ./...

lint:
	golangci-lint run || go vet ./...

deps:
	go mod tidy
	go mod download

clean:
	rm -f $(BUILD_DIR)/$(APP)
	go clean

install:
	go install ./cmd/prepump

# Build for different platforms
build-all: build-darwin build-linux build-windows

build-darwin:
	GOOS=darwin GOARCH=amd64 go build -o $(BUILD_DIR)/$(APP)-darwin-amd64 ./cmd/prepump
	GOOS=darwin GOARCH=arm64 go build -o $(BUILD_DIR)/$(APP)-darwin-arm64 ./cmd/prepump

build-linux:
	GOOS=linux GOARCH=amd64 go build -o $(BUILD_DIR)/$(APP)-linux-amd64 ./cmd/prepump

build-windows:
	GOOS=windows GOARCH=amd64 go build -o $(BUILD_DIR)/$(APP)-windows-amd64.exe ./cmd/prepump

# Development helpers
watch:
	reflex -r '\.go$$' -s -- make run

debug:
	dlv debug ./cmd/prepump
