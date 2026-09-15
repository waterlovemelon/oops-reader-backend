// Package reading persists per-account reading position.
package reading

import (
	"context"
	"errors"
	"strings"
	"time"
)

// Progress types accepted in ProgressInput.ProgressType. They mirror the
// reading_progress.progress_type column.
const (
	ProgressTypePage     = "page"
	ProgressTypeChapter  = "chapter"
	ProgressTypePercent  = "percent"
	ProgressTypeLocation = "location"
)

// BookKeyPrefixCatalog marks the book identifier of an online catalog book.
// Only online books sync: locally imported books are identified by the import
// timestamp, which differs on every device, so their progress cannot be
// merged across devices.
const BookKeyPrefixCatalog = "catalog:"

// Column bounds of reading_progress and sync_operations. Requests longer than
// these are rejected instead of letting MySQL truncate the value.
const (
	MaxBookKeyLength        = 191
	MaxProgressValueLength  = 64
	MaxChapterTitleLength   = 255
	MaxPositionCFILength    = 1024
	MaxContentVersionLength = 191
	MaxDeviceIDLength       = 128
	MaxOperationIDLength    = 128
)

// List bounds shared by the service and the HTTP layer.
const (
	DefaultProgressPageSize = 20
	MaxProgressPageSize     = 100
)

// ClockSkewAllowance is how far a client timestamp may lead the server clock
// before it is collapsed onto server time. Client clocks are not trusted, and
// a future timestamp would win every merge forever.
const ClockSkewAllowance = 5 * time.Minute

var (
	ErrInvalidBookKey        = errors.New("book_key is required and must be at most 191 characters")
	ErrUnsupportedBookKey    = errors.New("book_key must reference an online catalog book (catalog:<book_key>)")
	ErrUnknownBook           = errors.New("catalog book not found")
	ErrInvalidProgressType   = errors.New("progress_type must be page, chapter, percent, or location")
	ErrInvalidProgressValue  = errors.New("progress_value is required when progress_percent is missing and must be at most 64 characters")
	ErrInvalidPercent        = errors.New("progress_percent must be a number between 0 and 100")
	ErrInvalidChapterTitle   = errors.New("chapter_title must be at most 255 characters")
	ErrInvalidPositionCFI    = errors.New("position_cfi must be at most 1024 characters")
	ErrInvalidContentVersion = errors.New("content_version must be at most 191 characters")
	ErrInvalidDeviceID       = errors.New("device_id must be at most 128 characters")
	ErrInvalidOperationID    = errors.New("operation_id must be at most 128 characters")
	ErrNotFound              = errors.New("reading progress not found")
)

// Progress is the stored reading position of one account and one book.
type Progress struct {
	BookKey         string    `json:"book_key"`
	ProgressType    string    `json:"progress_type"`
	ProgressValue   string    `json:"progress_value"`
	ProgressPercent *float64  `json:"progress_percent,omitempty"`
	ChapterTitle    string    `json:"chapter_title,omitempty"`
	PositionCFI     string    `json:"position_cfi,omitempty"`
	ContentVersion  string    `json:"content_version,omitempty"`
	DeviceID        string    `json:"device_id,omitempty"`
	RecordedAt      time.Time `json:"recorded_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// ProgressInput is an upsert request before validation.
type ProgressInput struct {
	BookKey         string
	ProgressType    string
	ProgressValue   string
	ProgressPercent *float64
	ChapterTitle    string
	PositionCFI     string
	ContentVersion  string
	DeviceID        string
	RecordedAt      *time.Time
	// OperationID makes a client upload idempotent: replaying the same
	// (device_id, operation_id) never writes twice.
	OperationID string
}

// ProgressOperation is the idempotency and audit entry of one upload, stored in
// sync_operations.
type ProgressOperation struct {
	OperationID      string
	DeviceID         string
	Payload          []byte
	ClientOccurredAt time.Time
}

// UpsertOutcome reports what the store did with an upload.
type UpsertOutcome struct {
	// Progress is the authoritative stored row after the call. When Applied is
	// false it is the row that won the merge, so the client can reconcile.
	Progress  Progress
	Applied   bool
	Duplicate bool
}

// ProgressStore persists progress rows keyed by account and book.
type ProgressStore interface {
	GetProgress(ctx context.Context, userID uint64, bookKey string) (Progress, error)
	ListProgress(ctx context.Context, userID uint64, limit, offset int) ([]Progress, int, error)
	UpsertProgress(ctx context.Context, userID uint64, incoming Progress, operation ProgressOperation) (UpsertOutcome, error)
}

// BookLookup reports whether a catalog book_key exists.
type BookLookup func(ctx context.Context, catalogBookKey string) (bool, error)

// DeviceRecorder registers a device that reported reading activity.
type DeviceRecorder func(ctx context.Context, userID uint64, deviceID string) error

// CatalogBookKey returns the catalog book_key of an online book, or "" when the
// key does not address one.
func CatalogBookKey(bookKey string) string {
	if !strings.HasPrefix(bookKey, BookKeyPrefixCatalog) {
		return ""
	}
	return strings.TrimPrefix(bookKey, BookKeyPrefixCatalog)
}

// CatalogShelfBookKey returns the user_catalog_bookshelves.catalog_book_key of
// an online book in the same catalog:<id> form as reading_progress.book_key, so
// the shelf and the progress projection of "recently read" never disagree. The
// value is accepted with or without the catalog: prefix; "" when it addresses
// nothing.
func CatalogShelfBookKey(bookKey string) string {
	id := strings.TrimSpace(bookKey)
	id = strings.TrimPrefix(id, BookKeyPrefixCatalog)
	if id == "" {
		return ""
	}
	return BookKeyPrefixCatalog + id
}

// Preferred reports whether incoming must replace current. The later progress
// generation time wins; equal timestamps are broken by the lexicographically
// larger device_id so every device converges on the same row.
func Preferred(current, incoming Progress) bool {
	if incoming.RecordedAt.After(current.RecordedAt) {
		return true
	}
	if incoming.RecordedAt.Before(current.RecordedAt) {
		return false
	}
	return incoming.DeviceID > current.DeviceID
}

// ClampRecordedAt collapses a client timestamp that leads the server clock by
// more than ClockSkewAllowance onto server time, and reports whether it did.
func ClampRecordedAt(recordedAt, now time.Time) (time.Time, bool) {
	if recordedAt.After(now.Add(ClockSkewAllowance)) {
		return now, true
	}
	return recordedAt, false
}
