package catalog

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// MySQLShelfStore implements ShelfStore using MySQL.
type MySQLShelfStore struct {
	db *sql.DB
}

// NewMySQLShelfStore creates a new MySQLShelfStore.
func NewMySQLShelfStore(db *sql.DB) *MySQLShelfStore {
	return &MySQLShelfStore{db: db}
}

// GetShelf returns the shelf state for a user and catalog book.
// If no record exists or the record is soft-deleted, InLibrary is false.
func (s *MySQLShelfStore) GetShelf(ctx context.Context, userID uint64, bookKey string) (ShelfState, error) {
	var state ShelfState
	var shelfStatus sql.NullString
	var lastReadAt sql.NullTime
	err := s.db.QueryRowContext(ctx, `
SELECT catalog_book_key, shelf_status, last_read_at
FROM user_catalog_bookshelves
WHERE user_id = ? AND catalog_book_key = ? AND deleted_at IS NULL`,
		userID, bookKey,
	).Scan(&state.BookID, &shelfStatus, &lastReadAt)
	if err == sql.ErrNoRows {
		return ShelfState{InLibrary: false}, nil
	}
	if err != nil {
		return ShelfState{}, fmt.Errorf("get shelf: %w", err)
	}
	state.InLibrary = true
	state.ShelfStatus = shelfStatus.String
	state.LastReadAt = nullableTime(lastReadAt)
	return state, nil
}

// UpsertShelf creates or restores a shelf record.
// If the record exists and is soft-deleted, it restores it.
// If the record already exists and is active, it returns the current state.
func (s *MySQLShelfStore) UpsertShelf(ctx context.Context, userID uint64, bookKey string, localBookID string) (ShelfState, error) {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO user_catalog_bookshelves
    (user_id, catalog_book_key, shelf_status, local_book_id, added_at, deleted_at)
VALUES (?, ?, 'want_to_read', ?, NOW(), NULL)
ON DUPLICATE KEY UPDATE
    deleted_at = NULL,
    local_book_id = COALESCE(VALUES(local_book_id), local_book_id),
    updated_at = NOW()`,
		userID, bookKey, nullableString(localBookID),
	)
	if err != nil {
		return ShelfState{}, fmt.Errorf("upsert shelf: %w", err)
	}
	return s.GetShelf(ctx, userID, bookKey)
}

func nullableTime(t sql.NullTime) *time.Time {
	if !t.Valid {
		return nil
	}
	return &t.Time
}

func nullableString(value string) interface{} {
	if value == "" {
		return nil
	}
	return value
}
