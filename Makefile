APP_NAME := seer
START_CMD := start
BIN_DIR := ./bin
SRC_DIR := ./cmd
VERSION := $(shell git describe --tags --always --dirty)
BUILD_TIME := $(shell date -u '+%Y-%m-%dT%H:%M:%S')
GO := go


#Linting and formatting
LINTER := golangci-lint
FMT := gofmt

.PHONY: all build run lint

all: build

build:
	@echo "Building $(APP_NAME)..."
	@mkdir -p $(BIN_DIR)
	$(GO) build -o $(BIN_DIR)/$(APP_NAME) \
		-ldflags "-X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME)" $(SRC_DIR)

run: build
	@echo "Running $(APP_NAME)..."
	$(BIN_DIR)/$(APP_NAME) start

test:
	@echo "Running tests..."
	$(GO) test -v ./...

clean:
	@echo "Cleaning build artifacts..."
	@rm -f $(BIN_DIR)

lint:
	@echo "Linting code..."
	$(LINTER) run ./...

format:
	@echo "Formating code..."
	$(FMT) -s -w .

install:
	@echo "Installing dependencies..."
	$(GO) mod tidy