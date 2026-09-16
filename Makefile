# ==============================================================================
# Fling CLI - Makefile
# ==============================================================================

BINARY_NAME=fling
SRC_DIR=./cmd/fling
INSTALL_DIR=/usr/local/bin

.PHONY: all build install uninstall test test-race run clean help

all: build

## build: Compile the fling binary with optimized flags
build:
	@echo "==> Building $(BINARY_NAME)..."
	go build -ldflags="-s -w" -o $(BINARY_NAME) $(SRC_DIR)
	@echo "==> Build complete: ./$(BINARY_NAME)"

## install: Install fling to /usr/local/bin (may require sudo)
install: build
	@echo "==> Installing $(BINARY_NAME) to $(INSTALL_DIR)..."
	@if [ -w "$(INSTALL_DIR)" ]; then \
		install -m 755 $(BINARY_NAME) $(INSTALL_DIR)/$(BINARY_NAME); \
	else \
		sudo install -m 755 $(BINARY_NAME) $(INSTALL_DIR)/$(BINARY_NAME); \
	fi
	@echo "==> Successfully installed $(BINARY_NAME) to $(INSTALL_DIR)/$(BINARY_NAME)"

## uninstall: Remove fling from /usr/local/bin
uninstall:
	@echo "==> Uninstalling $(BINARY_NAME)..."
	@if [ -w "$(INSTALL_DIR)" ]; then \
		rm -f $(INSTALL_DIR)/$(BINARY_NAME); \
	else \
		sudo rm -f $(INSTALL_DIR)/$(BINARY_NAME); \
	fi
	@echo "==> Successfully uninstalled $(BINARY_NAME)"

## test: Run unit and integration tests
test:
	@echo "==> Running tests..."
	go test -v ./...

## test-race: Run tests with race detector enabled
test-race:
	@echo "==> Running tests with race detector..."
	go test -v -race ./...

## run: Build and launch Fling TUI
run: build
	./$(BINARY_NAME)

## clean: Remove compiled binaries and artifacts
clean:
	@echo "==> Cleaning build artifacts..."
	rm -f $(BINARY_NAME)
	rm -rf testdata/transfers/*

## help: Display this help message
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'
