MODULE     := github.com/lucaspose/goci-cli
BINARY     := gopherci
CMD_DIR    := ./cmd
OUT_DIR    := ./bin
IMAGE_NAME := gopherci

# Install path depends on the OS
UNAME := $(shell uname -s)
ifeq ($(UNAME), Linux)
  INSTALL_DIR := $(HOME)/.local/bin
else ifeq ($(UNAME), Darwin)
  INSTALL_DIR := /usr/local/bin
else
  INSTALL_DIR := $(HOME)/bin
endif

# Build info
VERSION  := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT   := $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
LDFLAGS  := -ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT)"

.DEFAULT_GOAL := help

# ── Build ────────────────────────────────────────────────────────────────────

.PHONY: build
build: ## Build the binary into ./bin/gopherci
	@mkdir -p $(OUT_DIR)
	go build $(LDFLAGS) -o $(OUT_DIR)/$(BINARY) $(CMD_DIR)
	@echo "→ $(OUT_DIR)/$(BINARY)"

.PHONY: build-dev
build-dev: ## Build without optimizations (debug)
	@mkdir -p $(OUT_DIR)
	go build -gcflags="all=-N -l" -o $(OUT_DIR)/$(BINARY)-dev $(CMD_DIR)

.PHONY: run
run: build ## Build and start the CLI
	$(OUT_DIR)/$(BINARY)

# ── Test ─────────────────────────────────────────────────────────────────────

.PHONY: test
test: ## Run all tests (verbose)
	go test ./... -v

.PHONY: test-short
test-short: ## Run all tests
	go test ./...

.PHONY: test-cover
test-cover: ## Run tests with an HTML coverage report
	@mkdir -p $(OUT_DIR)
	go test ./... -coverprofile=$(OUT_DIR)/coverage.out
	go tool cover -html=$(OUT_DIR)/coverage.out -o $(OUT_DIR)/coverage.html
	@echo "→ Report: $(OUT_DIR)/coverage.html"

.PHONY: test-race
test-race: ## Run tests with the race detector
	go test -race ./...

# ── Quality ─────────────────────────────────────────────────────────────────

.PHONY: vet
vet: ## Run go vet
	go vet ./...

.PHONY: fmt
fmt: ## Format the code
	gofmt -s -w .

.PHONY: fmt-check
fmt-check: ## Check formatting (used in CI)
	@out=$$(gofmt -s -l .); \
	if [ -n "$$out" ]; then \
		echo "Unformatted files:"; echo "$$out"; exit 1; \
	fi

.PHONY: tidy
tidy: ## Tidy go.mod and go.sum
	go mod tidy

.PHONY: lint
lint: ## Run golangci-lint (must be installed)
	@command -v golangci-lint >/dev/null 2>&1 || { \
		echo "golangci-lint is not installed. Install it with:"; \
		echo "  go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
		exit 1; \
	}
	golangci-lint run ./...

# ── Installation ─────────────────────────────────────────────────────────────

.PHONY: install
install: build ## Install the binary (~/.local/bin on Linux)
	@mkdir -p $(INSTALL_DIR)
	cp $(OUT_DIR)/$(BINARY) $(INSTALL_DIR)/$(BINARY)
	@echo "→ Installed to $(INSTALL_DIR)/$(BINARY)"

.PHONY: uninstall
uninstall: ## Remove the installed binary
	rm -f $(INSTALL_DIR)/$(BINARY)
	@echo "→ Removed $(INSTALL_DIR)/$(BINARY)"

# ── Clean ────────────────────────────────────────────────────────────────────

.PHONY: clean
clean: ## Remove build output
	rm -rf $(OUT_DIR)

# ── Dependencies─────────────────────────────────────────────────────────────

.PHONY: deps
deps: ## Download dependencies
	go mod download

.PHONY: deps-update
deps-update: ## Upgrade all dependencies
	go get -u ./...
	go mod tidy

# ── Info ─────────────────────────────────────────────────────────────────────

.PHONY: version
version: ## Print build version
	@echo "Version: $(VERSION)"
	@echo "Commit:  $(COMMIT)"
	@echo "Module:  $(MODULE)"

.PHONY: help
help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

# ── Docker ───────────────────────────────────────────────────────────────────

.PHONY: docker-build
docker-build: ## Build the gopherci:latest Docker image
	docker build \
		--target final \
		--build-arg VERSION=$(VERSION) \
		--build-arg COMMIT=$(COMMIT) \
		-t $(IMAGE_NAME):$(VERSION) \
		-t $(IMAGE_NAME):latest \
		.
	@echo "→ Image: $(IMAGE_NAME):latest"

.PHONY: docker-extract
docker-extract: ## Build inside Docker and extract ./bin/gopherci
	@mkdir -p $(OUT_DIR)
	docker build \
		--target export \
		--output type=local,dest=$(OUT_DIR) \
		--build-arg VERSION=$(VERSION) \
		--build-arg COMMIT=$(COMMIT) \
		.
	@echo "→ $(OUT_DIR)/$(BINARY)"

.PHONY: docker-run
docker-run: docker-build ## Run gopherci in Docker (uses the host network to reach the API)
	docker run --rm -it --network host $(IMAGE_NAME):latest

.PHONY: docker-clean
docker-clean: ## Remove the gopherci Docker images
	docker rmi -f $(IMAGE_NAME):latest $(IMAGE_NAME):$(VERSION) 2>/dev/null || true

# ── CI ───────────────────────────────────────────────────────────────────────

.PHONY: ci
ci: fmt-check vet test-race build ## fmt-check + vet + tests + build (what the CI runs)
