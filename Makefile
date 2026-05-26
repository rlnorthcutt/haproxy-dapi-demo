.PHONY: build run up down reset list test smoke lint fmt certs help

BINARY := dapi-demo
MODULE  := github.com/rlnorthcutt/haproxy-dapi-demo
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -ldflags "-X $(MODULE)/cmd/dapi-demo.version=$(VERSION)"

build:
	go build $(LDFLAGS) -o $(BINARY) ./cmd/dapi-demo

run: build
	./$(BINARY)

up:
	./$(BINARY) up

down:
	./$(BINARY) down

reset:
	./$(BINARY) reset

list:
	./$(BINARY) list

fmt:
	gofmt -w ./...

lint:
	go vet ./...

test:
	go test ./internal/...

# Run all scenarios in --auto mode against a live stack (canary test).
smoke: build
	@echo "==> Running smoke tests against live stack"
	@for f in scenarios/*.yaml; do \
		id=$$(basename $$f .yaml); \
		echo "  -> $$id"; \
		./$(BINARY) run $$id --auto || exit 1; \
	done
	@echo "==> All scenarios passed"

certs:
	@echo "Generating self-signed demo certs in haproxy/certs/"
	@cd haproxy/certs && \
		openssl req -x509 -newkey rsa:2048 -keyout key.pem -out cert.pem -days 3650 -nodes \
		  -subj "/CN=dapi-demo/O=HAProxy Demo/C=US" 2>/dev/null && \
		cat cert.pem key.pem > api-2025.pem && \
		openssl req -x509 -newkey rsa:2048 -keyout key2.pem -out cert2.pem -days 3650 -nodes \
		  -subj "/CN=dapi-demo-2026/O=HAProxy Demo/C=US" 2>/dev/null && \
		cat cert2.pem key2.pem > api-2026.pem && \
		rm -f key.pem cert.pem key2.pem cert2.pem
	@echo "Done"

help:
	@echo "Targets:"
	@echo "  build    compile dapi-demo binary"
	@echo "  run      build and launch TUI"
	@echo "  up       bring up compose stack"
	@echo "  down     tear down compose stack"
	@echo "  reset    restore baseline config"
	@echo "  list     list available scenarios"
	@echo "  test     run unit tests"
	@echo "  smoke    run all scenarios in --auto mode"
	@echo "  fmt      gofmt all Go files"
	@echo "  certs    regenerate self-signed demo certs"
