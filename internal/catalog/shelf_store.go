package catalog

import "context"

// ShelfStore defines persistence operations for user catalog bookshelves.
type ShelfStore interface {
	GetShelf(ctx context.Context, userID uint64, bookKey string) (ShelfState, error)
	UpsertShelf(ctx context.Context, userID uint64, bookKey string, localBookID string) (ShelfState, error)
}
