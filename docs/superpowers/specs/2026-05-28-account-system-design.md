# Oops Reader Account System Design

Date: 2026-05-28

## 1. Background

Oops Reader currently has two codebases:

- `oops-reader-app`: Flutter app, local SQLite reading data, profile page, login placeholder, and guarded account-only actions.
- `oops-reader-backend`: Go/Gin backend, MariaDB migrations for users and reading data, in-memory guest identity MVP, and many placeholder authenticated APIs.

The existing backend already has `users`, `user_sessions`, and reading data table migrations, but the active identity service stores users and refresh tokens in memory. The app also stores community guest tokens inside `CommunityApiClient`, while the visible login page is still a placeholder.

This design upgrades the existing MVP into a real account system. It covers app features, backend services, API contracts, database design, and first-version user data management.

## 2. Goals

1. Support email/password account registration and login.
2. Keep local reading usable without login.
3. Use one account identity across the whole app. There is no separate community identity.
4. Persist users, sessions, and password reset flows in MariaDB.
5. Support first-version cloud backup and restore for reading data.
6. Leave clean extension points for future VIP/subscription features without enabling VIP in version one.
7. Replace scattered token handling with a single account session layer in the app.

## 3. Non-Goals

1. Version one will not enable VIP business rules, subscriptions, payments, or paywall UI.
2. Version one will not implement full multi-device realtime sync or automatic conflict merge.
3. Version one will not require email verification before login.
4. Version one will not build a separate admin console.
5. Version one will not support phone number login, SMS verification, Apple login, WeChat login, or Google login, but the data model must allow them later.

## 4. Recommended Approach

Use a progressive account system:

1. Keep logged-out local app usage.
2. Launch email/password registration, login, logout, token refresh, and forgot-password email reset.
3. Persist identity and sessions in MariaDB.
4. Add account-scoped cloud backup and restore for local reading data.
5. Expose a simple entitlement API that returns normal-user capabilities now and can return VIP capabilities later.

This approach fits the current codebase. It avoids a large first release that mixes identity, billing, full sync, and conflict resolution. It still makes the account system useful immediately because users can back up and restore reading data.

## 5. App Design

### 5.1 Account State

Add a unified app account layer:

- `AccountSession`: current user, access token, refresh token, expiration metadata, and device ID.
- `AccountRepository`: login, register, logout, refresh token, load current user, request password reset, confirm password reset, and load entitlements.
- `AccountApiClient`: authenticated HTTP client wrapper that injects access tokens and retries once after refresh when a request returns `401`.
- Riverpod providers for session state, current user, entitlement state, and backup state.

The existing `user_signed_in` preference should be replaced by session-derived state. The account layer should be the only source of truth for authenticated requests.

### 5.2 Login and Registration

The current login placeholder becomes a real account flow:

- Login with email and password.
- Register with email, password, and optional nickname.
- Register success logs the user in immediately.
- Forgot-password entry sends reset email.
- Reset confirmation can be implemented either in app or via a web deep link. The first implementation should prefer an email link backed by a short-lived reset token.

Email verification is not required in version one. The schema still stores `email_verified_at` for future enforcement.

Future phone and third-party login should be added through `phone` and `user_auth_providers` rather than by creating another user identity model.

### 5.3 Profile Page

The profile page should show account state:

- Logged out: login/register entry and copy explaining that local reading remains available.
- Logged in: nickname, email, avatar, account status, and a reserved account type/entitlement area.
- Settings entries: account settings, cloud backup, cloud restore, logout, and password reset.

VIP should not be shown as a paid product in version one. A small internal account type field can exist in returned data, but the user-facing UI should avoid promising VIP until the business rules exist.

### 5.4 Authenticated API Calls

All authenticated app features must use the account layer for tokens. The current community API behavior that creates a guest token internally should be refactored so it asks the account layer for credentials.

Private APIs must never create hidden app-specific identities. They should use the same account `user_id`.

### 5.5 Local Reading Still Works

Users can import books, read, create notes, and track progress while logged out. Local SQLite remains the primary store for reading interaction. Login adds cloud backup and future cross-device features; it is not required for basic reading.

## 6. Backend Design

### 6.1 Modules

Keep the modular monolith structure and upgrade identity into database-backed services:

- `internal/identity`: account registration, login, profile, account status, and token issuing.
- `internal/session`: refresh token hashing, session persistence, rotation, logout, and device metadata. This can initially be a subpackage under identity if that is simpler.
- `internal/password`: password hashing and password reset tokens. This can initially be a subpackage under identity.
- `internal/entitlement`: capability lookup. Version one returns normal-user entitlements only.
- `internal/backup`: account-scoped reading data backup and restore.
- `internal/transport/http/handlers`: request validation and response shaping.

The existing in-memory `identity.Service` should be replaced or wrapped by a store-backed implementation. Tests that cover the current guest lifecycle should be migrated to the persistent service.

### 6.2 Token Model

Use short-lived access tokens and long-lived refresh tokens:

- Access token: signed token, expires quickly, sent as `Authorization: Bearer <token>`.
- Refresh token: opaque random token, stored by the app, submitted only to refresh/logout endpoints.
- Server stores only `refresh_token_hash`.
- Refresh rotates the refresh token and revokes the old one.
- Logout revokes the current refresh session.

The current custom HMAC token can be kept for the first implementation if tests cover it, but using a standard JWT library already present in the project plan is preferable before production.

### 6.3 Password Model

Use `bcrypt` or `argon2id`. Do not store raw passwords or reversible encrypted passwords.

Password reset flow:

1. User submits email.
2. Backend always returns success-shaped response to avoid account enumeration.
3. If the email exists, backend creates a short-lived reset token and sends email.
4. Backend stores only the reset token hash.
5. Reset confirmation verifies token, expiry, and unused state, then updates password and marks token used.
6. Existing sessions should be revoked after a successful password reset.

### 6.4 Account Status

Support these account statuses:

- `active`: normal account.
- `frozen`: login and private data APIs are denied.
- `deactivated`: account is no longer usable. Data retention and physical cleanup can be handled by backend maintenance later.

Version one app does not need a full self-service account deletion flow. Backend status fields must exist so support or admin scripts can disable accounts safely.

## 7. API Design

All responses should keep the existing `{"data": ...}` style where practical.

### 7.1 Auth APIs

#### `POST /v1/auth/register`

Request:

```json
{
  "email": "reader@example.com",
  "password": "strong-password",
  "nickname": "Reader",
  "device_id": "ios-device-id",
  "device_name": "iPhone",
  "platform": "ios"
}
```

Response `201`:

```json
{
  "data": {
    "user": {},
    "access_token": "...",
    "refresh_token": "...",
    "token_type": "Bearer",
    "entitlements": []
  }
}
```

Duplicate email returns `409`.

#### `POST /v1/auth/login`

Request:

```json
{
  "email": "reader@example.com",
  "password": "strong-password",
  "device_id": "ios-device-id",
  "device_name": "iPhone",
  "platform": "ios"
}
```

Invalid credentials return `401`. Frozen or deactivated accounts return `403`.

#### `POST /v1/auth/refresh`

Request:

```json
{
  "refresh_token": "..."
}
```

Response returns a new access token and new refresh token. The previous refresh token becomes invalid.

#### `POST /v1/auth/logout`

Requires an access token and should include the current refresh token in the request body when available. The backend revokes the matching refresh session. The app should clear local session state after success or after a best-effort failure.

#### `POST /v1/auth/password/reset-request`

Request:

```json
{
  "email": "reader@example.com"
}
```

Always returns `200` with a generic message.

#### `POST /v1/auth/password/reset-confirm`

Request:

```json
{
  "token": "...",
  "new_password": "new-strong-password"
}
```

Expired, invalid, or used tokens return `400`.

### 7.2 User APIs

#### `GET /v1/users/me`

Returns current account profile.

#### `PATCH /v1/users/me`

Allows updating nickname and avatar URL. Email change should be deferred until verification exists.

### 7.3 Entitlement API

#### `GET /v1/account/entitlements`

Version one returns normal-user capabilities:

```json
{
  "data": {
    "account_type": "normal",
    "entitlements": [
      {
        "key": "backup.reading_data",
        "status": "active",
        "expires_at": null
      }
    ]
  }
}
```

Future VIP should be represented as capability keys, not hard-coded checks such as `account_type == "vip"`.

### 7.4 Backup APIs

#### `GET /v1/backup/reading-data/summary`

Returns whether cloud backup exists and a summary:

```json
{
  "data": {
    "exists": true,
    "backup_id": "bkp_123",
    "book_count": 12,
    "note_count": 36,
    "progress_count": 12,
    "preference_count": 1,
    "updated_at": "2026-05-28T10:00:00Z",
    "device_name": "iPhone"
  }
}
```

#### `POST /v1/backup/reading-data`

Uploads a full local snapshot.

Request:

```json
{
  "mode": "create_if_empty",
  "device_id": "ios-device-id",
  "device_name": "iPhone",
  "schema_version": 1,
  "payload": {
    "bookshelf": [],
    "reading_progress": [],
    "notes": [],
    "preferences": {}
  }
}
```

If cloud data already exists and `mode` is `create_if_empty`, return `409`:

```json
{
  "error": "cloud backup already exists",
  "code": "cloud_data_exists",
  "data": {
    "summary": {}
  }
}
```

If the user explicitly chooses overwrite, App sends `mode: "overwrite"`.

#### `GET /v1/backup/reading-data`

Downloads the latest cloud snapshot for restore.

## 8. Database Design

### 8.1 Existing Tables to Adjust

The current `users` table should be extended or migrated:

- Keep `email`, `phone`, `password_hash`, `nickname`, `avatar_url`, `last_login_at`, timestamps.
- Rename or reinterpret `status` into account status values. If keeping numeric status, define constants in code and docs.
- Add `account_type` with default `normal`.
- Add `email_verified_at`.

Recommended `users` fields:

| Field | Purpose |
|---|---|
| `id` | Primary user ID |
| `email` | Unique email |
| `phone` | Future phone login |
| `password_hash` | Password hash |
| `nickname` | Display name |
| `avatar_url` | Avatar |
| `account_status` | `active/frozen/deactivated` |
| `account_type` | `normal`, future `vip` |
| `email_verified_at` | Future email verification |
| `last_login_at` | Last successful login |
| `created_at` / `updated_at` | Audit timestamps |

### 8.2 `user_sessions`

Extend the existing table:

| Field | Purpose |
|---|---|
| `user_id` | Account owner |
| `device_id` | App-generated stable device ID |
| `device_name` | Human-readable device name |
| `platform` | `ios/android/linux/web` |
| `refresh_token_hash` | Hash only |
| `expires_at` | Refresh session expiry |
| `revoked_at` | Logout or rotation revocation |
| `last_active_at` | Recent activity |
| `created_at` / `updated_at` | Audit timestamps |

### 8.3 `password_reset_tokens`

New table:

| Field | Purpose |
|---|---|
| `id` | Primary key |
| `user_id` | Account owner |
| `email` | Email used for reset |
| `token_hash` | Hash only |
| `expires_at` | Short expiry |
| `used_at` | One-time use marker |
| `created_at` | Creation time |

### 8.4 `account_entitlements`

New table for future VIP:

| Field | Purpose |
|---|---|
| `id` | Primary key |
| `user_id` | Account owner |
| `entitlement_key` | Capability key, such as `sync.multi_device` |
| `status` | `active/inactive/revoked` |
| `source` | `system/purchase/admin/promo` |
| `starts_at` | Optional start time |
| `expires_at` | Optional expiry |
| `metadata` | JSON extension |
| `created_at` / `updated_at` | Audit timestamps |

Version one may return default entitlements without inserting rows for every normal user. The table is for explicit grants and future VIP/subscription integration.

### 8.5 `user_data_backups`

New table for first-version cloud backup:

| Field | Purpose |
|---|---|
| `id` | Backup ID |
| `user_id` | Account owner |
| `schema_version` | Snapshot schema version |
| `payload` | JSON snapshot |
| `book_count` | Summary count |
| `note_count` | Summary count |
| `progress_count` | Summary count |
| `preference_count` | Summary count |
| `source_device_id` | Uploading device |
| `source_device_name` | Uploading device name |
| `created_at` / `updated_at` | Audit timestamps |

Use one active latest backup per user in version one. This can be implemented by upsert on `user_id` or by keeping versions and marking latest. Keeping versions is safer for recovery, but the API should expose only the latest snapshot first.

## 9. Reading Data Backup Design

Version one uses full snapshots instead of operation-level sync.

Snapshot content:

- Bookshelf records mapped from local books.
- Reading progress per book.
- Notes and highlights.
- Reader preferences and app preferences that are safe to sync.

The snapshot should include local IDs and stable client IDs where available. If an entity does not have a stable client ID, the app should generate one before upload. This prevents future sync work from losing object identity.

Cloud backup conflict rule:

1. If no cloud backup exists, upload succeeds.
2. If cloud backup exists and request mode is `create_if_empty`, backend returns `409 cloud_data_exists`.
3. If user chooses overwrite, app sends `mode: overwrite`.
4. No automatic merge is performed in version one.

Restore rule:

1. App shows cloud backup summary.
2. User confirms restore.
3. App creates a local safety backup before importing.
4. App imports the snapshot into local SQLite.
5. App refreshes local providers after import.

## 10. VIP Extension Design

VIP is not enabled in version one. The system should still avoid future dead ends:

- `users.account_type` defaults to `normal`.
- `account_entitlements` models concrete capabilities.
- App consumes entitlements through provider state.
- Backend business code asks an entitlement service whether a user has a capability.
- Future subscription integration writes entitlement grants or updates `account_type`, but user-facing feature checks rely on capability keys.

Possible future entitlement keys:

- `backup.reading_data`
- `sync.multi_device`
- `cloud.storage.extended`
- `catalog.premium`
- `tts.premium_voice`

## 11. Data Flows

### 11.1 Register

1. App submits email, password, nickname, and device metadata.
2. Backend validates input and unique email.
3. Backend hashes password and creates user.
4. Backend creates default preferences if needed.
5. Backend creates session and returns tokens.
6. App stores session and loads entitlements.
7. App prompts for first cloud backup if local reading data exists.

### 11.2 Login

1. App submits email, password, and device metadata.
2. Backend validates credentials and account status.
3. Backend rotates or creates session.
4. Backend returns tokens, profile, and entitlements.
5. App stores session and updates profile page.

### 11.3 Refresh

1. App receives `401` or detects access token expiry.
2. App submits refresh token.
3. Backend validates token hash, session status, and expiry.
4. Backend revokes old refresh token and issues a new pair.
5. App retries the original request once.

### 11.4 Backup

1. App calls backup summary.
2. App builds local SQLite snapshot.
3. If cloud is empty, app uploads `create_if_empty`.
4. If cloud exists, app shows overwrite/cancel.
5. Overwrite uploads `mode: overwrite`.

### 11.5 Restore

1. User opens cloud restore.
2. App loads backup summary.
3. User confirms.
4. App downloads latest snapshot.
5. App creates local safety backup.
6. App imports snapshot and refreshes local state.

## 12. Error Handling

Recommended status codes:

| Status | Scenario |
|---|---|
| `400` | Invalid input, weak password, invalid reset token |
| `401` | Invalid credentials, missing/invalid access token |
| `403` | Frozen or deactivated account |
| `404` | Resource not found |
| `409` | Duplicate email or cloud backup conflict |
| `429` | Login/register/reset rate limit |
| `500` | Unexpected server error |

App handling:

- `401` on private APIs triggers one refresh attempt.
- Refresh failure clears session and returns to logged-out state.
- `409 cloud_data_exists` opens overwrite/cancel UI.
- Network failures show retry affordances and never delete local reading data.

## 13. Security Requirements

1. Password hashes use `bcrypt` or `argon2id`.
2. Refresh tokens and reset tokens are stored as hashes only.
3. Refresh tokens rotate on every refresh.
4. Password reset tokens are short-lived and one-time use.
5. Password reset success revokes existing sessions.
6. Login, register, and password reset endpoints have basic rate limiting.
7. Private data queries are always scoped by authenticated `user_id`.
8. Account status is checked before issuing tokens and before serving private APIs.
9. Logs must not include raw passwords, access tokens, refresh tokens, or reset tokens.

## 14. Testing Plan

Backend tests:

- Register succeeds and stores hashed password.
- Duplicate email returns `409`.
- Login succeeds with valid password and fails with invalid password.
- Frozen account cannot login or refresh.
- Refresh rotates token and rejects the old refresh token.
- Logout revokes session.
- Password reset request does not reveal whether email exists.
- Password reset confirm rejects expired or used tokens.
- Backup upload succeeds when cloud is empty.
- Backup upload returns `409 cloud_data_exists` when cloud data exists and mode is `create_if_empty`.
- Backup overwrite replaces latest summary.

App tests:

- Session provider reports logged out when no token exists.
- Login stores session and updates profile state.
- Authenticated request retries once after token refresh.
- Refresh failure clears session.
- Logged-out reading remains available.
- Backup prompt appears after login when local reading data exists.
- Cloud conflict shows overwrite/cancel instead of merging.
- Restore imports snapshot only after user confirmation.

Migration tests:

- Local SQLite snapshot export includes books, progress, notes, and preferences.
- Restored SQLite has matching record counts and key fields.
- Snapshot schema version is accepted by backend.

## 15. Implementation Order

1. Backend database migrations for account fields, sessions, reset tokens, entitlements, and backups.
2. Backend persistent identity service and tests.
3. Backend auth endpoints and middleware update.
4. Backend backup endpoints and tests.
5. App account models, repository, API client, and providers.
6. App login/register/forgot-password/profile UI.
7. App backup summary/upload/restore UI and local snapshot export/import.
8. Replace feature-specific token storage with account session provider.
9. End-to-end verification against local backend.

## 16. Open Decisions Chosen for Version One

These choices are intentionally fixed for the first implementation:

- Login method: email/password first, with future provider bindings.
- VIP: not enabled, only extensibility is built.
- Data management: cloud backup/restore first, no multi-device automatic merge.
- Existing local data: new accounts can back up local data; if cloud data exists, user may overwrite or cancel.
- Password recovery: forgot-password email reset is included.
- Account deletion: not exposed in app in version one; backend supports account status for frozen/deactivated accounts.

## 17. Acceptance Criteria

The design is implemented when:

1. A user can register, login, refresh session, logout, and request password reset.
2. User sessions survive app restart through stored refresh tokens.
3. Authenticated APIs use one account identity across the app.
4. Backend users and sessions persist after server restart.
5. Logged-out local reading still works.
6. A logged-in user can back up local reading data to cloud.
7. Existing cloud backup conflict returns a clear `409` and the app does not auto-merge.
8. A logged-in user can restore the latest cloud backup after confirmation.
9. Entitlement API exists and returns normal-user capabilities.
10. No version-one UI claims VIP functionality is available.
