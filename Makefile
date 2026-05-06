.PHONY: test vet build

test:
	go test -race ./...

vet:
	go vet ./...

build:
	mkdir -p bin
	CGO_ENABLED=0 go build -ldflags="-w -s" -o bin/mem0-mcp-go ./cmd/mem0-mcp-go
