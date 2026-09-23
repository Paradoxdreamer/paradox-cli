.PHONY: build run test clean install

BINARY    := paradox
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
	@echo "Built $(BIN_DIR)/$(BINARY)"

run:
	GOTOOLCHAIN=local go run $(CMD)

test:
	GOTOOLCHAIN=local go test ./...

clean:
	rm -rf $(BIN_DIR)

install: build
	cp $(BIN_DIR)/$(BINARY) $(GOPATH)/bin/$(BINARY) 2>/dev/null || cp $(BIN_DIR)/$(BINARY) /usr/local/bin/$(BINARY)
	@echo "Installed $(BINARY)"
