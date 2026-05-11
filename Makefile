.PHONY: help fetch-binaries clean-binaries test tidy

SHELL := /bin/bash

HADOLINT_VERSION ?= v2.12.0
HADOLINT_RELEASE_URL := https://github.com/hadolint/hadolint/releases/download/$(HADOLINT_VERSION)
HADOLINT_NOTICE_URL := https://raw.githubusercontent.com/hadolint/hadolint/$(HADOLINT_VERSION)/ThirdPartyNotices.txt
HADOLINT_LICENSE_URL := https://raw.githubusercontent.com/hadolint/hadolint/$(HADOLINT_VERSION)/LICENSE
BIN_DIR := bin

help: ## Display this help page
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[33m%-20s\033[0m %s\n", $$1, $$2}'

fetch-binaries: ## Download hadolint binaries for all supported platforms into bin/
	@mkdir -p $(BIN_DIR)
	@echo ">> downloading hadolint $(HADOLINT_VERSION)"
	@curl -fsSL -o $(BIN_DIR)/linux-amd64   $(HADOLINT_RELEASE_URL)/hadolint-Linux-x86_64
	@curl -fsSL -o $(BIN_DIR)/linux-arm64   $(HADOLINT_RELEASE_URL)/hadolint-Linux-arm64
	@curl -fsSL -o $(BIN_DIR)/darwin-amd64  $(HADOLINT_RELEASE_URL)/hadolint-Darwin-x86_64
	@curl -fsSL -o $(BIN_DIR)/windows.exe   $(HADOLINT_RELEASE_URL)/hadolint-Windows-x86_64.exe
	@# Upstream does not publish a native darwin-arm64 build; reuse the x86_64 binary (runs via Rosetta).
	@cp $(BIN_DIR)/darwin-amd64 $(BIN_DIR)/darwin-arm64
	@chmod +x $(BIN_DIR)/linux-amd64 $(BIN_DIR)/linux-arm64 $(BIN_DIR)/darwin-amd64 $(BIN_DIR)/darwin-arm64
	@echo ">> downloading hadolint ThirdPartyNotices.txt"
	@curl -fsSL -o $(BIN_DIR)/ThirdPartyNotices.txt $(HADOLINT_NOTICE_URL)
	@echo ">> downloading hadolint LICENSE"
	@curl -fsSL -o $(BIN_DIR)/hadolint-LICENSE.txt $(HADOLINT_LICENSE_URL)
	@printf '%s\n' "hadolint $(HADOLINT_VERSION) (https://github.com/hadolint/hadolint)" \
		"linux-amd64    -> hadolint-Linux-x86_64" \
		"linux-arm64    -> hadolint-Linux-arm64" \
		"darwin-amd64   -> hadolint-Darwin-x86_64" \
		"darwin-arm64   -> hadolint-Darwin-x86_64 (no native arm64 upstream build)" \
		"windows.exe    -> hadolint-Windows-x86_64.exe" > $(BIN_DIR)/README.txt
	@echo ">> done. files in $(BIN_DIR)/"

clean-binaries: ## Remove downloaded hadolint binaries
	@rm -f $(BIN_DIR)/linux-amd64 $(BIN_DIR)/linux-arm64 $(BIN_DIR)/darwin-amd64 $(BIN_DIR)/darwin-arm64 $(BIN_DIR)/windows.exe $(BIN_DIR)/README.txt $(BIN_DIR)/ThirdPartyNotices.txt $(BIN_DIR)/hadolint-LICENSE.txt

test: ## Run tests
	@go test -v ./...

tidy: ## Tidy go modules
	@go mod tidy
