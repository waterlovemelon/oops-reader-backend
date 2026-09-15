package catalog

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
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

const shelfColumns = `catalog_book_key, shelf_status, local_book_id, added_at, last_read_at`

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

// GetShelfItem returns the account's active shelf row for a book.
func (s *MySQLShelfStore) GetShelfItem(ctx context.Context, userID uint64, bookKey string) (ShelfItem, error) {
	item, err := scanShelfItem(s.db.QueryRowContext(ctx, `
SELECT `+shelfColumns+`
FROM user_catalog_bookshelves
WHERE user_id = ? AND catalog_book_key = ? AND deleted_at IS NULL`,
		userID, bookKey,
	))
	if err == sql.ErrNoRows {
		return ShelfItem{}, ErrShelfNotFound
	}
	if err != nil {
		return ShelfItem{}, fmt.Errorf("get shelf item: %w", err)
	}
	return item, nil
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

// ListShelves returns one page of the account's shelf, most recently added
// first, plus the account's total number of active shelf rows.
func (s *MySQLShelfStore) ListShelves(ctx context.Context, userID uint64, limit, offset int) ([]ShelfItem, int, error) {
	if limit < 1 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	var total int
	if err := s.db.QueryRowContext(ctx, `
SELECT COUNT(*) FROM user_catalog_bookshelves
WHERE user_id = ? AND deleted_at IS NULL`, userID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count shelf records: %w", err)
	}
	if total == 0 {
		return []ShelfItem{}, 0, nil
	}

	rows, err := s.db.QueryContext(ctx, `
SELECT `+shelfColumns+`
FROM user_catalog_bookshelves
WHERE user_id = ? AND deleted_at IS NULL
ORDER BY added_at DESC, id DESC
LIMIT ? OFFSET ?`, userID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list shelf records: %w", err)
	}
	defer rows.Close()

	items := []ShelfItem{}
	for rows.Next() {
		item, err := scanShelfItem(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan shelf record: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate shelf records: %w", err)
	}
	return items, total, nil
}

// UpdateShelf replaces the non-empty fields of an active shelf row and returns
// the stored state. A book that is not on the shelf reports ErrShelfNotFound.
func (s *MySQLShelfStore) UpdateShelf(ctx context.Context, userID uint64, bookKey, shelfStatus, localBookID string) (ShelfState, error) {
	assignments := make([]string, 0, 2)
	args := make([]any, 0, 4)
	if shelfStatus != "" {
		assignments = append(assignments, "shelf_status = ?")
		args = append(args, shelfStatus)
	}
	if localBookID != "" {
		assignments = append(assignments, "local_book_id = ?")
		args = append(args, localBookID)
	}
	if len(assignments) == 0 {
		return ShelfState{}, fmt.Errorf("update shelf: %w", ErrShelfNotFound)
	}

	args = append(args, userID, bookKey)
	if _, err := s.db.ExecContext(ctx, `
UPDATE user_catalog_bookshelves
SET `+strings.Join(assignments, ", ")+`, updated_at = NOW()
WHERE user_id = ? AND catalog_book_key = ? AND deleted_at IS NULL`, args...); err != nil {
		return ShelfState{}, fmt.Errorf("update shelf: %w", err)
	}

	// A missing row is reported from the stored state, not from the affected
	// row count: MySQL also reports 0 affected rows when the update rewrote
	// identical values, which is a success.
	state, err := s.GetShelf(ctx, userID, bookKey)
	if err != nil {
		return ShelfState{}, err
	}
	if !state.InLibrary {
		return ShelfState{}, ErrShelfNotFound
	}
	return state, nil
}

// DeleteShelf soft-deletes an active shelf row, so re-adding the book later
// restores it instead of inserting a second row.
func (s *MySQLShelfStore) DeleteShelf(ctx context.Context, userID uint64, bookKey string) error {
	result, err := s.db.ExecContext(ctx, `
UPDATE user_catalog_bookshelves
SET deleted_at = NOW(), updated_at = NOW()
WHERE user_id = ? AND catalog_book_key = ? AND deleted_at IS NULL`, userID, bookKey)
	if err != nil {
		return fmt.Errorf("delete shelf: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete shelf: %w", err)
	}
	if affected == 0 {
		return ErrShelfNotFound
	}
	return nil
}

type shelfScanner interface {
	Scan(dest ...any) error
}

func scanShelfItem(scanner shelfScanner) (ShelfItem, error) {
	var item ShelfItem
	var localBookID sql.NullString
	var lastReadAt sql.NullTime
	if err := scanner.Scan(&item.BookKey, &item.ShelfStatus, &localBookID, &item.AddedAt, &lastReadAt); err != nil {
		return ShelfItem{}, err
	}
	item.LocalBookID = localBookID.String
	item.LastReadAt = nullableTime(lastReadAt)
	return item, nil
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
