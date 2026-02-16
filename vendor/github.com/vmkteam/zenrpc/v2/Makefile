PKG := `go list -f {{.Dir}} ./...`

fmt:
	@golangci-lint fmt

lint:
	@golangci-lint version
	@golangci-lint config verify
	@golangci-lint run

test:
	@go test -v ./...

mod:
	@go mod tidy

build:
	@go build -o zenrpc/zenrpc zenrpc/*.go