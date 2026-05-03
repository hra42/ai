BINARY := ai

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS := -s -w \
	-X main.version=$(VERSION) \
	-X main.commit=$(COMMIT) \
	-X main.date=$(DATE)

.PHONY: build install run test vet lint fmt fmt-check tidy snapshot clean help

build: ## Build the binary into ./ai with version metadata
	go build -ldflags '$(LDFLAGS)' -o $(BINARY) .

install: ## Install into $GOBIN with version metadata
	go install -ldflags '$(LDFLAGS)' .

run: ## Run from source (pass args via ARGS="...")
	go run . $(ARGS)

test: ## Run tests
	go test ./...

vet: ## Run go vet
	go vet ./...

lint: ## Run golangci-lint
	golangci-lint run

fmt: ## Format all Go files in place
	gofmt -w .

fmt-check: ## Fail if any files need formatting
	@out=$$(gofmt -l .); \
	if [ -n "$$out" ]; then \
		echo "Unformatted files:"; echo "$$out"; exit 1; \
	fi

tidy: ## Tidy go.mod / go.sum
	go mod tidy

snapshot: ## Local goreleaser snapshot (no publish)
	goreleaser release --snapshot --clean --skip=publish

clean: ## Remove build artefacts
	rm -rf $(BINARY) dist/

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'
