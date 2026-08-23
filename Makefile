# AgentDeck dev targets
.PHONY: help dev dev-go web build build-go test lint clean

help:
	@echo "Targets:"
	@echo "  build     build the web UI + the single Go binary (bin/agentdeck)"
	@echo "  dev       run the Phase-0 Python server (https://localhost:8444)"
	@echo "  dev-go    run the Go server (https://localhost:8444)"
	@echo "  web       frontend dev server with hot reload"
	@echo "  test      go test ./... (fake agents; deterministic, no tokens)"
	@echo "  clean     remove build artifacts"

build: build-go
	@:

build-go:
	cd web && npm run build
	go build -ldflags "-X main.Version=$(shell git describe --tags --always 2>/dev/null || echo dev)" -o bin/agentdeck .

dev:
	python3 -m legacy.backend.__main__

dev-go: build-go
	AGENTDECK_DATA ?= $(HOME)/agentdeck/data
	./bin/agentdeck

web:
	cd web && npm run dev

test:
	go test ./...

lint:
	go vet ./...

clean:
	rm -rf bin web/dist/*
	@[ -f web/dist/index.html ] || cp .github/stub-index.html web/dist/index.html 2>/dev/null || true
