.PHONY: fmt check-fmt test

# Format all Go files
fmt:
	go fmt ./...

# Check if files are formatted (for CI)
check-fmt:
	@test -z "$$(gofmt -l .)" || (echo "Files not formatted. Run 'make fmt'" && gofmt -l . && exit 1)

# Run all tests
test:
	go test ./...
