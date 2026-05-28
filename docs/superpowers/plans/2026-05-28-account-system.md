# Account System Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the first production account system for Oops Reader: email/password accounts, persistent sessions, password reset, entitlement extension points, and account-scoped reading data backup/restore.

**Architecture:** Implement the backend first as a database-backed identity and backup service in the existing Go/Gin modular monolith. Then add a Flutter account layer that owns session state and authenticated HTTP calls, followed by UI and local SQLite snapshot backup/restore. Keep logged-out local reading intact and treat VIP as entitlement metadata only.

**Tech Stack:** Go 1.21, Gin, MariaDB, `database/sql`, `golang.org/x/crypto/bcrypt`, Flutter, Dart 3.9, Riverpod, SharedPreferences, SQLite.

---

## Execution Environment

Backend Go commands are executed on the remote server `root@8.136.58.109`, not on the local machine. Local backend changes are synced to `/root/oops-reader-account-system-backend` before running `go test`, builds, migrations, or service deployment. The local machine is used for editing and Git only.

App validation is executed locally with the Linux target through `scripts/run_linux_local.sh`.

## Scope Check

This plan implements one coherent product capability: accounts with first-version cloud backup. It touches backend and app code, but the work is ordered so each phase is testable:

1. Backend database and identity APIs.
2. Backend entitlement and backup APIs.
3. App account session and authenticated client.
4. App login/profile and backup/restore flows.
5. Token cleanup for existing authenticated features.

Full multi-device merge sync, VIP purchase, phone login, third-party login, and email verification are outside this implementation.

## File Structure

### Backend Repository: `oops-reader-backend`

- Create `migrations/007_init_account_system.sql`
  - Adds account fields, reset tokens, entitlements, and backup table.
- Modify `internal/identity/service.go`
  - Replaces in-memory identity with persistent account service APIs.
- Create `internal/identity/store.go`
  - MariaDB queries for users, sessions, password reset tokens, and backup-safe profile access.
- Create `internal/identity/security.go`
  - Password hashing, token hashing, random token generation.
- Modify `internal/identity/service_test.go`
  - Covers registration, login, refresh rotation, logout, password reset, and status checks with fake store.
- Modify `internal/transport/http/handlers/identity.go`
  - Implements auth endpoints and user profile endpoints.
- Modify `internal/transport/http/middleware/middleware.go`
  - Checks access token and account status through the identity service.
- Create `internal/entitlement/service.go`
  - Returns normal-user entitlements and future explicit grants.
- Create `internal/transport/http/handlers/entitlement.go`
  - Implements `GET /v1/account/entitlements`.
- Create `internal/backup/service.go`
  - Stores and retrieves full reading-data snapshots.
- Create `internal/backup/service_test.go`
  - Covers empty upload, conflict, overwrite, summary, and restore payload.
- Create `internal/transport/http/handlers/backup.go`
  - Implements backup summary, upload, and download endpoints.
- Modify `cmd/api/main.go`
  - Wires persistent identity, entitlement, backup services, and routes.
- Modify `docs/api.md`
  - Documents the new account and backup endpoints after implementation.

### App Repository: `../oops-reader-app`

- Create `lib/domain/entities/account.dart`
  - Account user, session, entitlement, backup summary models.
- Create `lib/data/services/account_api_client.dart`
  - Auth API calls, token refresh, and authenticated request wrapper.
- Create `lib/data/repositories/account_repository_impl.dart`
  - Session persistence and account operations.
- Create `lib/domain/repositories/account_repository.dart`
  - App-facing account repository contract.
- Create `lib/core/providers/account_providers.dart`
  - Riverpod providers for session, current user, entitlements, backup state.
- Modify `lib/core/providers/auth_providers.dart`
  - Derives signed-in state from account session instead of `user_signed_in`.
- Modify `lib/data/services/community_api_client.dart`
  - Removes hidden guest-token creation and accepts bearer tokens from account layer.
- Modify `lib/core/providers/community_providers.dart`
  - Injects account-aware transport/token lookup into community API usage.
- Modify `lib/presentation/pages/profile/login_page.dart`
  - Adds login, register, and forgot-password UI.
- Modify `lib/presentation/pages/profile/profile_page.dart`
  - Shows account-aware logged-in/logged-out profile header.
- Create `lib/presentation/pages/profile/account_settings_page.dart`
  - Logout, password reset entry, cloud backup, cloud restore.
- Create `lib/data/services/reading_backup_snapshot_service.dart`
  - Exports/imports local SQLite reading data snapshot.
- Add tests under `test/account/` and `test/backup/`
  - Account repository, API client refresh, snapshot export/import, and backup conflict behavior.

---

## Task 1: Backend Account Migration

**Files:**
- Create: `migrations/007_init_account_system.sql`
- Modify: `README.md`

- [ ] **Step 1: Create the migration**

Create `migrations/007_init_account_system.sql` with this SQL:

```sql
-- Account system tables and fields
-- Date: 2026-05-28

ALTER TABLE `users`
    ADD COLUMN IF NOT EXISTS `account_status` VARCHAR(32) NOT NULL DEFAULT 'active' COMMENT 'active/frozen/deactivated' AFTER `avatar_url`,
    ADD COLUMN IF NOT EXISTS `account_type` VARCHAR(32) NOT NULL DEFAULT 'normal' COMMENT 'normal, future vip' AFTER `account_status`,
    ADD COLUMN IF NOT EXISTS `email_verified_at` DATETIME NULL COMMENT 'Email verification time' AFTER `account_type`;

CREATE INDEX IF NOT EXISTS `idx_users_account_status` ON `users` (`account_status`);
CREATE INDEX IF NOT EXISTS `idx_users_account_type` ON `users` (`account_type`);

ALTER TABLE `user_sessions`
    ADD COLUMN IF NOT EXISTS `revoked_at` DATETIME NULL COMMENT 'Revocation time' AFTER `expires_at`;

CREATE TABLE IF NOT EXISTS `password_reset_tokens` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY COMMENT 'Primary key',
    `user_id` BIGINT UNSIGNED NOT NULL COMMENT 'User ID',
    `email` VARCHAR(128) NOT NULL COMMENT 'Email used for reset',
    `token_hash` VARCHAR(255) NOT NULL COMMENT 'Reset token hash',
    `expires_at` DATETIME NOT NULL COMMENT 'Expiration time',
    `used_at` DATETIME NULL COMMENT 'Used time',
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Created time',
    UNIQUE KEY `uk_token_hash` (`token_hash`),
    INDEX `idx_user_id` (`user_id`),
    INDEX `idx_email_created_at` (`email`, `created_at`),
    INDEX `idx_expires_at` (`expires_at`),
    CONSTRAINT `fk_password_reset_tokens_user_id` FOREIGN KEY (`user_id`) REFERENCES `users`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Password reset tokens';

CREATE TABLE IF NOT EXISTS `account_entitlements` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY COMMENT 'Primary key',
    `user_id` BIGINT UNSIGNED NOT NULL COMMENT 'User ID',
    `entitlement_key` VARCHAR(64) NOT NULL COMMENT 'Capability key',
    `status` VARCHAR(32) NOT NULL DEFAULT 'active' COMMENT 'active/inactive/revoked',
    `source` VARCHAR(32) NOT NULL DEFAULT 'system' COMMENT 'system/purchase/admin/promo',
    `starts_at` DATETIME NULL COMMENT 'Start time',
    `expires_at` DATETIME NULL COMMENT 'Expiration time',
    `metadata` JSON NULL COMMENT 'Extended metadata',
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Created time',
    `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'Updated time',
    UNIQUE KEY `uk_user_entitlement` (`user_id`, `entitlement_key`),
    INDEX `idx_user_status` (`user_id`, `status`),
    INDEX `idx_expires_at` (`expires_at`),
    CONSTRAINT `fk_account_entitlements_user_id` FOREIGN KEY (`user_id`) REFERENCES `users`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Account entitlements';

CREATE TABLE IF NOT EXISTS `user_data_backups` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY COMMENT 'Primary key',
    `user_id` BIGINT UNSIGNED NOT NULL COMMENT 'User ID',
    `schema_version` INT UNSIGNED NOT NULL COMMENT 'Snapshot schema version',
    `payload` JSON NOT NULL COMMENT 'Reading data snapshot',
    `book_count` INT UNSIGNED NOT NULL DEFAULT 0 COMMENT 'Book count',
    `note_count` INT UNSIGNED NOT NULL DEFAULT 0 COMMENT 'Note count',
    `progress_count` INT UNSIGNED NOT NULL DEFAULT 0 COMMENT 'Progress count',
    `preference_count` INT UNSIGNED NOT NULL DEFAULT 0 COMMENT 'Preference count',
    `source_device_id` VARCHAR(128) NOT NULL COMMENT 'Source device ID',
    `source_device_name` VARCHAR(128) NULL COMMENT 'Source device name',
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Created time',
    `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'Updated time',
    UNIQUE KEY `uk_user_id` (`user_id`),
    INDEX `idx_updated_at` (`updated_at`),
    CONSTRAINT `fk_user_data_backups_user_id` FOREIGN KEY (`user_id`) REFERENCES `users`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Latest user reading data backup';
```

- [ ] **Step 2: Check MariaDB compatibility**

Run:

```bash
go test ./...
```

Expected: tests may fail because migration is not executed by Go tests yet, but there must be no SQL file formatting side effects in `git diff --check`.

- [ ] **Step 3: Update migration docs**

Modify `README.md` deployment migration list to include:

```bash
mariadb -u root -p oops_reader < migrations/007_init_account_system.sql
```

- [ ] **Step 4: Commit**

```bash
git add migrations/007_init_account_system.sql README.md
git commit -m "feat: add account system migration"
```

---

## Task 2: Backend Identity Security Helpers

**Files:**
- Create: `internal/identity/security.go`
- Test: `internal/identity/security_test.go`
- Modify: `go.mod`

- [ ] **Step 1: Write security helper tests**

Create `internal/identity/security_test.go`:

```go
package identity

import "testing"

func TestPasswordHashAndVerify(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("HashPassword returned error: %v", err)
	}
	if hash == "" || hash == "correct horse battery staple" {
		t.Fatalf("HashPassword returned unsafe hash %q", hash)
	}
	if !VerifyPassword(hash, "correct horse battery staple") {
		t.Fatal("VerifyPassword rejected the original password")
	}
	if VerifyPassword(hash, "wrong password") {
		t.Fatal("VerifyPassword accepted the wrong password")
	}
}

func TestRandomTokenAndTokenHash(t *testing.T) {
	token, err := NewOpaqueToken()
	if err != nil {
		t.Fatalf("NewOpaqueToken returned error: %v", err)
	}
	if len(token) < 32 {
		t.Fatalf("token length = %d, want at least 32", len(token))
	}
	hash := HashToken(token)
	if hash == "" || hash == token {
		t.Fatalf("HashToken returned unsafe hash %q", hash)
	}
	if HashToken(token) != hash {
		t.Fatal("HashToken is not deterministic")
	}
}
```

- [ ] **Step 2: Run tests to verify failure**

Run:

```bash
go test ./internal/identity -run 'TestPasswordHashAndVerify|TestRandomTokenAndTokenHash' -count=1
```

Expected: compile failure because helper functions are not defined.

- [ ] **Step 3: Implement helpers**

Create `internal/identity/security.go`:

```go
package identity

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"

	"golang.org/x/crypto/bcrypt"
)

func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func VerifyPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

func NewOpaqueToken() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b[:]), nil
}

func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
```

- [ ] **Step 4: Add dependency**

Run:

```bash
go get golang.org/x/crypto/bcrypt
go mod tidy
```

Expected: `go.mod` keeps `golang.org/x/crypto` and `go.sum` may be updated or created according to the repository's module state.

- [ ] **Step 5: Run tests**

Run:

```bash
go test ./internal/identity -run 'TestPasswordHashAndVerify|TestRandomTokenAndTokenHash' -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add go.mod go.sum internal/identity/security.go internal/identity/security_test.go
git commit -m "feat: add identity security helpers"
```

---

## Task 3: Backend Persistent Identity Store

**Files:**
- Create: `internal/identity/store.go`
- Test: `internal/identity/store_test.go`

- [ ] **Step 1: Define store contract and models**

Create `internal/identity/store.go` with the store interface and SQL-backed struct:

```go
package identity

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

const (
	AccountStatusActive      = "active"
	AccountStatusFrozen      = "frozen"
	AccountStatusDeactivated = "deactivated"
	AccountTypeNormal        = "normal"
)

type Account struct {
	ID              uint64
	Email           string
	Phone           string
	PasswordHash    string
	Nickname        string
	AvatarURL       string
	AccountStatus   string
	AccountType     string
	EmailVerifiedAt sql.NullTime
	LastLoginAt     sql.NullTime
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type Session struct {
	ID               uint64
	UserID           uint64
	DeviceID         string
	DeviceName       string
	Platform         string
	RefreshTokenHash string
	ExpiresAt        time.Time
	RevokedAt        sql.NullTime
	LastActiveAt     time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type PasswordResetToken struct {
	ID        uint64
	UserID    uint64
	Email     string
	TokenHash string
	ExpiresAt time.Time
	UsedAt    sql.NullTime
	CreatedAt time.Time
}

type Store interface {
	CreateAccount(ctx context.Context, account Account) (Account, error)
	FindAccountByEmail(ctx context.Context, email string) (Account, error)
	FindAccountByID(ctx context.Context, userID uint64) (Account, error)
	UpdateLastLogin(ctx context.Context, userID uint64, at time.Time) error
	UpdateProfile(ctx context.Context, userID uint64, nickname, avatarURL string) (Account, error)
	UpdatePasswordHash(ctx context.Context, userID uint64, passwordHash string) error
	CreateSession(ctx context.Context, session Session) (Session, error)
	FindSessionByRefreshHash(ctx context.Context, refreshHash string) (Session, error)
	RevokeSession(ctx context.Context, sessionID uint64, at time.Time) error
	RevokeUserSessions(ctx context.Context, userID uint64, at time.Time) error
	CreatePasswordResetToken(ctx context.Context, token PasswordResetToken) (PasswordResetToken, error)
	FindPasswordResetToken(ctx context.Context, tokenHash string) (PasswordResetToken, error)
	MarkPasswordResetTokenUsed(ctx context.Context, tokenID uint64, at time.Time) error
}

var ErrDuplicateEmail = errors.New("duplicate email")

type MySQLStore struct {
	db *sql.DB
}

func NewMySQLStore(db *sql.DB) *MySQLStore {
	return &MySQLStore{db: db}
}
```

- [ ] **Step 2: Add SQL methods**

Append SQL implementations in the same file. Keep each method small and use `context.Context`. Convert duplicate email errors from MySQL error number `1062` to `ErrDuplicateEmail`.

Required method signatures:

```go
func (s *MySQLStore) CreateAccount(ctx context.Context, account Account) (Account, error)
func (s *MySQLStore) FindAccountByEmail(ctx context.Context, email string) (Account, error)
func (s *MySQLStore) FindAccountByID(ctx context.Context, userID uint64) (Account, error)
func (s *MySQLStore) UpdateLastLogin(ctx context.Context, userID uint64, at time.Time) error
func (s *MySQLStore) UpdateProfile(ctx context.Context, userID uint64, nickname, avatarURL string) (Account, error)
func (s *MySQLStore) UpdatePasswordHash(ctx context.Context, userID uint64, passwordHash string) error
func (s *MySQLStore) CreateSession(ctx context.Context, session Session) (Session, error)
func (s *MySQLStore) FindSessionByRefreshHash(ctx context.Context, refreshHash string) (Session, error)
func (s *MySQLStore) RevokeSession(ctx context.Context, sessionID uint64, at time.Time) error
func (s *MySQLStore) RevokeUserSessions(ctx context.Context, userID uint64, at time.Time) error
func (s *MySQLStore) CreatePasswordResetToken(ctx context.Context, token PasswordResetToken) (PasswordResetToken, error)
func (s *MySQLStore) FindPasswordResetToken(ctx context.Context, tokenHash string) (PasswordResetToken, error)
func (s *MySQLStore) MarkPasswordResetTokenUsed(ctx context.Context, tokenID uint64, at time.Time) error
```

- [ ] **Step 3: Compile store**

Run:

```bash
go test ./internal/identity -run TestNonExistent -count=1
```

Expected: package compiles.

- [ ] **Step 4: Commit**

```bash
git add internal/identity/store.go
git commit -m "feat: add persistent identity store"
```

---

## Task 4: Backend Identity Service

**Files:**
- Modify: `internal/identity/service.go`
- Modify: `internal/identity/service_test.go`

- [ ] **Step 1: Replace service tests with account lifecycle tests**

Update `internal/identity/service_test.go` to cover:

```go
func TestRegisterLoginRefreshLogoutLifecycle(t *testing.T)
func TestRegisterRejectsDuplicateEmail(t *testing.T)
func TestLoginRejectsWrongPassword(t *testing.T)
func TestFrozenAccountCannotLogin(t *testing.T)
func TestPasswordResetRevokesSessions(t *testing.T)
```

Use an in-memory fake implementing `Store`, not MariaDB. The fake must enforce unique email and refresh token lookup.

- [ ] **Step 2: Run tests to verify failure**

Run:

```bash
go test ./internal/identity -count=1
```

Expected: failure because service methods are not implemented.

- [ ] **Step 3: Implement account service API**

Modify `internal/identity/service.go` to expose these methods:

```go
type Service struct {
	store Store
	secret []byte
	now func() time.Time
	sendPasswordResetEmail func(email, token string) error
}

func NewService(store Store, secret string) *Service
func (s *Service) Register(ctx context.Context, input RegisterInput) (AuthResult, error)
func (s *Service) Login(ctx context.Context, input LoginInput) (AuthResult, error)
func (s *Service) Refresh(ctx context.Context, refreshToken string) (AuthResult, error)
func (s *Service) Logout(ctx context.Context, refreshToken string) error
func (s *Service) RequestPasswordReset(ctx context.Context, email string) error
func (s *Service) ConfirmPasswordReset(ctx context.Context, token, newPassword string) error
func (s *Service) GetUser(ctx context.Context, userID uint64) (Account, error)
func (s *Service) UpdateProfile(ctx context.Context, userID uint64, nickname, avatarURL string) (Account, error)
func (s *Service) ValidateAccessToken(ctx context.Context, token string) (TokenClaims, error)
```

Define input/result types in `service.go`:

```go
type RegisterInput struct {
	Email string
	Password string
	Nickname string
	DeviceID string
	DeviceName string
	Platform string
}

type LoginInput struct {
	Email string
	Password string
	DeviceID string
	DeviceName string
	Platform string
}

type AuthResult struct {
	User Account
	AccessToken string
	RefreshToken string
	TokenType string
}
```

- [ ] **Step 4: Preserve token validation compatibility**

Keep `TokenClaims` with `UserID`, `Type`, `IssuedAt`, `ExpiresAt`, and `Nonce`, but change `UserID` to `uint64` if possible. If JSON compatibility with existing middleware is easier with string IDs, parse to `uint64` at the middleware boundary.

- [ ] **Step 5: Run tests**

Run:

```bash
go test ./internal/identity -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/identity/service.go internal/identity/service_test.go
git commit -m "feat: implement persistent identity service"
```

---

## Task 5: Backend Auth Handlers and Middleware

**Files:**
- Modify: `internal/transport/http/handlers/identity.go`
- Modify: `internal/transport/http/middleware/middleware.go`
- Modify: `cmd/api/main.go`
- Test: `cmd/api/main_test.go`

- [ ] **Step 1: Add handler tests**

Update `cmd/api/main_test.go` with HTTP tests:

```go
func TestRegisterLoginMeAndRefresh(t *testing.T)
func TestDuplicateRegisterReturnsConflict(t *testing.T)
func TestProtectedRouteRequiresBearerToken(t *testing.T)
func TestPasswordResetRequestIsGeneric(t *testing.T)
```

Use a fake identity store and `setupRouter` where possible. If `setupRouter` currently requires `*sql.DB`, extract a `setupRouterWithServices` helper that accepts services for tests.

- [ ] **Step 2: Run tests to verify failure**

Run:

```bash
go test ./cmd/api -run 'TestRegisterLoginMeAndRefresh|TestDuplicateRegisterReturnsConflict|TestProtectedRouteRequiresBearerToken|TestPasswordResetRequestIsGeneric' -count=1
```

Expected: failure because routes still point to placeholder handlers or old service signatures.

- [ ] **Step 3: Implement auth handlers**

Update `internal/transport/http/handlers/identity.go` to implement:

```go
func (h *IdentityHandler) Register(c *gin.Context)
func (h *IdentityHandler) Login(c *gin.Context)
func (h *IdentityHandler) Refresh(c *gin.Context)
func (h *IdentityHandler) Logout(c *gin.Context)
func (h *IdentityHandler) RequestPasswordReset(c *gin.Context)
func (h *IdentityHandler) ConfirmPasswordReset(c *gin.Context)
func (h *IdentityHandler) GetCurrentUser(c *gin.Context)
func (h *IdentityHandler) UpdateCurrentUser(c *gin.Context)
```

Use response helpers:

```go
func authResultJSON(result identity.AuthResult) gin.H
func accountJSON(account identity.Account) gin.H
func writeIdentityError(c *gin.Context, err error)
```

Map errors:

- `identity.ErrInvalidInput` to `400`.
- `identity.ErrDuplicateEmail` to `409`.
- `identity.ErrInvalidToken`, `identity.ErrExpiredToken`, `identity.ErrRefreshRevoked` to `401`.
- `identity.ErrAccountForbidden` to `403`.
- `identity.ErrUserNotFound` to `404`.

- [ ] **Step 4: Update middleware**

Change `middleware.Auth` to call `identityService.ValidateAccessToken(c.Request.Context(), tokenString)` and store numeric user ID:

```go
c.Set("user_id", claims.UserID)
```

Update `CurrentUserID` to return `uint64`.

- [ ] **Step 5: Wire routes**

Modify `cmd/api/main.go`:

- Construct `identityStore := identity.NewMySQLStore(db)`.
- Construct `identityService := identity.NewService(identityStore, cfg.JWT.Secret)`.
- Replace placeholder `handlers.Login`, `handlers.Register`, `handlers.UpdateCurrentUser` routes with `identityHandler` methods.
- Add `POST /v1/auth/logout`, `POST /v1/auth/password/reset-request`, `POST /v1/auth/password/reset-confirm`.

- [ ] **Step 6: Run tests**

Run:

```bash
go test ./cmd/api ./internal/transport/http/... ./internal/identity -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add cmd/api/main.go cmd/api/main_test.go internal/transport/http/handlers/identity.go internal/transport/http/middleware/middleware.go
git commit -m "feat: expose account auth endpoints"
```

---

## Task 6: Backend Entitlement Service

**Files:**
- Create: `internal/entitlement/service.go`
- Create: `internal/entitlement/service_test.go`
- Create: `internal/transport/http/handlers/entitlement.go`
- Modify: `cmd/api/main.go`

- [ ] **Step 1: Write entitlement tests**

Create `internal/entitlement/service_test.go`:

```go
package entitlement

import "testing"

func TestDefaultNormalEntitlements(t *testing.T) {
	service := NewService(nil)
	result, err := service.ListForUser(123, "normal")
	if err != nil {
		t.Fatalf("ListForUser returned error: %v", err)
	}
	if result.AccountType != "normal" {
		t.Fatalf("AccountType = %q, want normal", result.AccountType)
	}
	if !result.Has("backup.reading_data") {
		t.Fatal("normal account missing backup.reading_data entitlement")
	}
	if result.Has("sync.multi_device") {
		t.Fatal("normal account unexpectedly has sync.multi_device")
	}
}
```

- [ ] **Step 2: Run test to verify failure**

Run:

```bash
go test ./internal/entitlement -count=1
```

Expected: failure because package does not exist.

- [ ] **Step 3: Implement service**

Create `internal/entitlement/service.go`:

```go
package entitlement

type Entitlement struct {
	Key string `json:"key"`
	Status string `json:"status"`
	ExpiresAt *string `json:"expires_at"`
}

type Result struct {
	AccountType string `json:"account_type"`
	Entitlements []Entitlement `json:"entitlements"`
}

func (r Result) Has(key string) bool {
	for _, entitlement := range r.Entitlements {
		if entitlement.Key == key && entitlement.Status == "active" {
			return true
		}
	}
	return false
}

type Service struct{}

func NewService(_ any) *Service {
	return &Service{}
}

func (s *Service) ListForUser(userID uint64, accountType string) (Result, error) {
	return Result{
		AccountType: accountType,
		Entitlements: []Entitlement{
			{Key: "backup.reading_data", Status: "active", ExpiresAt: nil},
		},
	}, nil
}
```

- [ ] **Step 4: Add HTTP handler**

Create `internal/transport/http/handlers/entitlement.go`:

```go
package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/oops-reader/oops-reader-backend/internal/entitlement"
	"github.com/oops-reader/oops-reader-backend/internal/identity"
	"github.com/oops-reader/oops-reader-backend/internal/transport/http/middleware"
)

type EntitlementHandler struct {
	identityService *identity.Service
	entitlementService *entitlement.Service
}

func NewEntitlementHandler(identityService *identity.Service, entitlementService *entitlement.Service) *EntitlementHandler {
	return &EntitlementHandler{identityService: identityService, entitlementService: entitlementService}
}

func (h *EntitlementHandler) List(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authorization required"})
		return
	}
	account, err := h.identityService.GetUser(c.Request.Context(), userID)
	if err != nil {
		writeIdentityError(c, err)
		return
	}
	result, err := h.entitlementService.ListForUser(userID, account.AccountType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": result})
}
```

- [ ] **Step 5: Wire route and run tests**

Add `GET /v1/account/entitlements` under auth middleware in `cmd/api/main.go`.

Run:

```bash
go test ./internal/entitlement ./cmd/api -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/entitlement internal/transport/http/handlers/entitlement.go cmd/api/main.go
git commit -m "feat: add account entitlements endpoint"
```

---

## Task 7: Backend Reading Data Backup

**Files:**
- Create: `internal/backup/service.go`
- Create: `internal/backup/service_test.go`
- Create: `internal/transport/http/handlers/backup.go`
- Modify: `cmd/api/main.go`

- [ ] **Step 1: Write backup service tests**

Create `internal/backup/service_test.go` with tests:

```go
func TestUploadCreateIfEmptySucceeds(t *testing.T)
func TestUploadCreateIfEmptyConflictsWhenBackupExists(t *testing.T)
func TestUploadOverwriteReplacesBackup(t *testing.T)
func TestDownloadReturnsLatestPayload(t *testing.T)
```

Use a fake store with a map keyed by `userID`.

- [ ] **Step 2: Define backup service**

Create `internal/backup/service.go` with:

```go
package backup

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

var ErrCloudDataExists = errors.New("cloud backup already exists")

type UploadMode string

const (
	ModeCreateIfEmpty UploadMode = "create_if_empty"
	ModeOverwrite UploadMode = "overwrite"
)

type Backup struct {
	ID uint64
	UserID uint64
	SchemaVersion uint
	Payload json.RawMessage
	BookCount uint
	NoteCount uint
	ProgressCount uint
	PreferenceCount uint
	SourceDeviceID string
	SourceDeviceName string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Summary struct {
	Exists bool `json:"exists"`
	BackupID string `json:"backup_id,omitempty"`
	BookCount uint `json:"book_count"`
	NoteCount uint `json:"note_count"`
	ProgressCount uint `json:"progress_count"`
	PreferenceCount uint `json:"preference_count"`
	UpdatedAt *time.Time `json:"updated_at,omitempty"`
	DeviceName string `json:"device_name,omitempty"`
}

type Store interface {
	GetLatest(ctx context.Context, userID uint64) (Backup, bool, error)
	UpsertLatest(ctx context.Context, backup Backup) (Backup, error)
}

type Service struct {
	store Store
}

func NewService(store Store) *Service {
	return &Service{store: store}
}
```

Add methods:

```go
func (s *Service) Summary(ctx context.Context, userID uint64) (Summary, error)
func (s *Service) Upload(ctx context.Context, mode UploadMode, backup Backup) (Backup, error)
func (s *Service) Download(ctx context.Context, userID uint64) (Backup, bool, error)
```

- [ ] **Step 3: Add MySQL backup store**

In `internal/backup/service.go`, add `MySQLStore` with:

```go
func NewMySQLStore(db *sql.DB) *MySQLStore
func (s *MySQLStore) GetLatest(ctx context.Context, userID uint64) (Backup, bool, error)
func (s *MySQLStore) UpsertLatest(ctx context.Context, backup Backup) (Backup, error)
```

Use `INSERT ... ON DUPLICATE KEY UPDATE` for `user_data_backups`.

- [ ] **Step 4: Add HTTP handler**

Create `internal/transport/http/handlers/backup.go` with handlers:

```go
func (h *BackupHandler) Summary(c *gin.Context)
func (h *BackupHandler) Upload(c *gin.Context)
func (h *BackupHandler) Download(c *gin.Context)
```

On `backup.ErrCloudDataExists`, return:

```json
{
  "error": "cloud backup already exists",
  "code": "cloud_data_exists",
  "data": {
    "summary": {}
  }
}
```

- [ ] **Step 5: Wire routes**

In `cmd/api/main.go`, add:

```go
backupRoutes := api.Group("/backup")
backupRoutes.Use(authRequired)
{
	backupRoutes.GET("/reading-data/summary", backupHandler.Summary)
	backupRoutes.POST("/reading-data", backupHandler.Upload)
	backupRoutes.GET("/reading-data", backupHandler.Download)
}
```

- [ ] **Step 6: Run tests**

Run:

```bash
go test ./internal/backup ./cmd/api -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/backup internal/transport/http/handlers/backup.go cmd/api/main.go
git commit -m "feat: add reading data backup endpoints"
```

---

## Task 8: App Account Models and Repository Contract

**Files in `../oops-reader-app`:**
- Create: `lib/domain/entities/account.dart`
- Create: `lib/domain/repositories/account_repository.dart`
- Test: `test/account/account_model_test.dart`

- [ ] **Step 1: Add model tests**

Create `test/account/account_model_test.dart`:

```dart
import 'package:flutter_test/flutter_test.dart';
import 'package:myreader/domain/entities/account.dart';

void main() {
  test('AccountUser parses JSON', () {
    final user = AccountUser.fromJson({
      'id': 1,
      'email': 'reader@example.com',
      'nickname': 'Reader',
      'avatar_url': '',
      'account_status': 'active',
      'account_type': 'normal',
    });

    expect(user.id, '1');
    expect(user.email, 'reader@example.com');
    expect(user.accountType, 'normal');
    expect(user.isActive, isTrue);
  });

  test('AccountEntitlements checks active key', () {
    final entitlements = AccountEntitlements.fromJson({
      'account_type': 'normal',
      'entitlements': [
        {'key': 'backup.reading_data', 'status': 'active', 'expires_at': null},
      ],
    });

    expect(entitlements.has('backup.reading_data'), isTrue);
    expect(entitlements.has('sync.multi_device'), isFalse);
  });
}
```

- [ ] **Step 2: Create account entities**

Create `lib/domain/entities/account.dart` with immutable classes:

```dart
class AccountUser {
  final String id;
  final String email;
  final String nickname;
  final String avatarUrl;
  final String accountStatus;
  final String accountType;

  const AccountUser({
    required this.id,
    required this.email,
    required this.nickname,
    required this.avatarUrl,
    required this.accountStatus,
    required this.accountType,
  });

  bool get isActive => accountStatus == 'active';

  factory AccountUser.fromJson(Map<String, dynamic> json) {
    return AccountUser(
      id: json['id'].toString(),
      email: json['email']?.toString() ?? '',
      nickname: json['nickname']?.toString() ?? '',
      avatarUrl: json['avatar_url']?.toString() ?? '',
      accountStatus: json['account_status']?.toString() ?? 'active',
      accountType: json['account_type']?.toString() ?? 'normal',
    );
  }

  Map<String, dynamic> toJson() => {
    'id': id,
    'email': email,
    'nickname': nickname,
    'avatar_url': avatarUrl,
    'account_status': accountStatus,
    'account_type': accountType,
  };
}

class AccountSession {
  final AccountUser user;
  final String accessToken;
  final String refreshToken;
  final String tokenType;

  const AccountSession({
    required this.user,
    required this.accessToken,
    required this.refreshToken,
    this.tokenType = 'Bearer',
  });
}

class AccountEntitlement {
  final String key;
  final String status;
  final DateTime? expiresAt;

  const AccountEntitlement({
    required this.key,
    required this.status,
    required this.expiresAt,
  });

  factory AccountEntitlement.fromJson(Map<String, dynamic> json) {
    final expiresText = json['expires_at']?.toString();
    return AccountEntitlement(
      key: json['key']?.toString() ?? '',
      status: json['status']?.toString() ?? '',
      expiresAt: expiresText == null || expiresText.isEmpty ? null : DateTime.tryParse(expiresText),
    );
  }
}

class AccountEntitlements {
  final String accountType;
  final List<AccountEntitlement> entitlements;

  const AccountEntitlements({
    required this.accountType,
    required this.entitlements,
  });

  bool has(String key) {
    return entitlements.any((item) => item.key == key && item.status == 'active');
  }

  factory AccountEntitlements.fromJson(Map<String, dynamic> json) {
    final items = json['entitlements'] is List<dynamic>
        ? (json['entitlements'] as List<dynamic>)
        : const <dynamic>[];
    return AccountEntitlements(
      accountType: json['account_type']?.toString() ?? 'normal',
      entitlements: items
          .cast<Map<String, dynamic>>()
          .map(AccountEntitlement.fromJson)
          .toList(growable: false),
    );
  }
}
```

- [ ] **Step 3: Add repository contract**

Create `lib/domain/repositories/account_repository.dart`:

```dart
import 'package:myreader/domain/entities/account.dart';

abstract class AccountRepository {
  Future<AccountSession?> loadSession();
  Future<AccountSession> register({
    required String email,
    required String password,
    required String nickname,
  });
  Future<AccountSession> login({
    required String email,
    required String password,
  });
  Future<AccountSession> refresh();
  Future<void> logout();
  Future<void> requestPasswordReset(String email);
  Future<AccountUser> getCurrentUser();
  Future<AccountEntitlements> getEntitlements();
}
```

- [ ] **Step 4: Run tests**

Run from `../oops-reader-app`:

```bash
scripts/flutter-local.sh test test/account/account_model_test.dart
```

Expected: PASS.

- [ ] **Step 5: Commit in app repo**

```bash
git add lib/domain/entities/account.dart lib/domain/repositories/account_repository.dart test/account/account_model_test.dart
git commit -m "feat: add account domain models"
```

---

## Task 9: App Account API Client and Providers

**Files in `../oops-reader-app`:**
- Create: `lib/data/services/account_api_client.dart`
- Create: `lib/data/repositories/account_repository_impl.dart`
- Create: `lib/core/providers/account_providers.dart`
- Modify: `lib/core/providers/auth_providers.dart`
- Test: `test/account/account_repository_test.dart`

- [ ] **Step 1: Write repository tests**

Create `test/account/account_repository_test.dart` with tests for:

```dart
void main() {
  test('login stores session in preferences', () async {
    SharedPreferences.setMockInitialValues({});
    final prefs = await SharedPreferences.getInstance();
    final repository = AccountRepositoryImpl(
      sharedPreferences: prefs,
      apiClient: AccountApiClient(
        baseUrl: Uri.parse('http://example.test/v1/'),
        transport: (request) async {
          expect(request.uri.path, '/v1/auth/login');
          return const AccountResponse(
            statusCode: 200,
            body: '{"data":{"user":{"id":1,"email":"reader@example.com","nickname":"Reader","avatar_url":"","account_status":"active","account_type":"normal"},"access_token":"access-1","refresh_token":"refresh-1","token_type":"Bearer"}}',
          );
        },
      ),
    );

    final session = await repository.login(
      email: 'reader@example.com',
      password: 'password12345',
    );

    expect(session.accessToken, 'access-1');
    expect(prefs.getString('account_access_token'), 'access-1');
    expect(prefs.getString('account_refresh_token'), 'refresh-1');
  });

  test('refresh replaces stored tokens', () async {
    SharedPreferences.setMockInitialValues({
      'account_refresh_token': 'refresh-1',
      'account_user_json': '{"id":"1","email":"reader@example.com","nickname":"Reader","avatar_url":"","account_status":"active","account_type":"normal"}',
    });
    final prefs = await SharedPreferences.getInstance();
    final repository = AccountRepositoryImpl(
      sharedPreferences: prefs,
      apiClient: AccountApiClient(
        baseUrl: Uri.parse('http://example.test/v1/'),
        transport: (request) async => const AccountResponse(
          statusCode: 200,
          body: '{"data":{"user":{"id":1,"email":"reader@example.com","nickname":"Reader","avatar_url":"","account_status":"active","account_type":"normal"},"access_token":"access-2","refresh_token":"refresh-2","token_type":"Bearer"}}',
        ),
      ),
    );

    final session = await repository.refresh();

    expect(session.accessToken, 'access-2');
    expect(prefs.getString('account_refresh_token'), 'refresh-2');
  });

  test('logout clears stored session', () async {
    SharedPreferences.setMockInitialValues({
      'account_access_token': 'access-1',
      'account_refresh_token': 'refresh-1',
      'account_user_json': '{"id":"1","email":"reader@example.com","nickname":"Reader","avatar_url":"","account_status":"active","account_type":"normal"}',
    });
    final prefs = await SharedPreferences.getInstance();
    final repository = AccountRepositoryImpl(
      sharedPreferences: prefs,
      apiClient: AccountApiClient(
        baseUrl: Uri.parse('http://example.test/v1/'),
        transport: (request) async => const AccountResponse(statusCode: 204, body: ''),
      ),
    );

    await repository.logout();

    expect(prefs.getString('account_access_token'), isNull);
    expect(prefs.getString('account_refresh_token'), isNull);
    expect(prefs.getString('account_user_json'), isNull);
  });
}
```

Use `SharedPreferences.setMockInitialValues({})` and a fake transport that returns JSON payloads for `/auth/login`, `/auth/refresh`, and `/auth/logout`.

- [ ] **Step 2: Implement API client**

Create `lib/data/services/account_api_client.dart` with:

```dart
class AccountRequest {
  final String method;
  final Uri uri;
  final Map<String, String> headers;
  final List<int>? bodyBytes;

  const AccountRequest({
    required this.method,
    required this.uri,
    this.headers = const {},
    this.bodyBytes,
  });
}

class AccountResponse {
  final int statusCode;
  final String body;

  const AccountResponse({
    required this.statusCode,
    required this.body,
  });
}

class AccountApiClient {
  final Uri baseUrl;
  final Future<AccountResponse> Function(AccountRequest request) transport;

  AccountApiClient({required this.baseUrl, required this.transport});

  Future<Map<String, dynamic>> register(Map<String, dynamic> body);
  Future<Map<String, dynamic>> login(Map<String, dynamic> body);
  Future<Map<String, dynamic>> refresh(String refreshToken);
  Future<void> logout(String accessToken, String refreshToken);
  Future<void> requestPasswordReset(String email);
  Future<Map<String, dynamic>> getMe(String accessToken);
  Future<Map<String, dynamic>> getEntitlements(String accessToken);
}
```

Use `dart:io` `HttpClient` for the default transport, following the existing `CommunityApiClient` transport pattern.

- [ ] **Step 3: Implement repository**

Create `lib/data/repositories/account_repository_impl.dart`. It must:

- Store `account_access_token`, `account_refresh_token`, and `account_user_json`.
- Generate or reuse stable `account_device_id`.
- Parse auth responses into `AccountSession`.
- Clear session on logout even when the network request fails.

- [ ] **Step 4: Add providers**

Create `lib/core/providers/account_providers.dart` with:

```dart
final accountBaseUriProvider = Provider<Uri>((ref) {
  return Uri.parse(AppConstants.communityBaseUrl.replaceFirst('/community/', '/'));
});

final accountRepositoryProvider = Provider<AccountRepository>((ref) {
  return AccountRepositoryImpl(
    apiClient: ref.watch(accountApiClientProvider),
    sharedPreferences: ref.watch(sharedPreferencesProvider),
  );
});

final accountSessionProvider = FutureProvider<AccountSession?>((ref) {
  return ref.watch(accountRepositoryProvider).loadSession();
});
```

- [ ] **Step 5: Update auth provider**

Modify `lib/core/providers/auth_providers.dart` so signed-in state derives from `accountSessionProvider`:

```dart
final isUserSignedInProvider = Provider<bool>((ref) {
  final session = ref.watch(accountSessionProvider);
  return session.maybeWhen(data: (value) => value != null, orElse: () => false);
});
```

- [ ] **Step 6: Run tests**

Run:

```bash
scripts/flutter-local.sh test test/account/account_repository_test.dart
```

Expected: PASS.

- [ ] **Step 7: Commit in app repo**

```bash
git add lib/data/services/account_api_client.dart lib/data/repositories/account_repository_impl.dart lib/core/providers/account_providers.dart lib/core/providers/auth_providers.dart test/account/account_repository_test.dart
git commit -m "feat: add account session repository"
```

---

## Task 10: App Login and Profile UI

**Files in `../oops-reader-app`:**
- Modify: `lib/presentation/pages/profile/login_page.dart`
- Modify: `lib/presentation/pages/profile/profile_page.dart`
- Create: `lib/presentation/pages/profile/account_settings_page.dart`
- Test: `test/account/login_page_test.dart`

- [ ] **Step 1: Write widget tests**

Create `test/account/login_page_test.dart`:

```dart
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:myreader/presentation/pages/profile/login_page.dart';

void main() {
  testWidgets('login page shows email password and register toggle', (tester) async {
    await tester.pumpWidget(const ProviderScope(child: MaterialApp(home: LoginPage())));

    expect(find.byType(TextField), findsAtLeastNWidgets(2));
    expect(find.textContaining('邮箱'), findsWidgets);
    expect(find.textContaining('密码'), findsWidgets);
    expect(find.textContaining('注册'), findsWidgets);
  });
}
```

- [ ] **Step 2: Implement login page**

Modify `login_page.dart`:

- Show email field.
- Show password field.
- Toggle login/register mode.
- In register mode show nickname field.
- Show forgot-password action.
- Call `accountRepositoryProvider.login` or `register`.
- On success, invalidate `accountSessionProvider` and pop page.
- On failure, show `SnackBar` with user-friendly message.

- [ ] **Step 3: Implement profile account header**

Modify `profile_page.dart`:

- Watch `accountSessionProvider`.
- If logged out, keep login entry and local reading copy.
- If logged in, show nickname, email, and account type text.
- Add navigation to `AccountSettingsPage`.

- [ ] **Step 4: Implement account settings page**

Create `account_settings_page.dart`:

- Logout button calls `accountRepositoryProvider.logout`.
- Password reset entry asks for email and calls `requestPasswordReset`.
- Cloud backup and restore entries can navigate to backup UI added in Task 12.

- [ ] **Step 5: Run widget test**

Run:

```bash
scripts/flutter-local.sh test test/account/login_page_test.dart
```

Expected: PASS.

- [ ] **Step 6: Run analyzer**

Run:

```bash
scripts/flutter-local.sh analyze
```

Expected: no new analyzer errors.

- [ ] **Step 7: Commit in app repo**

```bash
git add lib/presentation/pages/profile/login_page.dart lib/presentation/pages/profile/profile_page.dart lib/presentation/pages/profile/account_settings_page.dart test/account/login_page_test.dart
git commit -m "feat: add account login and profile UI"
```

---

## Task 11: App Reading Backup Snapshot Service

**Files in `../oops-reader-app`:**
- Create: `lib/data/services/reading_backup_snapshot_service.dart`
- Test: `test/backup/reading_backup_snapshot_service_test.dart`

- [ ] **Step 1: Write snapshot tests**

Create `test/backup/reading_backup_snapshot_service_test.dart` with tests:

```dart
void main() {
  test('snapshot includes top-level reading data keys', () async {
    final service = ReadingBackupSnapshotService(
      bookExporter: () async => [{'id': 1, 'title': 'Book'}],
      progressExporter: () async => [{'book_id': 1, 'percentage': 0.5}],
      noteExporter: () async => [{'id': 7, 'book_id': 1, 'note_text': 'Note'}],
      preferenceExporter: () async => {'theme': 'dark'},
      importer: (_) async {},
    );

    final snapshot = await service.exportSnapshot();

    expect(snapshot.payload.keys, containsAll(['bookshelf', 'reading_progress', 'notes', 'preferences']));
  });

  test('summary counts books progress notes and preferences', () async {
    final service = ReadingBackupSnapshotService(
      bookExporter: () async => [{'id': 1}, {'id': 2}],
      progressExporter: () async => [{'book_id': 1}],
      noteExporter: () async => [{'id': 1}, {'id': 2}, {'id': 3}],
      preferenceExporter: () async => {'theme': 'dark'},
      importer: (_) async {},
    );

    final snapshot = await service.exportSnapshot();

    expect(snapshot.bookCount, 2);
    expect(snapshot.progressCount, 1);
    expect(snapshot.noteCount, 3);
    expect(snapshot.preferenceCount, 1);
  });
}
```

Use fake repository implementations for books, reading progress, notes, and preferences instead of touching a real SQLite file.

- [ ] **Step 2: Implement snapshot service**

Create `lib/data/services/reading_backup_snapshot_service.dart` with:

```dart
typedef JsonListExporter = Future<List<Map<String, dynamic>>> Function();
typedef JsonMapExporter = Future<Map<String, dynamic>> Function();
typedef SnapshotImporter = Future<void> Function(Map<String, dynamic> payload);

class ReadingBackupSnapshot {
  final int schemaVersion;
  final Map<String, dynamic> payload;
  final int bookCount;
  final int noteCount;
  final int progressCount;
  final int preferenceCount;

  const ReadingBackupSnapshot({
    required this.schemaVersion,
    required this.payload,
    required this.bookCount,
    required this.noteCount,
    required this.progressCount,
    required this.preferenceCount,
  });
}

class ReadingBackupSnapshotService {
  final JsonListExporter bookExporter;
  final JsonListExporter progressExporter;
  final JsonListExporter noteExporter;
  final JsonMapExporter preferenceExporter;
  final SnapshotImporter importer;

  const ReadingBackupSnapshotService({
    required this.bookExporter,
    required this.progressExporter,
    required this.noteExporter,
    required this.preferenceExporter,
    required this.importer,
  });

  Future<ReadingBackupSnapshot> exportSnapshot();
  Future<void> importSnapshot(Map<String, dynamic> payload);
}
```

Use existing domain repositories from `repository_providers.dart` or inject them in the constructor. Export keys:

- `bookshelf`
- `reading_progress`
- `notes`
- `preferences`

- [ ] **Step 3: Run tests**

Run:

```bash
scripts/flutter-local.sh test test/backup/reading_backup_snapshot_service_test.dart
```

Expected: PASS.

- [ ] **Step 4: Commit in app repo**

```bash
git add lib/data/services/reading_backup_snapshot_service.dart test/backup/reading_backup_snapshot_service_test.dart
git commit -m "feat: add reading backup snapshot service"
```

---

## Task 12: App Backup API and UI

**Files in `../oops-reader-app`:**
- Modify: `lib/data/services/account_api_client.dart`
- Modify: `lib/core/providers/account_providers.dart`
- Create: `lib/presentation/pages/profile/cloud_backup_page.dart`
- Modify: `lib/presentation/pages/profile/account_settings_page.dart`
- Test: `test/backup/cloud_backup_flow_test.dart`

- [ ] **Step 1: Write backup flow tests**

Create `test/backup/cloud_backup_flow_test.dart` with tests:

```dart
AccountRepository fakeLoggedInAccountRepository() {
  return FakeAccountRepository(
    session: AccountSession(
      user: AccountUser(
        id: '1',
        email: 'reader@example.com',
        nickname: 'Reader',
        avatarUrl: '',
        accountStatus: 'active',
        accountType: 'normal',
      ),
      accessToken: 'access-token',
      refreshToken: 'refresh-token',
    ),
  );
}

ReadingBackupSnapshotService fakeSnapshotService() {
  return ReadingBackupSnapshotService(
    bookExporter: () async => [{'id': 1}],
    progressExporter: () async => [{'book_id': 1}],
    noteExporter: () async => [{'id': 1}],
    preferenceExporter: () async => {'theme': 'dark'},
    importer: (_) async {},
  );
}

void main() {
  test('create_if_empty conflict is exposed as cloud data exists', () async {
    final notifier = CloudBackupNotifier(
      accountRepository: fakeLoggedInAccountRepository(),
      snapshotService: fakeSnapshotService(),
      apiClient: AccountApiClient(
        baseUrl: Uri.parse('http://example.test/v1/'),
        transport: (request) async => const AccountResponse(
          statusCode: 409,
          body: '{"error":"cloud backup already exists","code":"cloud_data_exists","data":{"summary":{"exists":true,"book_count":3,"note_count":1,"progress_count":3,"preference_count":1}}}',
        ),
      ),
    );

    await notifier.uploadCreateIfEmpty();

    expect(notifier.state.conflictSummary?.exists, isTrue);
    expect(notifier.state.errorCode, 'cloud_data_exists');
  });

  test('overwrite sends overwrite mode', () async {
    late String requestBody;
    final notifier = CloudBackupNotifier(
      accountRepository: fakeLoggedInAccountRepository(),
      snapshotService: fakeSnapshotService(),
      apiClient: AccountApiClient(
        baseUrl: Uri.parse('http://example.test/v1/'),
        transport: (request) async {
          requestBody = utf8.decode(request.bodyBytes!);
          return const AccountResponse(statusCode: 200, body: '{"data":{"id":"1"}}');
        },
      ),
    );

    await notifier.uploadOverwrite();

    expect(requestBody, contains('"mode":"overwrite"'));
  });
}
```

Use fake transport responses for `409` and `200`.

- [ ] **Step 2: Add backup API methods**

Add to `AccountApiClient`:

```dart
Future<Map<String, dynamic>> getBackupSummary(String accessToken);
Future<Map<String, dynamic>> uploadBackup({
  required String accessToken,
  required Map<String, dynamic> body,
});
Future<Map<String, dynamic>> downloadBackup(String accessToken);
```

- [ ] **Step 3: Add backup provider/controller**

In `account_providers.dart`, add a `CloudBackupNotifier` that:

- Loads summary.
- Exports local snapshot.
- Uploads with `mode: create_if_empty`.
- Converts `409 cloud_data_exists` into a state that UI can render.
- Uploads with `mode: overwrite` only after explicit user action.
- Downloads and imports snapshot only after explicit user action.

- [ ] **Step 4: Add cloud backup page**

Create `cloud_backup_page.dart`:

- Shows cloud summary.
- Shows backup button.
- Shows restore button when cloud exists.
- On conflict, show dialog with overwrite and cancel.
- On restore, show confirmation dialog.

- [ ] **Step 5: Wire settings page**

Modify `account_settings_page.dart` to open `CloudBackupPage`.

- [ ] **Step 6: Run tests and analyzer**

Run:

```bash
scripts/flutter-local.sh test test/backup/cloud_backup_flow_test.dart
scripts/flutter-local.sh analyze
```

Expected: PASS and no new analyzer errors.

- [ ] **Step 7: Commit in app repo**

```bash
git add lib/data/services/account_api_client.dart lib/core/providers/account_providers.dart lib/presentation/pages/profile/cloud_backup_page.dart lib/presentation/pages/profile/account_settings_page.dart test/backup/cloud_backup_flow_test.dart
git commit -m "feat: add cloud backup UI"
```

---

## Task 13: Remove Hidden Community Guest Token Flow

**Files in `../oops-reader-app`:**
- Modify: `lib/data/services/community_api_client.dart`
- Modify: `lib/core/providers/community_providers.dart`
- Modify: `lib/presentation/widgets/guest_action_guard.dart`
- Test: `test/account/community_auth_test.dart`

- [ ] **Step 1: Write community auth tests**

Create `test/account/community_auth_test.dart`:

```dart
class AccountRequiredException implements Exception {
  const AccountRequiredException();
}

Ref fakeRefWithNoAccountSession() {
  return FakeRef(accountSession: null);
}

void main() {
  test('authorized community request uses account bearer token', () async {
    late Map<String, String> headers;
    final client = CommunityApiClient(
      baseUrl: Uri.parse('http://example.test/v1/community/'),
      transport: (request) async {
        headers = request.headers;
        return const CommunityResponse(statusCode: 201, body: '{"data":{"id":"1","board_id":"general","user_id":"1","title":"Title","content":"Body","optional_book_id":"","status":"active","comment_count":0,"reaction_counts":{},"comments":[]}}');
      },
    );

    await client.createThread(
      title: 'Title',
      content: 'Body',
      bearerToken: 'account-token',
    );

    expect(headers[HttpHeaders.authorizationHeader], 'Bearer account-token');
  });

  test('authorized community request fails when logged out', () async {
    final notifier = CommunityNotifier(fakeRefWithNoAccountSession());

    expect(
      () => notifier.createGeneralThread(title: 'Title', content: 'Body'),
      throwsA(isA<AccountRequiredException>()),
    );
  });
}
```

Use fake account repository and fake community transport.

- [ ] **Step 2: Modify community API client**

Remove these persisted keys from active behavior:

- `community_access_token`
- `community_refresh_token`
- `community_device_id`

Change authorized methods to accept `String bearerToken` from caller:

```dart
Future<CommunityThread> createThread({
  required String title,
  required String content,
  required String bearerToken,
});
```

Apply the same pattern to comments, likes, and "my threads".

- [ ] **Step 3: Modify community provider**

In `community_providers.dart`, read account session before authorized calls. If no session exists, throw a typed exception or return state error that `guest_action_guard.dart` can convert into login UI.

- [ ] **Step 4: Keep public community reads unauthenticated**

`fetchGeneralThreads` and `fetchThread` remain unauthenticated.

- [ ] **Step 5: Run tests**

Run:

```bash
scripts/flutter-local.sh test test/account/community_auth_test.dart
scripts/flutter-local.sh analyze
```

Expected: PASS and no new analyzer errors.

- [ ] **Step 6: Commit in app repo**

```bash
git add lib/data/services/community_api_client.dart lib/core/providers/community_providers.dart lib/presentation/widgets/guest_action_guard.dart test/account/community_auth_test.dart
git commit -m "refactor: use account session for community auth"
```

---

## Task 14: API Documentation and End-to-End Verification

**Files:**
- Modify backend: `docs/api.md`
- Modify backend: `docs/api-quick.md`
- Modify backend: `test_api.sh`
- Modify app: `env/dart_defines.example.json`

- [ ] **Step 1: Update backend API docs**

Document endpoints:

- `POST /v1/auth/register`
- `POST /v1/auth/login`
- `POST /v1/auth/refresh`
- `POST /v1/auth/logout`
- `POST /v1/auth/password/reset-request`
- `POST /v1/auth/password/reset-confirm`
- `GET /v1/users/me`
- `PATCH /v1/users/me`
- `GET /v1/account/entitlements`
- `GET /v1/backup/reading-data/summary`
- `POST /v1/backup/reading-data`
- `GET /v1/backup/reading-data`

- [ ] **Step 2: Update test API script**

Add curl checks to `test_api.sh`:

```bash
curl -X POST "$BASE_URL/v1/auth/register" \
  -H "Content-Type: application/json" \
  -d '{"email":"reader-test@example.com","password":"password12345","nickname":"Reader","device_id":"test-device","platform":"linux"}'
```

Parse token with `jq` if available; otherwise print manual test instructions.

- [ ] **Step 3: Update app env example**

Add `ACCOUNT_BASE_URL` if account API base URL should differ from `COMMUNITY_BASE_URL`. If the implementation reuses root API base URL derivation, document that instead of adding another define.

- [ ] **Step 4: Run backend verification**

Run from backend repo:

```bash
go test ./...
git diff --check
```

Expected: PASS and no whitespace errors.

- [ ] **Step 5: Run app verification**

Run from app repo:

```bash
scripts/flutter-local.sh test
scripts/flutter-local.sh analyze
git diff --check
```

Expected: PASS and no whitespace errors.

- [ ] **Step 6: Commit docs and verification helpers**

Commit backend docs:

```bash
git add docs/api.md docs/api-quick.md test_api.sh
git commit -m "docs: document account and backup APIs"
```

Commit app env/docs:

```bash
git add env/dart_defines.example.json
git commit -m "docs: document account API app config"
```

---

## Final Verification Checklist

- [ ] Backend `go test ./...` passes.
- [ ] App `scripts/flutter-local.sh test` passes.
- [ ] App `scripts/flutter-local.sh analyze` passes.
- [ ] Register, login, refresh, logout, and password reset request work against a local backend.
- [ ] `GET /v1/users/me` uses the same account ID as community posting.
- [ ] Entitlement API returns `backup.reading_data` for normal users.
- [ ] Backup upload succeeds when no backup exists.
- [ ] Backup upload returns `409 cloud_data_exists` when backup exists and mode is `create_if_empty`.
- [ ] Backup overwrite requires explicit user action.
- [ ] Restore requires explicit user confirmation.
- [ ] Logged-out local reading still works.
- [ ] No UI claims VIP is available in version one.
