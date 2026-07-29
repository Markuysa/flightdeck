# FlightDeck build entry points.
#
# Two things here are not cosmetic:
#
#  1. GO_PKGS is `./cmd/... ./internal/...`, never `./...`. The UI's
#     node_modules contains a stray Go package (flatted's golang helper), and
#     `go test ./...` picks it up and builds it. Naming our own trees keeps
#     someone else's vendored Go out of our test run.
#
#  2. `ui` wipes internal/webui/dist/assets before copying. Vite content-hashes
#     every filename, so a plain copy accumulates one index-<hash>.js and
#     index-<hash>.css per build — all of them embedded into the binary
#     forever. .gitignore keeps internal/webui/dist/index.html (the committed
#     placeholder that keeps go:embed resolving on a clean checkout), so the
#     clean must be surgical: assets only, then re-copy.

GO_PKGS   := ./cmd/... ./internal/...
BIN       := bin/flightdeck
EMBED_DIR := internal/webui/dist

.PHONY: all ui build test test-race lint vet fmt run demo clean help

all: ui build

## ui: build the React app and refresh the embedded copy
ui:
	cd ui && npm ci && npm run build
	rm -rf $(EMBED_DIR)/assets
	cp -r ui/dist/. $(EMBED_DIR)/

## build: compile the single binary (run `make ui` first for a release build)
build:
	go build -o $(BIN) ./cmd/flightdeck

## test: Go + UI test suites
test:
	go test $(GO_PKGS)
	cd ui && npm test

## test-race: Go tests under the race detector
test-race:
	go test -race $(GO_PKGS)

## lint: vet Go, then the UI's token check + eslint
lint: vet
	cd ui && npm run lint

vet:
	go vet $(GO_PKGS)

fmt:
	gofmt -w cmd internal

## run: serve against the current working directory's config
run:
	go run ./cmd/flightdeck serve

## demo: serve the seeded fixture project, no real repo needed
demo:
	go run ./cmd/flightdeck serve --demo

clean:
	rm -rf $(BIN) ui/dist $(EMBED_DIR)/assets

help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/^## /  /'
