SHELL := /bin/bash

APP_NAME ?= adronaut
CMD_DIR  ?= ./cmd/$(APP_NAME)

BUILD_DIR ?= build
DEST_DIR  ?= $(HOME)/.local/bin

# auto .exe on Windows
ifeq ($(OS),Windows_NT)
  BIN_EXT := .exe
else
  BIN_EXT :=
endif

BIN_PATH := $(BUILD_DIR)/$(APP_NAME)$(BIN_EXT)

GO      ?= go
GOFLAGS ?=
LDFLAGS ?= -s -w

.PHONY: all build run install clean tidy help

all: build

build:
	@mkdir -p "$(BUILD_DIR)"
	@echo "→ Building $(APP_NAME)…"
	@$(GO) build -trimpath -ldflags "$(LDFLAGS)" $(GOFLAGS) -o "$(BIN_PATH)" "$(CMD_DIR)"
	@echo "✔ Built: $(BIN_PATH)"

run: build
	@"$(BIN_PATH)"

install: build
	@mkdir -p "$(DEST_DIR)"
	@cp "$(BIN_PATH)" "$(DEST_DIR)/$(APP_NAME)$(BIN_EXT)"
	@chmod +x "$(DEST_DIR)/$(APP_NAME)$(BIN_EXT)" || true
	@echo "✔ Installed: $(DEST_DIR)/$(APP_NAME)$(BIN_EXT)"
	@if ! echo "$$PATH" | tr ':' '\n' | grep -qx "$(DEST_DIR)"; then \
		echo ""; \
		echo "ℹ️  $(DEST_DIR) is not in your PATH."; \
		echo "   Add it, e.g. for bash/zsh:"; \
		echo "     echo 'export PATH=\$$HOME/.local/bin:\$$PATH' >> ~/.bashrc  # or ~/.zshrc"; \
		echo ""; \
	fi

clean:
	@echo "→ Cleaning…"
	@rm -f "$(DEST_DIR)/$(APP_NAME)$(BIN_EXT)" || true
	@rm -rf "$(BUILD_DIR)"
	@echo "✔ Done."

tidy:
	@$(GO) mod tidy

help:
	@echo "Targets:"
	@echo "  make build    - Build binary into $(BUILD_DIR)/"
	@echo "  make run      - Build and run the binary"
	@echo "  make install  - Install to $(DEST_DIR)"
	@echo "  make tidy     - go mod tidy"
	@echo "  make clean    - Remove build artifacts"

