APP_NAME := seer
START_CMD := start
BIN_DIR := ./bin
SRC_DIR := ./cmd
VERSION := $(shell git describe --tags --always --dirty)
BUILD_TIME := $(shell date -u '+%Y-%m-%dT%H:%M:%S')
GO := go
MOCK_GEN := mockgen
MOCKS_DIR := ./mocks
INTERFACES_DIR := ./interfaces


#Linting and formatting
LINTER := golangci-lint
FMT := gofmt

.PHONY: all build run lint

all: build

mock-gen:
	@echo "generating mocks..."
	@for file in $(INTERFACES_DIR)/*.go; do \
    		filename=$$(basename $$file .go); \
    		$(MOCK_GEN) -source=$$file -destination=$(MOCKS_DIR)/$$filename\_mock.go -package=mocks; \
    		echo "Generated mock for $$file"; \
    	done

#	$(MOCK_GEN) -source=./interfaces/abiParser.go -destination=./mocks/abiParser_mock.go -package=mocks
#	$(MOCK_GEN) -source=./interfaces/databseHandler.go -destination=./mocks/dbHandler_mock.go -package=mocks
#	$(MOCK_GEN) -source=./interfaces/listener.go -destination=./mocks/listener_mock.go -package=mocks

build: mock-gen
	@echo "Building $(APP_NAME)..."
	@mkdir -p $(BIN_DIR)
	$(GO) build -o $(BIN_DIR)/$(APP_NAME) \
		-ldflags "-X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME)" $(SRC_DIR)

run: build
	@echo "Running $(APP_NAME)..."
	$(BIN_DIR)/$(APP_NAME) $(START_CMD)

test:
	@echo "Running tests..."
	$(GO) test -v ./...

clean:
	@echo "Cleaning build artifacts..."
	$(GO) clean -cache
	@rm -rf $(BIN_DIR)

lint:
	@echo "Linting code..."
	$(LINTER) run ./...

format:
	@echo "Formating code..."
	$(FMT) -s -w .

install:
	@echo "Installing dependencies..."
	$(GO) mod tidy
