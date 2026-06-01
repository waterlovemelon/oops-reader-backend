package catalog

import "time"

// BookComment represents a reader comment on a catalog book.
type BookComment struct {
	ID             string
	BookID         string
	UserID         uint64
	DisplayName    string
	Source         string
	ExternalSource string
	AuthorName     string
	Content        string
	LikeCount      int
	Status         string
	CreatedAt      time.Time
}
