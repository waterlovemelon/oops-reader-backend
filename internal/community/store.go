package community

import (
	"context"
	"time"
)

// Additional status constants for threads and attachments.
const (
	StatusHidden   = "hidden"
	StatusLocked   = "locked"
	StatusPending  = "pending"
	StatusRejected = "rejected"

	FileTypeImage = "image"

	MaxThreadAttachments  = 9
	MaxCommentAttachments = 3
)

// Attachment represents a file (image) uploaded for a thread or comment.
type Attachment struct {
	ID              string
	OwnerUserID     string
	TargetType      string
	TargetID        string
	FileType        string
	StorageProvider string
	StorageKey      string
	PublicURL       string
	MIMEType        string
	FileSize        uint
	Width           uint
	Height          uint
	ChecksumSHA256  string
	Status          string
	SortOrder       int
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// AttachmentTarget identifies a set of attachments to look up.
type AttachmentTarget struct {
	TargetType string
	TargetID   string
}

// ReactionTarget identifies a target for counting reactions.
type ReactionTarget struct {
	TargetType string
	TargetID   string
}

// Store defines the persistence contract for the community domain.
// Implementations: MySQLStore (production), MemoryStore (testing / nil-DB).
type Store interface {
	// Boards
	ListBoards(ctx context.Context) ([]Board, error)
	EnsureDefaultBoards(ctx context.Context, boards []Board) error

	// Threads
	ListThreads(ctx context.Context, boardID string, limit, offset int) ([]Thread, error)
	CreateThread(ctx context.Context, thread Thread, attachmentIDs []string) (Thread, error)
	GetThread(ctx context.Context, id string) (Thread, error)

	// Comments
	ListComments(ctx context.Context, threadID string) ([]Comment, error)
	CreateComment(ctx context.Context, comment Comment, attachmentIDs []string) (Comment, error)
	GetComment(ctx context.Context, id string) (Comment, error)

	// Reactions
	UpsertReaction(ctx context.Context, reaction Reaction) (Reaction, error)
	CountReactions(ctx context.Context, targets []ReactionTarget) (map[ReactionTarget]map[string]int, error)

	// Attachments
	CreateAttachment(ctx context.Context, attachment Attachment) (Attachment, error)
	BindAttachments(ctx context.Context, ownerUserID, targetType, targetID string, attachmentIDs []string) error
	ListAttachments(ctx context.Context, targets []AttachmentTarget) (map[AttachmentTarget][]Attachment, error)
}
