# Justfile for OpenClaw Dashboard

# Build the application
build:
    go build -o openclaw cmd/server/main.go

# Run the application locally
run:
    go run cmd/server/main.go

# Run all tests
test:
    go test ./...

# Run tests with coverage
test-cover:
    go test -cover ./...

# Format code (Go and Nix)
fmt:
    gofmt -s -w .
    nix fmt

# Lint code
lint:
    go vet ./...

# Run e2e tests
test-e2e:
    go test -v ./tests/e2e/...

# Clean build artifacts
clean:
    go clean
    rm -f openclaw

# Tidy go modules
tidy:
    go mod tidy

# Watch and sync files to remote
sync:
    ./scripts/sync_watch.sh

# Run air for live reloading (Run this on the remote machine)
air:
    air

# Reload Chromium browser
reload-browser:
    @echo "Reloading Chromium..."
    @xdotool search --onlyvisible --class chromium windowactivate --sync key F5 || true

