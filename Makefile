.PHONY: build test clean run

APP_NAME := dragshift
BUILD_DIR := build

build:
	go build -o $(BUILD_DIR)/$(APP_NAME) ./cmd/dragshift

test:
	go test -v ./...

clean:
	rm -rf $(BUILD_DIR)

run: build
	./$(BUILD_DIR)/$(APP_NAME)

lint:
	golangci-lint run ./...
