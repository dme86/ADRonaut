SHELL := /bin/bash

APP_NAME ?= adronaut
SRC      ?= main.go

BUILD_DIR ?= build
DEST_DIR  ?= $(HOME)/.local/bin
BIN_PATH  ?= $(BUILD_DIR)/$(APP_NAME)

GO        ?= go
GOFLAGS   ?=
LDFLAGS   ?= -s -w

.PHONY: all build install clean help

all: build

build:
	@mkdir -p "$(BUILD_DIR)"
	@echo "→ Building $(APP_NAME)…"
	@$(GO) build -trimpath -ldflags "$(LDFLAGS)" $(GOFLAGS) -o "$(BIN_PATH)" "$(SRC)"
	@echo "✔ Built: $(BIN_PATH)"

install: build
	@mkdir -p "$(DEST_DIR)"
	@cp "$(BIN_PATH)" "$(DEST_DIR)/$(APP_NAME)"
	@chmod +x "$(DEST_DIR)/$(APP_NAME)"
	@echo "✔ Installed: $(DEST_DIR)/$(APP_NAME)"
	@# Hinweis, falls ~/.local/bin nicht im PATH ist
	@if ! echo "$$PATH" | tr ':' '\n' | grep -qx "$(DEST_DIR)"; then \
		echo ""; \
		echo "ℹ️  $(DEST_DIR) ist derzeit nicht in deinem PATH."; \
		echo "   Füge es hinzu, z. B. für bash/zsh:"; \
		echo "     echo 'export PATH=\$$HOME/.local/bin:\$$PATH' >> ~/.bashrc  # oder ~/.zshrc"; \
		echo "   Für fish:"; \
		echo "     set -Ux PATH \$$HOME/.local/bin \$$PATH"; \
		echo ""; \
	fi

clean:
	@echo "→ Entferne Installation und Build-Artefakte…"
	@rm -f "$(DEST_DIR)/$(APP_NAME)" || true
	@rm -rf "$(BUILD_DIR)"
	@echo "✔ Deinstalliert und bereinigt."

help:
	@echo "Targets:"
	@echo "  make build    - Binary lokal in $(BUILD_DIR)/ bauen"
	@echo "  make install  - Binary nach $(DEST_DIR) installieren"
	@echo "  make clean    - Binary deinstallieren und Build-Ordner löschen"
