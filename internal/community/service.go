package community

import (
	"crypto/rand"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
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
)

type Board struct {
	ID          string
	Name        string
	Description string
	Status      string
	CreatedAt   time.Time
}

type Thread struct {
	ID             string
	BoardID        string
	UserID         string
	Title          string
	Content        string
	OptionalBookID string
	Status         string
	CommentCount   int
	ReactionCounts map[string]int
	Comments       []Comment
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type Comment struct {
	ID              string
	ThreadID        string
	UserID          string
	ParentCommentID string
	Content         string
	Status          string
	ReactionCounts  map[string]int
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
	mu        sync.RWMutex
	boards    map[string]Board
	threads   map[string]Thread
	comments  map[string]Comment
	reactions map[string]Reaction
	now       func() time.Time
}

func NewService() *Service {
	s := &Service{
		boards:    make(map[string]Board),
		threads:   make(map[string]Thread),
		comments:  make(map[string]Comment),
		reactions: make(map[string]Reaction),
		now:       time.Now,
	}
	s.seedBoards()
	return s
}

func (s *Service) ListBoards() []Board {
	s.mu.RLock()
	defer s.mu.RUnlock()

	boards := make([]Board, 0, len(s.boards))
	for _, board := range s.boards {
		boards = append(boards, board)
	}
	sort.Slice(boards, func(i, j int) bool {
		return boards[i].CreatedAt.Before(boards[j].CreatedAt)
	})
	return boards
}

func (s *Service) ListThreads(boardID string, page, pageSize int) ([]Thread, error) {
	boardID = strings.TrimSpace(boardID)
	if boardID == "" {
		return nil, fmt.Errorf("%w: board ID is required", ErrInvalidInput)
	}
	page, pageSize = normalizePage(page, pageSize)

	s.mu.RLock()
	defer s.mu.RUnlock()

	if _, ok := s.boards[boardID]; !ok {
		return nil, ErrBoardNotFound
	}

	threads := make([]Thread, 0)
	for _, thread := range s.threads {
		if thread.BoardID == boardID && thread.Status == StatusActive {
			threads = append(threads, s.hydrateThreadLocked(thread))
		}
	}
	sort.Slice(threads, func(i, j int) bool {
		return threads[i].UpdatedAt.After(threads[j].UpdatedAt)
	})

	start := (page - 1) * pageSize
	if start >= len(threads) {
		return []Thread{}, nil
	}
	end := start + pageSize
	if end > len(threads) {
		end = len(threads)
	}
	return threads[start:end], nil
}

func (s *Service) CreateThread(userID, boardID, title, content, optionalBookID string) (Thread, error) {
	userID = strings.TrimSpace(userID)
	boardID = strings.TrimSpace(boardID)
	title = strings.TrimSpace(title)
	content = strings.TrimSpace(content)
	optionalBookID = strings.TrimSpace(optionalBookID)
	if userID == "" || boardID == "" || title == "" || content == "" {
		return Thread{}, fmt.Errorf("%w: user ID, board ID, title, and content are required", ErrInvalidInput)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.boards[boardID]; !ok {
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
	s.threads[thread.ID] = thread
	return thread, nil
}

func (s *Service) GetThread(id string) (Thread, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return Thread{}, fmt.Errorf("%w: thread ID is required", ErrInvalidInput)
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	thread, ok := s.threads[id]
	if !ok || thread.Status != StatusActive {
		return Thread{}, ErrThreadNotFound
	}
	return s.hydrateThreadLocked(thread), nil
}

func (s *Service) AddComment(userID, threadID, parentCommentID, content string) (Comment, error) {
	userID = strings.TrimSpace(userID)
	threadID = strings.TrimSpace(threadID)
	parentCommentID = strings.TrimSpace(parentCommentID)
	content = strings.TrimSpace(content)
	if userID == "" || threadID == "" || content == "" {
		return Comment{}, fmt.Errorf("%w: user ID, thread ID, and content are required", ErrInvalidInput)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	thread, ok := s.threads[threadID]
	if !ok || thread.Status != StatusActive {
		return Comment{}, ErrThreadNotFound
	}
	if parentCommentID != "" {
		parent, ok := s.comments[parentCommentID]
		if !ok || parent.ThreadID != threadID || parent.Status != StatusActive {
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
	s.comments[comment.ID] = comment
	thread.CommentCount++
	thread.UpdatedAt = now
	s.threads[threadID] = thread

	return comment, nil
}

func (s *Service) React(userID, targetType, targetID, reactionType string) (Reaction, error) {
	userID = strings.TrimSpace(userID)
	targetType = strings.TrimSpace(targetType)
	targetID = strings.TrimSpace(targetID)
	reactionType = strings.TrimSpace(reactionType)
	if userID == "" || targetID == "" || !validTargetType(targetType) || !validReactionType(reactionType) {
		return Reaction{}, fmt.Errorf("%w: valid user, target, and reaction are required", ErrInvalidInput)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.validateTargetLocked(targetType, targetID); err != nil {
		return Reaction{}, err
	}

	now := s.now().UTC()
	key := reactionKey(userID, targetType, targetID)
	reaction, ok := s.reactions[key]
	if ok {
		reaction.ReactionType = reactionType
		reaction.Status = StatusActive
		reaction.UpdatedAt = now
	} else {
		reaction = Reaction{
			ID:           newID("rxn"),
			UserID:       userID,
			TargetType:   targetType,
			TargetID:     targetID,
			ReactionType: reactionType,
			Status:       StatusActive,
			CreatedAt:    now,
			UpdatedAt:    now,
		}
	}
	s.reactions[key] = reaction

	return reaction, nil
}

func (s *Service) seedBoards() {
	now := s.now().UTC()
	for i, board := range []Board{
		{ID: "general", Name: "General", Description: "Open discussion for readers."},
		{ID: "book-club", Name: "Book Club", Description: "Shared reads, prompts, and group notes."},
		{ID: "help", Name: "Help", Description: "Questions about books, imports, and the app."},
	} {
		board.Status = StatusActive
		board.CreatedAt = now.Add(time.Duration(i) * time.Millisecond)
		s.boards[board.ID] = board
	}
}

func (s *Service) hydrateThreadLocked(thread Thread) Thread {
	thread.Comments = []Comment{}
	thread.ReactionCounts = s.reactionCountsLocked(TargetTypeThread, thread.ID)
	for _, comment := range s.comments {
		if comment.ThreadID == thread.ID && comment.Status == StatusActive {
			comment.ReactionCounts = s.reactionCountsLocked(TargetTypeComment, comment.ID)
			thread.Comments = append(thread.Comments, comment)
		}
	}
	sort.Slice(thread.Comments, func(i, j int) bool {
		return thread.Comments[i].CreatedAt.Before(thread.Comments[j].CreatedAt)
	})
	thread.CommentCount = len(thread.Comments)
	return thread
}

func (s *Service) reactionCountsLocked(targetType, targetID string) map[string]int {
	counts := map[string]int{}
	for _, reaction := range s.reactions {
		if reaction.TargetType == targetType && reaction.TargetID == targetID && reaction.Status == StatusActive {
			counts[reaction.ReactionType]++
		}
	}
	return counts
}

func (s *Service) validateTargetLocked(targetType, targetID string) error {
	switch targetType {
	case TargetTypeThread:
		thread, ok := s.threads[targetID]
		if !ok || thread.Status != StatusActive {
			return ErrThreadNotFound
		}
	case TargetTypeComment:
		comment, ok := s.comments[targetID]
		if !ok || comment.Status != StatusActive {
			return ErrCommentNotFound
		}
	default:
		return ErrInvalidInput
	}
	return nil
}

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
