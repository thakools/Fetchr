.PHONY: help deps web build run dev test lint certs docker docker-amd64 docker-multi clean

BINARY   := bin/fetchr
IMAGE    ?= fetchr:dev
PLATFORM ?= linux/amd64

help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-10s\033[0m %s\n", $$1, $$2}'

deps: ## Install Go and npm dependencies
	go mod download
	cd web && npm ci

web: ## Build the SPA into internal/web/dist
	cd web && npm run build

build: web ## Build the server binary with the SPA embedded
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o $(BINARY) ./cmd/fetchr

run: build ## Build and run with authentication disabled
	$(BINARY) --skip-login --cookie-secure=false

dev: ## Run the Go server and the Vite dev server together
	@$(MAKE) -j2 dev-api dev-web

dev-api:
	go run ./cmd/fetchr --skip-login --cookie-secure=false

dev-web:
	cd web && npm run dev

test: ## Run Go tests and the frontend type check
	go vet ./...
	go test ./...
	cd web && npm run typecheck

certs: ## Export admin-installed root CAs for networks that intercept TLS (macOS)
	security find-certificate -a -p /Library/Keychains/System.keychain > certs/corporate-ca.crt
	@echo "wrote certs/corporate-ca.crt ($$(grep -c 'BEGIN CERT' certs/corporate-ca.crt) certificates)"

docker: ## Build the container image for the host architecture
	docker buildx build -t $(IMAGE) --load .

docker-amd64: ## Build a linux/amd64 image (cross-compiled, no emulation)
	docker buildx build --platform=$(PLATFORM) -t $(IMAGE) --load .

docker-multi: ## Build and push a multi-arch image; set IMAGE to a remote ref
	docker buildx build --platform=linux/amd64,linux/arm64 -t $(IMAGE) --push .

clean:
	rm -rf bin internal/web/dist web/node_modules
