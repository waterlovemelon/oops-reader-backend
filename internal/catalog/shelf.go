package catalog

import "time"

// ShelfStatus represents the user's reading status for a catalog book.
type ShelfStatus string

const ShelfStatusWantToRead ShelfStatus = "want_to_read"

// ShelfState represents the current shelf state for a user and catalog book.
type ShelfState struct {
	BookID      string
	InLibrary   bool
	ShelfStatus string
	LastReadAt  *time.Time
}
