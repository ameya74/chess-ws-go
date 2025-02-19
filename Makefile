# Variables
GO_CMD=go
GO_BUILD=$(GO_CMD) build
GO_TEST=$(GO_CMD) test
GO_MOD=$(GO_CMD) mod

# Directories
GO_DIR=./cmd/server

# Targets
all: build

# Build Go project
build:
    @echo "Building Go project..."
    cd $(GO_DIR) && $(GO_BUILD) -o ../../bin/server

# Run Go tests
test:
    @echo "Running Go tests..."
    cd $(GO_DIR) && $(GO_TEST) ./...

# Clean build artifacts
clean:
    @echo "Cleaning build artifacts..."
    rm -rf bin/*

# Initialize Go modules
init:
    @echo "Initializing Go modules..."
    cd $(GO_DIR) && $(GO_MOD) tidy

.PHONY: all build test clean init