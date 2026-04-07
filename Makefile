VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  := $(shell git rev-parse --short=7 HEAD 2>/dev/null || echo unknown)
DIRTY   := $(shell git diff --quiet 2>/dev/null && git diff --cached --quiet 2>/dev/null || echo +$(shell date -u +%Y%m%d%H%M%S))
LDFLAGS := -ldflags "-s -w -X github.com/clarive/cla-worker-go/internal/version.Version=$(VERSION) -X github.com/clarive/cla-worker-go/internal/version.Commit=$(COMMIT)$(DIRTY)"

.PHONY: build test test-integration test-all bench cover lint cross clean

build:
	go build $(LDFLAGS) -o bin/cla-worker .

test:
	go test -v -race -count=1 ./internal/... ./cmd/...

test-integration:
	go test -v -race -tags=integration -count=1 ./tests/...

test-all: test test-integration

bench:
	go test -bench=. -benchmem -run=^$$ ./internal/...

cover:
	go test -coverprofile=coverage.out -covermode=atomic ./internal/... ./cmd/...
	go tool cover -html=coverage.out -o coverage.html

lint:
	golangci-lint run

cross:
	GOOS=linux   GOARCH=amd64 go build $(LDFLAGS) -o dist/cla-worker-linux-x64 .
	GOOS=linux   GOARCH=arm64 go build $(LDFLAGS) -o dist/cla-worker-linux-arm64 .
	GOOS=darwin  GOARCH=amd64 go build $(LDFLAGS) -o dist/cla-worker-darwin-x64 .
	GOOS=darwin  GOARCH=arm64 go build $(LDFLAGS) -o dist/cla-worker-darwin-arm64 .
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o dist/cla-worker-windows-x64.exe .
	GOOS=windows GOARCH=arm64 go build $(LDFLAGS) -o dist/cla-worker-windows-arm64.exe .

clean:
	rm -rf bin/ dist/ coverage.out coverage.html
