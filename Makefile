.PHONY: build run test clean fmt lint deps help

BINARY_NAME=whisper-transcribe
BUILD_DIR=./build
CMD_DIR=./cmd/whisper-transcribe

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'

build: ## Build the binary
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_DIR)

run: build ## Build and run the TUI
	$(BUILD_DIR)/$(BINARY_NAME)

run-cli: build ## Run in CLI mode with a URL
	$(BUILD_DIR)/$(BINARY_NAME) --no-tui --url "$(URL)" --model "$(MODEL)"

test: ## Run tests
	go test -v ./...

test-short: ## Run short tests only
	go test -v -short ./...

clean: ## Clean build artifacts
	rm -rf $(BUILD_DIR)
	go clean

fmt: ## Format code
	go fmt ./...
	gofmt -s -w .

lint: ## Run linter
	golangci-lint run ./...

deps: ## Download dependencies
	go mod download
	go mod tidy

dev: ## Enter nix development shell
	nix develop

.DEFAULT_GOAL := help
