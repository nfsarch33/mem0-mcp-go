.PHONY: test vet build cover cli-build cli-test

test:
	go test -race ./...

vet:
	go vet ./...

build:
	mkdir -p bin
	CGO_ENABLED=0 go build -ldflags="-w -s" -o bin/mem0-mcp-go ./cmd/mem0-mcp-go

# CLI surface targets
cli-build: build

cli-test:
	go test -race ./internal/cliconfig/... ./internal/clicmd/...

cover:
	go test -race -coverprofile=coverage.out -covermode=atomic ./...
	go tool cover -func=coverage.out | tail -1
