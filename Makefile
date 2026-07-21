GO := go
PKG := ./...
BACKEND_DIR := backend
FRONTEND_DIR := frontend

.PHONY: build-backend test-backend vet-backend run-backend clean
.PHONY: build-frontend dev-frontend lint-frontend typecheck-frontend test-frontend
.PHONY: test-all

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

test-all: build-frontend test-frontend build-backend vet-backend test-backend
	@echo "All tests passed!"
