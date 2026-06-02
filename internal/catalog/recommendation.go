package catalog

import "time"

// Recommendation represents a scheduled or historical recommended catalog book.
type Recommendation struct {
	ID                 int64
	BookKey            string
	Comment            string
	ScheduledPublishAt time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
	Book               Book
}
