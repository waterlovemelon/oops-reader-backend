package community

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
)

// MemoryStore implements Store using in-memory maps.
// Used for testing and when no database is available.
type MemoryStore struct {
	mu          sync.RWMutex
	boards      map[string]Board
	threads     map[string]Thread
	comments    map[string]Comment
	reactions   map[string]Reaction   // keyed by "userID:targetType:targetID"
	attachments map[string]Attachment // keyed by attachment ID
	now         func() time.Time
}

// NewMemoryStore creates a MemoryStore with default boards seeded.
func NewMemoryStore() *MemoryStore {
	s := &MemoryStore{
		boards:      make(map[string]Board),
		threads:     make(map[string]Thread),
		comments:    make(map[string]Comment),
		reactions:   make(map[string]Reaction),
		attachments: make(map[string]Attachment),
		now:         time.Now,
	}
	return s
}

// ── Boards ───────────────────────────────────────────────────────────────────

func (s *MemoryStore) ListBoards(_ context.Context) ([]Board, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	boards := make([]Board, 0, len(s.boards))
	for _, board := range s.boards {
		boards = append(boards, board)
	}
	sort.Slice(boards, func(i, j int) bool {
		return boards[i].CreatedAt.Before(boards[j].CreatedAt)
	})
	return boards, nil
}

func (s *MemoryStore) EnsureDefaultBoards(_ context.Context, boards []Board) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, board := range boards {
		if _, exists := s.boards[board.ID]; !exists {
			if board.CreatedAt.IsZero() {
				board.CreatedAt = s.now().UTC()
			}
			if board.UpdatedAt.IsZero() {
				board.UpdatedAt = board.CreatedAt
			}
			s.boards[board.ID] = board
		}
	}
	return nil
}

// ── Threads ──────────────────────────────────────────────────────────────────

func (s *MemoryStore) ListThreads(_ context.Context, boardID string, limit, offset int) ([]Thread, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	threads := make([]Thread, 0)
	for _, thread := range s.threads {
		if thread.BoardID == boardID && thread.Status == StatusActive {
			threads = append(threads, thread)
		}
	}
	sort.Slice(threads, func(i, j int) bool {
		return threads[i].UpdatedAt.After(threads[j].UpdatedAt)
	})

	if offset >= len(threads) {
		return []Thread{}, nil
	}
	end := offset + limit
	if end > len(threads) {
		end = len(threads)
	}
	return threads[offset:end], nil
}

func (s *MemoryStore) CreateThread(_ context.Context, thread Thread, attachmentIDs []string) (Thread, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now().UTC()

	// Validate all attachments first to avoid partial writes.
	for _, attID := range attachmentIDs {
		att, ok := s.attachments[attID]
		if !ok {
			return Thread{}, fmt.Errorf("%w: attachment %s", ErrAttachmentNotFound, attID)
		}
		if att.OwnerUserID != thread.UserID {
			return Thread{}, fmt.Errorf("%w: attachment %s", ErrAttachmentNotOwned, attID)
		}
		if att.Status != StatusPending {
			return Thread{}, fmt.Errorf("%w: attachment %s already bound", ErrAttachmentAlreadyBound, attID)
		}
	}

	// All validation passed — write thread.
	thread.CreatedAt = now
	thread.UpdatedAt = now
	thread.Status = StatusActive
	if thread.ReactionCounts == nil {
		thread.ReactionCounts = map[string]int{}
	}
	s.threads[thread.ID] = thread

	// Bind attachments.
	for _, attID := range attachmentIDs {
		att := s.attachments[attID]
		att.TargetType = TargetTypeThread
		att.TargetID = thread.ID
		att.Status = StatusActive
		att.UpdatedAt = now
		s.attachments[attID] = att
	}

	return thread, nil
}

func (s *MemoryStore) GetThread(_ context.Context, id string) (Thread, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	thread, ok := s.threads[id]
	if !ok || thread.Status != StatusActive {
		return Thread{}, ErrThreadNotFound
	}
	return thread, nil
}

// ── Comments ─────────────────────────────────────────────────────────────────

func (s *MemoryStore) ListComments(_ context.Context, threadID string) ([]Comment, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	comments := make([]Comment, 0)
	for _, comment := range s.comments {
		if comment.ThreadID == threadID && comment.Status == StatusActive {
			comments = append(comments, comment)
		}
	}
	sort.Slice(comments, func(i, j int) bool {
		return comments[i].CreatedAt.Before(comments[j].CreatedAt)
	})
	return comments, nil
}

func (s *MemoryStore) GetComment(_ context.Context, id string) (Comment, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	comment, ok := s.comments[id]
	if !ok || comment.Status != StatusActive {
		return Comment{}, ErrCommentNotFound
	}
	return comment, nil
}

func (s *MemoryStore) CreateComment(_ context.Context, comment Comment, attachmentIDs []string) (Comment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now().UTC()

	// Validate all attachments first to avoid partial writes.
	for _, attID := range attachmentIDs {
		att, ok := s.attachments[attID]
		if !ok {
			return Comment{}, fmt.Errorf("%w: attachment %s", ErrAttachmentNotFound, attID)
		}
		if att.OwnerUserID != comment.UserID {
			return Comment{}, fmt.Errorf("%w: attachment %s", ErrAttachmentNotOwned, attID)
		}
		if att.Status != StatusPending {
			return Comment{}, fmt.Errorf("%w: attachment %s already bound", ErrAttachmentAlreadyBound, attID)
		}
	}

	// All validation passed — write comment.
	comment.CreatedAt = now
	comment.UpdatedAt = now
	comment.Status = StatusActive
	if comment.ReactionCounts == nil {
		comment.ReactionCounts = map[string]int{}
	}
	s.comments[comment.ID] = comment

	// Update thread comment count and last_commented_at.
	thread, ok := s.threads[comment.ThreadID]
	if ok {
		thread.CommentCount++
		thread.UpdatedAt = now
		thread.LastCommentedAt = &now
		s.threads[comment.ThreadID] = thread
	}

	// Bind attachments.
	for _, attID := range attachmentIDs {
		att := s.attachments[attID]
		att.TargetType = TargetTypeComment
		att.TargetID = comment.ID
		att.Status = StatusActive
		att.UpdatedAt = now
		s.attachments[attID] = att
	}

	return comment, nil
}

// ── Reactions ────────────────────────────────────────────────────────────────

func (s *MemoryStore) UpsertReaction(_ context.Context, reaction Reaction) (Reaction, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := reactionKey(reaction.UserID, reaction.TargetType, reaction.TargetID)
	now := s.now().UTC()

	existing, ok := s.reactions[key]
	if ok {
		existing.ReactionType = reaction.ReactionType
		existing.Status = StatusActive
		existing.UpdatedAt = now
		s.reactions[key] = existing
		return existing, nil
	}

	reaction.CreatedAt = now
	reaction.UpdatedAt = now
	reaction.Status = StatusActive
	s.reactions[key] = reaction
	return reaction, nil
}

func (s *MemoryStore) CountReactions(_ context.Context, targets []ReactionTarget) (map[ReactionTarget]map[string]int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[ReactionTarget]map[string]int)
	for _, t := range targets {
		result[t] = map[string]int{}
	}

	for _, reaction := range s.reactions {
		if reaction.Status != StatusActive {
			continue
		}
		t := ReactionTarget{TargetType: reaction.TargetType, TargetID: reaction.TargetID}
		if _, ok := result[t]; ok {
			result[t][reaction.ReactionType]++
		}
	}
	return result, nil
}

// ── Attachments ──────────────────────────────────────────────────────────────

func (s *MemoryStore) CreateAttachment(_ context.Context, attachment Attachment) (Attachment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now().UTC()
	attachment.CreatedAt = now
	attachment.UpdatedAt = now
	if attachment.Status == "" {
		attachment.Status = StatusPending
	}
	s.attachments[attachment.ID] = attachment
	return attachment, nil
}

func (s *MemoryStore) BindAttachments(_ context.Context, ownerUserID, targetType, targetID string, attachmentIDs []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now().UTC()
	for _, attID := range attachmentIDs {
		att, ok := s.attachments[attID]
		if !ok {
			return fmt.Errorf("%w: attachment %s", ErrAttachmentNotFound, attID)
		}
		if att.OwnerUserID != ownerUserID {
			return fmt.Errorf("%w: attachment %s", ErrAttachmentNotOwned, attID)
		}
		if att.Status != StatusPending {
			return fmt.Errorf("%w: attachment %s already bound", ErrAttachmentAlreadyBound, attID)
		}
		att.TargetType = targetType
		att.TargetID = targetID
		att.Status = StatusActive
		att.UpdatedAt = now
		s.attachments[attID] = att
	}
	return nil
}

func (s *MemoryStore) ListAttachments(_ context.Context, targets []AttachmentTarget) (map[AttachmentTarget][]Attachment, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[AttachmentTarget][]Attachment)
	for _, t := range targets {
		result[t] = []Attachment{}
	}

	for _, att := range s.attachments {
		if att.Status != StatusActive {
			continue
		}
		t := AttachmentTarget{TargetType: att.TargetType, TargetID: att.TargetID}
		if _, ok := result[t]; ok {
			result[t] = append(result[t], att)
		}
	}

	// Sort each attachment list by sort_order then created_at
	for t := range result {
		atts := result[t]
		sort.Slice(atts, func(i, j int) bool {
			if atts[i].SortOrder != atts[j].SortOrder {
				return atts[i].SortOrder < atts[j].SortOrder
			}
			return atts[i].CreatedAt.Before(atts[j].CreatedAt)
		})
		result[t] = atts
	}

	return result, nil
}
