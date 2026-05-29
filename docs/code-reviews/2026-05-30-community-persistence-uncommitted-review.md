# Community Persistence Uncommitted Changes Review

Date: 2026-05-30

## Scope

Reviewed current staged, unstaged, and untracked changes related to community persistence:

- `cmd/api/main.go`
- `internal/community/service.go`
- `internal/community/service_test.go`
- `internal/community/local_storage.go`
- `internal/community/memory_store.go`
- `internal/community/mysql_store.go`
- `internal/community/store.go`
- `internal/platform/config/config.go`
- `internal/transport/http/handlers/community.go`
- `migrations/009_init_community.sql`

CodeRabbit was also run against uncommitted changes. Its narrowed review raised one issue about `community.NewServiceWithDB(db)` depending on a concrete DB constructor. I did not treat that as blocking because this repository already uses the same constructor pattern in `catalog.NewServiceWithDB`.

## Findings

### 1. [P1] Reactions can target nonexistent comments

File: `internal/community/service.go`

Lines: `validateTarget`, comment branch

The `TargetTypeComment` branch currently accepts any comment ID:

```go
case TargetTypeComment:
    // We don't have GetComment, so check via list
    // For now, we accept any target and let the store handle FK constraints
```

This is incorrect because `community_reactions.target_id` is polymorphic and has no foreign key to `community_comments`. A request can create reactions for nonexistent comments and leave bogus reaction rows and counts.

Recommended fix:

- Add a Store method such as `GetComment(ctx, id) (Comment, error)`.
- Implement it in both `MemoryStore` and `MySQLStore`.
- In `validateTarget`, return `ErrCommentNotFound` unless the comment exists and has `StatusActive`.

### 2. [P2] Local image URLs are not served

File: `cmd/api/main.go`

Lines: community local storage setup

The default config sets:

```go
community.images.local_path = "./data/community/images"
community.images.public_url = "/community/images"
```

Uploaded attachments return URLs under `/community/images/...`, but the router does not serve `cfg.Community.Images.LocalPath` at that path. In a default local deployment, uploaded image URLs will return `404`.

Recommended fix:

- Register a static route for local storage when using local storage, or
- Require `public_url` to be an externally served absolute URL and document the deployment requirement.

If local serving is chosen, avoid exposing unintended paths and ensure the URL prefix maps only to the attachment directory.

### 3. [P2] Deleted or hidden threads remain readable through MySQL

File: `internal/community/mysql_store.go`

Function: `GetThread`

`MySQLStore.GetThread` fetches by ID without filtering status:

```sql
WHERE id = ?
```

`MemoryStore.GetThread` returns `ErrThreadNotFound` for non-active threads. Once moderation or soft delete sets a thread to `hidden`, `locked`, or `deleted`, the MySQL-backed detail endpoint will still expose that record.

Recommended fix:

```sql
WHERE id = ? AND status = 'active'
```

Alternatively, enforce active status in `Service.GetThread` after reading. Filtering in SQL is simpler and consistent with `ListThreads`.

### 4. [P2] Upload size check runs too late

File: `internal/transport/http/handlers/community.go`

Function: `UploadAttachment`

The handler calls `c.Request.FormFile("file")` before checking `header.Size`, and `LocalStorage.Save` then reads the full file into memory with `io.ReadAll`. Large uploads can consume memory or temp disk before being rejected.

Recommended fix:

- Wrap the request body with `http.MaxBytesReader` before multipart parsing.
- Also enforce the limit while reading in storage, for example with a limited reader that errors if the actual payload exceeds 5 MB.
- Keep the existing `400` behavior for oversized uploads.

### 5. [P2] MemoryStore leaves partial writes on attachment errors

File: `internal/community/memory_store.go`

Functions: `CreateThread`, `CreateComment`

`CreateThread` inserts the thread before validating attachments. If an attachment is missing, owned by another user, or already bound, the method returns an error but leaves the new thread in memory.

`CreateComment` has the same issue and additionally increments the thread comment count before validating attachments.

Recommended fix:

- Validate all attachment IDs first.
- Prepare updated attachment values in local variables.
- Only after all validation succeeds, write the thread/comment, update counters, and update attachments.

This keeps MemoryStore behavior aligned with the transactional MySQL implementation.

## Verification

Attempted to run:

```bash
go test ./...
```

The command could not run because Go is not installed in this environment:

```text
/usr/bin/bash: line 1: go: command not found
```

`git diff --check` passed with no whitespace errors.
