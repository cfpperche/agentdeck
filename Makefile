# AgentDeck dev targets
.PHONY: help dev web build test clean

help:
	@echo "Targets:"
	@echo "  dev     run the Phase-0 server (https://localhost:8444)"
	@echo "  web     frontend dev server with hot reload"
	@echo "  build   build the web UI into web/dist"
	@echo "  test    run tests (Go port: phase 1)"
	@echo "  clean   remove build artifacts"

dev:
	python3 -m legacy.backend.__main__

web:
	cd web && npm run dev

build:
	cd web && npm run build

test:
	@echo "no tests yet (Phase 1: Go port with fake-agent test doubles)"

clean:
	rm -rf web/dist
