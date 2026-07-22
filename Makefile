GO := go
PKG := ./...
BACKEND_DIR := backend
FRONTEND_DIR := frontend

.PHONY: build-backend test-backend vet-backend run-backend clean
.PHONY: build-frontend dev-frontend lint-frontend typecheck-frontend test-frontend
.PHONY: build-go-binary build-electron test-all
.PHONY: cross-build-all

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
	rm -rf $(FRONTEND_DIR)/bin/* $(FRONTEND_DIR)/dist-electron/*

build-go-binary:
	cd $(BACKEND_DIR) && GOOS=windows GOARCH=amd64 $(GO) build -ldflags="-s -w" -o ../$(FRONTEND_DIR)/bin/flowpartner-backend.exe ./cmd/server/

build-electron: build-frontend build-go-binary
	cd $(FRONTEND_DIR) && npm run build:electron

cross-build-all: build-frontend
	@echo "=== Building Go binaries for all platforms ==="
	@mkdir -p $(FRONTEND_DIR)/bin
	# Windows x64
	cd $(BACKEND_DIR) && GOOS=windows GOARCH=amd64 $(GO) build -ldflags="-s -w" -o ../$(FRONTEND_DIR)/bin/flowpartner-backend.exe ./cmd/server/
	# Windows arm64
	cd $(BACKEND_DIR) && GOOS=windows GOARCH=arm64 $(GO) build -ldflags="-s -w" -o ../$(FRONTEND_DIR)/bin/flowpartner-backend-arm64.exe ./cmd/server/
	# macOS x64
	cd $(BACKEND_DIR) && GOOS=darwin GOARCH=amd64 $(GO) build -ldflags="-s -w" -o ../$(FRONTEND_DIR)/bin/flowpartner-backend-darwin-x64 ./cmd/server/
	# macOS arm64
	cd $(BACKEND_DIR) && GOOS=darwin GOARCH=arm64 $(GO) build -ldflags="-s -w" -o ../$(FRONTEND_DIR)/bin/flowpartner-backend-darwin-arm64 ./cmd/server/
	# Linux x64
	cd $(BACKEND_DIR) && GOOS=linux GOARCH=amd64 $(GO) build -ldflags="-s -w" -o ../$(FRONTEND_DIR)/bin/flowpartner-backend-linux-x64 ./cmd/server/
	# Linux arm64
	cd $(BACKEND_DIR) && GOOS=linux GOARCH=arm64 $(GO) build -ldflags="-s -w" -o ../$(FRONTEND_DIR)/bin/flowpartner-backend-linux-arm64 ./cmd/server/
	@echo "=== All binaries built ==="
	@ls -la $(FRONTEND_DIR)/bin/

test-all: build-frontend test-frontend build-go-binary build-backend vet-backend test-backend
	@echo "All tests passed!"
