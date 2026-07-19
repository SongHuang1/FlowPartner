GO := go
PKG := ./...
BACKEND_DIR := backend

.PHONY: build-backend test-backend vet-backend run-backend clean

build-backend:
	cd $(BACKEND_DIR) && $(GO) build $(PKG)

test-backend:
	cd $(BACKEND_DIR) && $(GO) test $(PKG) -race

vet-backend:
	cd $(BACKEND_DIR) && $(GO) vet $(PKG)

run-backend:
	cd $(BACKEND_DIR) && $(GO) run cmd/server/main.go

clean:
	$(GO) clean -cache -testcache
