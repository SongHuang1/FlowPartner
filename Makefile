GO := go
PKG := ./...
BACKEND_DIR := backend
FRONTEND_DIR := frontend
BINARY_NAME := flowpartner-backend.exe

.PHONY: build-backend test-backend vet-backend run-backend clean
.PHONY: build-frontend dev-frontend lint-frontend typecheck-frontend test-frontend
.PHONY: build-go-binary build-electron test-all

build-backend:
	cd $(BACKEND_DIR) && $(GO) build $(PKG)

test-backend:
	cd $(BACKEND_DIR) && $(GO) test $(PKG) -race

vet-backend:
	cd $(BACKEND_DIR) && $(GO) vet $(PKG)

run-backend:
	cd $(BACKEND_DIR) && $(GO) run cmd/server/main.go

build-frontend:
	cd $(FRONTEND_DIR) && npm run build

dev-frontend:
	cd $(FRONTEND_DIR) && npm run dev

lint-frontend:
	cd $(FRONTEND_DIR) && npm run lint

typecheck-frontend:
	cd $(FRONTEND_DIR) && npm run typecheck

test-frontend:
	cd $(FRONTEND_DIR) && npm run test

clean:
	$(GO) clean -cache -testcache

build-go-binary:
	cd $(BACKEND_DIR) && GOOS=windows GOARCH=amd64 $(GO) build -o ../$(FRONTEND_DIR)/bin/$(BINARY_NAME) ./cmd/server/

build-electron: build-frontend build-go-binary
	cd $(FRONTEND_DIR) && npm run build:electron

test-all: build-frontend test-frontend build-go-binary build-backend vet-backend test-backend
	@echo "All tests passed!"
