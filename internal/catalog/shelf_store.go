package catalog

import (
	"context"
	"errors"
	"time"
)

// ShelfItem is one active row of a user's catalog bookshelf.
type ShelfItem struct {
	BookKey     string
	ShelfStatus string
	LocalBookID string
	AddedAt     time.Time
	LastReadAt  *time.Time
}

// ErrShelfNotFound reports that the account has no active shelf row for a book.
var ErrShelfNotFound = errors.New("shelf record not found")

// ShelfStore defines persistence operations for user catalog bookshelves.
// Every operation reads and writes only rows that are not soft-deleted.
type ShelfStore interface {
	GetShelf(ctx context.Context, userID uint64, bookKey string) (ShelfState, error)
	// GetShelfItem returns the row behind GetShelf in full; it reports
	// ErrShelfNotFound instead of an empty state, so a caller that must answer
	// with the stored row can tell "not on the shelf" from "on the shelf".
	GetShelfItem(ctx context.Context, userID uint64, bookKey string) (ShelfItem, error)
	UpsertShelf(ctx context.Context, userID uint64, bookKey string, localBookID string) (ShelfState, error)
	ListShelves(ctx context.Context, userID uint64, limit, offset int) ([]ShelfItem, int, error)
	UpdateShelf(ctx context.Context, userID uint64, bookKey, shelfStatus, localBookID string) (ShelfState, error)
	DeleteShelf(ctx context.Context, userID uint64, bookKey string) error
}
