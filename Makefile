# SPDX-License-Identifier: Apache-2.0
# Copyright (C) 2026 The PharosVPN Authors

# PharosVPN Linux client — build targets. The GUI + AppImage need a Linux box
# with GTK/WebKit2GTK, Wails, and Node; the helper cross-compiles anywhere.
# See BUILD.md for the full prerequisites and the Linux-only steps.

GOARCH ?= amd64

.PHONY: help helper gui dev frontend test vet fmt appimage clean install-helper

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

helper: ## Build the privileged helper (pharos-helper) — cross-compiles anywhere
	CGO_ENABLED=0 GOOS=linux GOARCH=$(GOARCH) go build -trimpath -ldflags "-s -w" \
		-o bin/pharos-helper ./cmd/pharos-helper

frontend: ## Build the Svelte frontend (needs Node)
	cd frontend && npm install && npm run build

gui: ## Build the Wails GUI app (Linux only; needs wails + GTK/WebKit2GTK)
	wails build -platform linux/$(GOARCH)

dev: ## Run the app in Wails dev mode with hot reload (Linux only)
	wails dev

test: ## Run the Go unit tests (the offline display/geometry logic)
	go test ./internal/...

vet: ## go vet for the Linux build
	GOOS=linux GOARCH=$(GOARCH) go vet ./...

fmt: ## gofmt the Go sources
	gofmt -w .

appimage: ## Build the distributable AppImage (Linux only)
	./build/appimage.sh

install-helper: ## (root) install the systemd helper from a local build
	sudo ./bin/pharos-helper install

clean: ## Remove build artifacts
	rm -rf bin dist build/bin frontend/dist/assets
