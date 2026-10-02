.PHONY: build run test clean install

BINARY    := paradox
ALIAS     := prx
CMD       := ./cmd/paradox
BIN_DIR   := bin
VERSION   ?= 0.1.0-dev
COMMIT    ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
BUILD_DATE?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS   := -X github.com/paradox-cloud/paradox/internal/version.Version=$(VERSION) \
             -X github.com/paradox-cloud/paradox/internal/version.Commit=$(COMMIT) \
             -X github.com/paradox-cloud/paradox/internal/version.BuildDate=$(BUILD_DATE)

build:
	@mkdir -p $(BIN_DIR)
	GOTOOLCHAIN=local go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(BINARY) $(CMD)
	@# Short alias: prx → same binary
	cp $(BIN_DIR)/$(BINARY) $(BIN_DIR)/$(ALIAS)
	@echo "Built $(BIN_DIR)/$(BINARY) and $(BIN_DIR)/$(ALIAS)"

run:
	GOTOOLCHAIN=local go run $(CMD)

test:
	GOTOOLCHAIN=local go test ./...

clean:
	rm -rf $(BIN_DIR)

install: build
	@dest=""; \
	if [ -n "$$GOPATH" ] && [ -d "$$GOPATH/bin" ]; then dest="$$GOPATH/bin"; \
	elif [ -d "$$HOME/go/bin" ]; then dest="$$HOME/go/bin"; \
	elif [ -w /usr/local/bin ]; then dest="/usr/local/bin"; \
	else dest="$$HOME/bin"; mkdir -p "$$dest"; fi; \
	cp $(BIN_DIR)/$(BINARY) "$$dest/$(BINARY)"; \
	cp $(BIN_DIR)/$(ALIAS) "$$dest/$(ALIAS)"; \
	echo "Installed $(BINARY) and $(ALIAS) → $$dest"; \
	echo "If 'prx' is not found, add $$dest to PATH"
