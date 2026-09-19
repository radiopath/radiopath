ifneq (,$(wildcard .env))
include .env
export $(shell sed -E 's/^[[:space:]]*\#.*//; s/=.*//' .env)
endif

VERSION := $(shell git describe --tags --always --dirty 2>/dev/null)
COMPOSE := docker compose -f docker/compose.yaml

.PHONY: run test vet build docker db-up db-down css css-watch e2e e2e-open

.env:
	cp .env.example .env

run: .env css
	go run -ldflags="-X main.version=$(VERSION)" ./cmd/radiopath

test:
	go test ./...

vet:
	go vet ./...

build: css
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.version=$(VERSION)" -o bin/radiopath ./cmd/radiopath

# Tailwind source is internal/web/ui/app.css; the generated app.css is committed so go build needs no node.
node_modules: package-lock.json
	npm ci
	@touch node_modules

css: node_modules
	npx tailwindcss -i internal/web/ui/app.css -o internal/web/static/app.css --minify

css-watch: node_modules
	npx tailwindcss -i internal/web/ui/app.css -o internal/web/static/app.css --watch

e2e: build
	e2e/e2e.sh

e2e-open: build
	E2E_OPEN=1 e2e/e2e.sh

docker:
	docker build -f docker/Dockerfile -t radiopath:dev .

db-up:
	$(COMPOSE) up -d

db-down:
	$(COMPOSE) down
