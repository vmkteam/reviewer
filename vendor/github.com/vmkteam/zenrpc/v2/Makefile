PKG := `go list -f {{.Dir}} ./...`

fmt:
	@golangci-lint fmt

lint:
	@golangci-lint version
	@golangci-lint config verify
	@golangci-lint run

test:
	@go test -race -v ./...

bench:
	@go test -bench=. -benchmem -run=^$$ ./...

fuzz:
	@go test -fuzz FuzzServerDo -fuzztime 30s
	@go test -fuzz FuzzIsArray -fuzztime 30s
	@go test -fuzz FuzzConvertToObject -fuzztime 30s
	@go test -fuzz FuzzServeHTTP -fuzztime 30s

mod:
	@go mod tidy

build:
	@go build -o zenrpc/zenrpc zenrpc/*.go