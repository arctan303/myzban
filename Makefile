.PHONY: build build-linux clean

# Build for current platform
build:
	go build -o build/pnm ./cmd/pnm/

# Cross-compile for Linux
build-linux:
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o build/pnm-linux-amd64 ./cmd/pnm/
	CGO_ENABLED=1 GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o build/pnm-linux-arm64 ./cmd/pnm/

# Clean build artifacts
clean:
	rm -rf build/

# Run tests
test:
	go test ./...

# Tidy modules
tidy:
	go mod tidy
