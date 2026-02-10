# OpenClaw Dashboard - Agent Guidelines

This repository contains the source code for the OpenClaw Dashboard, a lightweight, single-binary monitoring tool built with Go and Vanilla web technologies. It follows the "Solid Nord" visual design system.

## 1. Build, Lint, and Test Commands

This project uses **Nix Flakes** for reproducible builds and **Go** for the backend.

### Environment Setup
- **Enter Dev Shell**: `nix develop` (Ensures Go, gopls, and tools are available)
- **Direnv**: If available, allow the .envrc: `direnv allow`

### Build Commands
- **Build Binary**: `go build -o openclaw cmd/server/main.go`
- **Build with Nix**: `nix build`
- **Run Locally**: `go run cmd/server/main.go`
- **Clean**: `go clean`

### Testing
- **Run All Tests**: `go test ./...`
- **Run Single Test**: `go test -v -run TestName ./path/to/package`
- **Test with Coverage**: `go test -cover ./...`
- **Watch Mode**: Use `entr` if available: `find . -name "*.go" | entr -r go test ./...`

### Linting & Formatting
- **Format Go**: `gofmt -s -w .`
- **Lint Go**: `go vet ./...` (or `staticcheck ./...` if available in flake)
- **Format Nix**: `nix fmt`
- **Verify Dependencies**: `go mod verify`
- **Tidy Modules**: `go mod tidy`

## 2. Code Style & Conventions

### 2.1 General Principles
- **Zero Dependencies (Frontend)**: Do NOT use React, Vue, Tailwind, or Bootstrap. Use raw CSS and Vanilla JS.
- **Minimal Dependencies (Backend)**: Prefer Go standard library (`net/http`, `html/template`, `embed`) over frameworks like Gin or Echo unless routing complexity demands it.
- **Nord Theme**: Strictly adhere to the Nord color palette.

### 2.2 Go (Backend)
- **Project Structure**:
    - `cmd/server/`: Entry point.
    - `internal/`: Private application logic.
    - `ui/`: Embedded static assets (HTML/CSS/JS).
- **Formatting**: Always run `gofmt`.
- **Error Handling**:
    - Handle errors explicitly. Do not ignore errors.
    - Use `if err != nil { return fmt.Errorf("context: %w", err) }` wrapping.
- **Naming**:
    - PascalCase for exported symbols.
    - camelCase for unexported.
    - Short variable names for short scopes (`i`, `ctx`), descriptive for larger scopes.
- **Concurrency**:
    - Use `context.Context` for cancellation and timeouts.
    - Prefer channels/goroutines for the polling mechanism.
- **Embedding**:
    - Use `//go:embed` directives.
    - Ensure the `ui` directory is embedded correctly into the binary.

### 2.3 Frontend (HTML/CSS/JS)
- **HTML**: Semantic HTML5.
- **CSS**:
    - Use CSS Variables for Nord colors (defined in `:root`).
    - Use CSS Grid for layout (12-column system).
    - No external fonts unless locally embedded.
- **JavaScript**:
    - Modern ES6+.
    - Use `fetch()` for API calls.
    - Avoid global state; encapsulate logic in functions or classes.

### 2.4 Nord Color Palette Reference
Use these CSS variables in `style.css`:
```css
:root {
  /* Polar Night */
  --nord0: #2E3440;
  --nord1: #3B4252;
  --nord2: #434C5E;
  --nord3: #4C566A;
  /* Snow Storm */
  --nord4: #D8DEE9;
  --nord5: #E5E9F0;
  --nord6: #ECEFF4;
  /* Frost */
  --nord7: #8FBCBB;
  --nord8: #88C0D0; /* Primary Accent */
  --nord9: #81A1C1;
  --nord10: #5E81AC;
  /* Aurora */
  --nord11: #BF616A; /* Error */
  --nord12: #D08770;
  --nord13: #EBCB8B; /* Warning */
  --nord14: #A3BE8C; /* Success */
  --nord15: #B48EAD;
}
```

### 2.5 Nix (Infrastructure)
- **Flakes**: Use `flake.nix` for all dependency management.
- **Formatting**: Ensure nix files are formatted with `nixpkgs-fmt` or `alejandra`.
- **Reproducibility**: Pin inputs in `flake.lock`.

## 3. Data & Directory conventions
- **Config Path**: `~/.openclaw` (Resolve using `os.UserHomeDir`).
- **Data Files**:
    - `logs/error.log` -> Alerts
    - `todo.json` -> Kanban
    - `sessions/active.json` -> Sessions

## 4. Agent Operational Rules

When writing code for this repository, Agents must:

1.  **Check Constraints**: Verify if a standard library solution exists before adding a generic dependency.
2.  **Verify UI**: When generating CSS, ensure contrast ratios match the Nord theme guidelines (High contrast, no transparency).
3.  **Test First**: Create a test case for backend logic before implementing the full feature.
4.  **Flake Integrity**: Do not modify `flake.nix` unless adding a required system tool.
5.  **Documentation**: Update `README.md` if new environment variables or config files are required.
6.  **Idempotency**: Ensure setup scripts or run commands are idempotent.

## 5. Specific Feature Implementation Guides

### Dashboard Refresh Logic
- Implement a `time.Ticker` in Go to refresh data.
- Frontend should poll `/api/status` every 60s.
- Use `fsnotify` if file system events are needed for immediate updates.

### Error Logging
- Use a structured logger (slog or simple log package) writing to stdout/stderr.
- Dashboard should read the application's own logs if required, but primarily monitors the user's system logs defined in configuration.

---
*Generated for OpenClaw Dashboard Development*
