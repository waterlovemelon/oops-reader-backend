# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What This Is

Oops Reader Backend — a Go REST API service for an e-book/reading platform. Uses Gin web framework, MariaDB/MySQL, Viper config, and Zap logging. Module path: `github.com/oops-reader/oops-reader-backend`.

## Build & Run

```bash
# Build the main API server
go build -o bin/api ./cmd/api

# Run the API server (requires config.yaml)
./bin/api

# Run all tests
go test ./...

# Run tests for a single package
go test ./internal/identity/...
go test ./internal/catalog/...

# Run a single test
go test ./internal/identity/ -run TestRegisterLoginRefreshLogoutLifecycle

# Run the integration test (cmd/api)
go test ./cmd/api/...

# Run with verbose output
go test -v ./...

# Tidy dependencies
go mod tidy
```

No Makefile exists. There is no linter configured — use `go vet ./...` for basic checks.

## Configuration

- Copy `config.yaml.example` to `config.yaml` (gitignored)
- Viper loads from `./config.yaml`, `./config/config.yaml`, or `/etc/oops-reader-backend/config.yaml`
- Environment variables override config file values via `viper.AutomaticEnv()`
- All config structs are in `internal/platform/config/config.go`
- private-info private info of server, can connect the server by ssh with password

## Architecture

### Layer Structure

```
cmd/api/main.go          → Wire-up: creates services, handlers, middleware, routes
internal/
  platform/config/        → Config loading (Viper + YAML)
  platform/db/            → MySQL connection pool
  platform/log/           → Zap logger factory
  identity/               → Auth, JWT, sessions, password reset
  catalog/                → Book catalog (EPUB scanning + MySQL)
  content/                → EPUB parsing
  community/              → Forums (in-memory only, no DB yet)
  entitlement/            → User entitlements (stub)
  backup/                 → Reading data backup/restore
  tts/                    → Text-to-speech with pluggable providers
  transport/http/
    handlers/             → HTTP handlers (one file per domain)
    middleware/            → Logger, Recovery, CORS, Auth, RateLimit
```

### Store Interface Pattern

Each domain that needs persistence defines a `Store` interface in its package. The service depends on the interface, not the concrete implementation. This enables testing with in-memory fakes.

```go
// In internal/identity/store.go
type Store interface {
    CreateAccount(ctx context.Context, account Account) (Account, error)
    FindAccountByEmail(ctx context.Context, email string) (Account, error)
    // ...
}

type MySQLStore struct { db *sql.DB }

// In tests — fakeStore implements the same interface
```

Domains using this pattern: `identity`, `catalog`, `backup`. The `community` service uses in-memory maps directly (no Store interface). The `tts` package uses a `TTSProvider` interface for pluggable backends.

### Handler → Service → Store Flow

Handlers are thin: bind JSON, call service, map errors to HTTP status codes. Each handler file has a `writeXxxError(c *gin.Context, err error)` function that translates sentinel errors to status codes using `errors.Is()`.

```go
// Handler binds request, delegates to service
func (h *IdentityHandler) Login(c *gin.Context) {
    var req loginRequest
    if err := c.ShouldBindJSON(&req); err != nil { ... }
    result, err := h.service.Login(c.Request.Context(), identity.LoginInput{...})
    if err != nil { writeIdentityError(c, err); return }
    c.JSON(http.StatusOK, gin.H{"data": authResultJSON(result)})
}
```

### Error Handling

- Domain packages define sentinel errors: `var ErrUserNotFound = errors.New("user not found")`
- Wrap with context: `fmt.Errorf("%w: password must be at least 8 characters", ErrInvalidInput)`
- Handler error mappers use `errors.Is()` to match and return appropriate HTTP status
- Response format: `gin.H{"error": "message"}` or `gin.H{"data": ...}` for success

### Auth Flow

Custom JWT-like tokens (not using a JWT library). Tokens are `base64(json_claims).base64(hmac_sha256_signature)`. Auth middleware extracts `Bearer` token, validates via `identityService.ValidateAccessToken()`, sets `user_id` in Gin context. Handlers retrieve it via `middleware.CurrentUserID(c)`.

### Testing Patterns

- Tests use in-memory fake store implementations that satisfy the `Store` interface
- `identity.Service` exposes a `now` field for time injection: `service.now = fixedClock()`
- Integration tests in `cmd/api/main_test.go` use `httptest.NewRecorder()` against the full router, passing `nil` for db (falls back to memory store)
- Test helpers like `writeCatalogTestEPUB()` create temporary EPUB fixtures

### TTS Provider System

`internal/tts/` defines a `TTSProvider` interface. Providers (Edge, MiMo) are registered at startup based on config. User preferences are stored in `user_tts_prefs` table. Rate limiting is applied per-user via middleware.

## Database Migrations

SQL files in `migrations/` are applied manually (no migration tool):
```bash
mariadb -u root -p oops_reader < migrations/001_init_user_auth.sql
```

Migrations are numbered sequentially. There is no migration 006 (skips from 005 to 007).

## Key Conventions

- **Context propagation**: All service and store methods accept `context.Context` as first parameter
- **Nullable DB fields**: Use `sql.NullString`, `sql.NullTime` — helper `nullableString()` converts empty strings to `sql.NullString`
- **Password hashing**: Uses `golang.org/x/crypto/bcrypt` via `identity.HashPassword()` / `identity.VerifyPassword()`
- **Token hashing**: Refresh tokens are stored as SHA-256 hashes, never plaintext
- **ID generation**: Community uses `crypto/rand` hex IDs with prefixes (e.g., `thr_`, `cmt_`)
- **Catalog book IDs**: Derived from filename via slug + optional SHA1 hash for deduplication
- **Response envelope**: Successful responses wrap data in `gin.H{"data": ...}`
- **Chinese documentation**: README and docs are in Simplified Chinese
