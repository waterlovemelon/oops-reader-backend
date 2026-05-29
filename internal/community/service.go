package community

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	StatusActive  = "active"
	StatusDeleted = "deleted"

	TargetTypeThread  = "thread"
	TargetTypeComment = "comment"

	ReactionLike = "like"
	ReactionLove = "love"
)

var (
	ErrInvalidInput    = errors.New("invalid input")
	ErrBoardNotFound   = errors.New("board not found")
	ErrThreadNotFound  = errors.New("thread not found")
	ErrCommentNotFound = errors.New("comment not found")

	ErrAttachmentNotFound      = errors.New("attachment not found")
	ErrAttachmentNotOwned      = errors.New("attachment not owned by user")
	ErrAttachmentAlreadyBound  = errors.New("attachment already bound")
	ErrAttachmentLimitExceeded = errors.New("attachment limit exceeded")
)

type Board struct {
	ID          string
	Name        string
	Description string
	Status      string
	SortOrder   int
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type Thread struct {
	ID              string
	BoardID         string
	UserID          string
	Title           string
	Content         string
	OptionalBookID  string
	Status          string
	CommentCount    int
	ReactionCount   int // total reactions, from DB column
	ReactionCounts  map[string]int
	Comments        []Comment
	Attachments     []Attachment
	LastCommentedAt *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type Comment struct {
	ID              string
	ThreadID        string
	UserID          string
	ParentCommentID string
	Content         string
	Status          string
	ReactionCount   int // total reactions, from DB column
	ReactionCounts  map[string]int
	Attachments     []Attachment
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type Reaction struct {
	ID           string
	UserID       string
	TargetType   string
	TargetID     string
	ReactionType string
	Status       string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type Service struct {
	store Store
	now   func() time.Time
}

// NewService creates a community Service with the given Store.
func NewService(store Store) *Service {
	s := &Service{
		store: store,
		now:   time.Now,
	}
	s.seedBoards()
	return s
}

// NewServiceWithDB creates a community Service backed by MySQL or MemoryStore.
func NewServiceWithDB(db *sql.DB) *Service {
	var store Store
	if db == nil {
		store = NewMemoryStore()
	} else {
		store = NewMySQLStore(db)
	}
	return NewService(store)
}

func (s *Service) seedBoards() {
	ctx := context.Background()
	boards := []Board{
		{ID: "general", Name: "General", Description: "Open discussion for readers.", SortOrder: 1},
		{ID: "book-club", Name: "Book Club", Description: "Shared reads, prompts, and group notes.", SortOrder: 2},
		{ID: "help", Name: "Help", Description: "Questions about books, imports, and the app.", SortOrder: 3},
	}
	for i := range boards {
		boards[i].Status = StatusActive
	}
	_ = s.store.EnsureDefaultBoards(ctx, boards)
}

func (s *Service) ListBoards(ctx context.Context) ([]Board, error) {
	return s.store.ListBoards(ctx)
}

func (s *Service) ListThreads(ctx context.Context, boardID string, page, pageSize int) ([]Thread, error) {
	boardID = strings.TrimSpace(boardID)
	if boardID == "" {
		return nil, fmt.Errorf("%w: board ID is required", ErrInvalidInput)
	}
	page, pageSize = normalizePage(page, pageSize)

	// Verify board exists
	boards, err := s.store.ListBoards(ctx)
	if err != nil {
		return nil, err
	}
	found := false
	for _, b := range boards {
		if b.ID == boardID {
			found = true
			break
		}
	}
	if !found {
		return nil, ErrBoardNotFound
	}

	offset := (page - 1) * pageSize
	threads, err := s.store.ListThreads(ctx, boardID, pageSize, offset)
	if err != nil {
		return nil, err
	}

	// Hydrate threads with comments, reactions, and attachments
	hydrated := make([]Thread, 0, len(threads))
	for _, thread := range threads {
		t, err := s.hydrateThread(ctx, thread)
		if err != nil {
			return nil, err
		}
		hydrated = append(hydrated, t)
	}
	return hydrated, nil
}

func (s *Service) CreateThread(ctx context.Context, userID, boardID, title, content, optionalBookID string, attachmentIDs []string) (Thread, error) {
	userID = strings.TrimSpace(userID)
	boardID = strings.TrimSpace(boardID)
	title = strings.TrimSpace(title)
	content = strings.TrimSpace(content)
	optionalBookID = strings.TrimSpace(optionalBookID)
	if userID == "" || boardID == "" || title == "" || content == "" {
		return Thread{}, fmt.Errorf("%w: user ID, board ID, title, and content are required", ErrInvalidInput)
	}

	// Validate attachment count
	if len(attachmentIDs) > MaxThreadAttachments {
		return Thread{}, fmt.Errorf("%w: thread can have at most %d attachments", ErrAttachmentLimitExceeded, MaxThreadAttachments)
	}

	// Verify board exists
	boards, err := s.store.ListBoards(ctx)
	if err != nil {
		return Thread{}, err
	}
	found := false
	for _, b := range boards {
		if b.ID == boardID {
			found = true
			break
		}
	}
	if !found {
		return Thread{}, ErrBoardNotFound
	}

	now := s.now().UTC()
	thread := Thread{
		ID:             newID("thr"),
		BoardID:        boardID,
		UserID:         userID,
		Title:          title,
		Content:        content,
		OptionalBookID: optionalBookID,
		Status:         StatusActive,
		ReactionCounts: map[string]int{},
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	created, err := s.store.CreateThread(ctx, thread, attachmentIDs)
	if err != nil {
		return Thread{}, err
	}

	// Return hydrated thread
	return s.hydrateThread(ctx, created)
}

func (s *Service) GetThread(ctx context.Context, id string) (Thread, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return Thread{}, fmt.Errorf("%w: thread ID is required", ErrInvalidInput)
	}

	thread, err := s.store.GetThread(ctx, id)
	if err != nil {
		return Thread{}, err
	}
	return s.hydrateThread(ctx, thread)
}

func (s *Service) AddComment(ctx context.Context, userID, threadID, parentCommentID, content string, attachmentIDs []string) (Comment, error) {
	userID = strings.TrimSpace(userID)
	threadID = strings.TrimSpace(threadID)
	parentCommentID = strings.TrimSpace(parentCommentID)
	content = strings.TrimSpace(content)
	if userID == "" || threadID == "" || content == "" {
		return Comment{}, fmt.Errorf("%w: user ID, thread ID, and content are required", ErrInvalidInput)
	}

	// Validate attachment count
	if len(attachmentIDs) > MaxCommentAttachments {
		return Comment{}, fmt.Errorf("%w: comment can have at most %d attachments", ErrAttachmentLimitExceeded, MaxCommentAttachments)
	}

	// Verify thread exists and is active
	thread, err := s.store.GetThread(ctx, threadID)
	if err != nil {
		return Comment{}, ErrThreadNotFound
	}
	if thread.Status != StatusActive {
		return Comment{}, ErrThreadNotFound
	}

	// If replying, verify parent comment exists and belongs to same thread
	if parentCommentID != "" {
		comments, err := s.store.ListComments(ctx, threadID)
		if err != nil {
			return Comment{}, err
		}
		found := false
		for _, c := range comments {
			if c.ID == parentCommentID && c.Status == StatusActive {
				found = true
				break
			}
		}
		if !found {
			return Comment{}, ErrCommentNotFound
		}
	}

	now := s.now().UTC()
	comment := Comment{
		ID:              newID("cmt"),
		ThreadID:        threadID,
		UserID:          userID,
		ParentCommentID: parentCommentID,
		Content:         content,
		Status:          StatusActive,
		ReactionCounts:  map[string]int{},
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	created, err := s.store.CreateComment(ctx, comment, attachmentIDs)
	if err != nil {
		return Comment{}, err
	}

	// Hydrate comment with attachments
	created, err = s.hydrateComment(ctx, created)
	if err != nil {
		return Comment{}, err
	}

	return created, nil
}

func (s *Service) React(ctx context.Context, userID, targetType, targetID, reactionType string) (Reaction, error) {
	userID = strings.TrimSpace(userID)
	targetType = strings.TrimSpace(targetType)
	targetID = strings.TrimSpace(targetID)
	reactionType = strings.TrimSpace(reactionType)
	if userID == "" || targetID == "" || !validTargetType(targetType) || !validReactionType(reactionType) {
		return Reaction{}, fmt.Errorf("%w: valid user, target, and reaction are required", ErrInvalidInput)
	}

	// Validate target exists
	if err := s.validateTarget(ctx, targetType, targetID); err != nil {
		return Reaction{}, err
	}

	now := s.now().UTC()
	reaction := Reaction{
		ID:           newID("rxn"),
		UserID:       userID,
		TargetType:   targetType,
		TargetID:     targetID,
		ReactionType: reactionType,
		Status:       StatusActive,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	return s.store.UpsertReaction(ctx, reaction)
}

// ── Attachments ──────────────────────────────────────────────────────────────

// CreateAttachmentInput contains the data for creating an attachment record.
type CreateAttachmentInput struct {
	OwnerUserID     string
	FileType        string
	StorageProvider string
	StorageKey      string
	PublicURL       string
	MIMEType        string
	FileSize        uint
	Width           uint
	Height          uint
	ChecksumSHA256  string
}

// CreateAttachment creates a pending attachment record in the store.
func (s *Service) CreateAttachment(ctx context.Context, input CreateAttachmentInput) (Attachment, error) {
	now := s.now().UTC()
	att := Attachment{
		ID:              newID("att"),
		OwnerUserID:     input.OwnerUserID,
		FileType:        input.FileType,
		StorageProvider: input.StorageProvider,
		StorageKey:      input.StorageKey,
		PublicURL:       input.PublicURL,
		MIMEType:        input.MIMEType,
		FileSize:        input.FileSize,
		Width:           input.Width,
		Height:          input.Height,
		ChecksumSHA256:  input.ChecksumSHA256,
		Status:          StatusPending,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	return s.store.CreateAttachment(ctx, att)
}

// ── hydration helpers ────────────────────────────────────────────────────────

func (s *Service) hydrateThread(ctx context.Context, thread Thread) (Thread, error) {
	// Get comments
	comments, err := s.store.ListComments(ctx, thread.ID)
	if err != nil {
		return Thread{}, err
	}

	// Get reaction counts for thread and all comments
	targets := []ReactionTarget{
		{TargetType: TargetTypeThread, TargetID: thread.ID},
	}
	for _, c := range comments {
		targets = append(targets, ReactionTarget{TargetType: TargetTypeComment, TargetID: c.ID})
	}

	reactionCounts, err := s.store.CountReactions(ctx, targets)
	if err != nil {
		return Thread{}, err
	}

	thread.ReactionCounts = reactionCounts[ReactionTarget{TargetType: TargetTypeThread, TargetID: thread.ID}]
	if thread.ReactionCounts == nil {
		thread.ReactionCounts = map[string]int{}
	}

	// Hydrate comments with reaction counts and attachments
	attTargets := []AttachmentTarget{{TargetType: TargetTypeThread, TargetID: thread.ID}}
	for _, c := range comments {
		attTargets = append(attTargets, AttachmentTarget{TargetType: TargetTypeComment, TargetID: c.ID})
	}
	attMap, err := s.store.ListAttachments(ctx, attTargets)
	if err != nil {
		return Thread{}, err
	}

	thread.Attachments = attMap[AttachmentTarget{TargetType: TargetTypeThread, TargetID: thread.ID}]

	hydratedComments := make([]Comment, 0, len(comments))
	for _, c := range comments {
		c.ReactionCounts = reactionCounts[ReactionTarget{TargetType: TargetTypeComment, TargetID: c.ID}]
		if c.ReactionCounts == nil {
			c.ReactionCounts = map[string]int{}
		}
		c.Attachments = attMap[AttachmentTarget{TargetType: TargetTypeComment, TargetID: c.ID}]
		hydratedComments = append(hydratedComments, c)
	}
	thread.Comments = hydratedComments
	thread.CommentCount = len(hydratedComments)

	return thread, nil
}

func (s *Service) hydrateComment(ctx context.Context, comment Comment) (Comment, error) {
	targets := []ReactionTarget{
		{TargetType: TargetTypeComment, TargetID: comment.ID},
	}
	reactionCounts, err := s.store.CountReactions(ctx, targets)
	if err != nil {
		return Comment{}, err
	}
	comment.ReactionCounts = reactionCounts[ReactionTarget{TargetType: TargetTypeComment, TargetID: comment.ID}]
	if comment.ReactionCounts == nil {
		comment.ReactionCounts = map[string]int{}
	}

	attTargets := []AttachmentTarget{
		{TargetType: TargetTypeComment, TargetID: comment.ID},
	}
	attMap, err := s.store.ListAttachments(ctx, attTargets)
	if err != nil {
		return Comment{}, err
	}
	comment.Attachments = attMap[AttachmentTarget{TargetType: TargetTypeComment, TargetID: comment.ID}]

	return comment, nil
}

func (s *Service) validateTarget(ctx context.Context, targetType, targetID string) error {
	switch targetType {
	case TargetTypeThread:
		thread, err := s.store.GetThread(ctx, targetID)
		if err != nil || thread.Status != StatusActive {
			return ErrThreadNotFound
		}
	case TargetTypeComment:
		comment, err := s.store.GetComment(ctx, targetID)
		if err != nil || comment.Status != StatusActive {
			return ErrCommentNotFound
		}
	default:
		return ErrInvalidInput
	}
	return nil
}

// ── utilities ────────────────────────────────────────────────────────────────

func validTargetType(targetType string) bool {
	return targetType == TargetTypeThread || targetType == TargetTypeComment
}

func validReactionType(reactionType string) bool {
	return reactionType == ReactionLike || reactionType == ReactionLove
}

func reactionKey(userID, targetType, targetID string) string {
	return userID + ":" + targetType + ":" + targetID
}

func normalizePage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 50 {
		pageSize = 50
	}
	return page, pageSize
}

func newID(prefix string) string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
	}
	return fmt.Sprintf("%s_%x", prefix, b[:])
}
