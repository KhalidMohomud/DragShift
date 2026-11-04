.PHONY: build test clean run app

APP_NAME := dragshift
BUILD_DIR := build
APP_BUNDLE := $(BUILD_DIR)/DragShift.app

build:
	go build -o $(BUILD_DIR)/$(APP_NAME) ./cmd/dragshift

test:
	go test -v ./...

clean:
	rm -rf $(BUILD_DIR)

run: build
	./$(BUILD_DIR)/$(APP_NAME)

# Build a stable macOS application identity. This is required for macOS to
# show and remember Automation permission for the Finder-selection wheel.
app:
	mkdir -p $(APP_BUNDLE)/Contents/MacOS
	cp packaging/Info.plist $(APP_BUNDLE)/Contents/Info.plist
	go build -o $(APP_BUNDLE)/Contents/MacOS/DragShift ./cmd/dragshift
	codesign --force --deep --sign - $(APP_BUNDLE)

lint:
	golangci-lint run ./...
