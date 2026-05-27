package community

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type MySQLStore struct {
	db *sql.DB
}

func NewMySQLStore(db *sql.DB) *MySQLStore {
	return &MySQLStore{db: db}
}

func (s *MySQLStore) ListBoards() ([]Board, error) {
	rows, err := s.db.Query(`
SELECT id, name, description, status, created_at
FROM community_boards
WHERE status = ?
ORDER BY sort_order ASC, created_at ASC`, StatusActive)
	if err != nil {
		return nil, fmt.Errorf("list community boards: %w", err)
	}
	defer rows.Close()

	boards := []Board{}
	for rows.Next() {
		var board Board
		if err := rows.Scan(&board.ID, &board.Name, &board.Description, &board.Status, &board.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan community board: %w", err)
		}
		boards = append(boards, board)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate community boards: %w", err)
	}
	return boards, nil
}

func (s *MySQLStore) ListThreads(boardID string, page, pageSize int) ([]Thread, error) {
	var exists bool
	if err := s.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM community_boards WHERE id = ? AND status = ?)`, boardID, StatusActive).Scan(&exists); err != nil {
		return nil, fmt.Errorf("check community board: %w", err)
	}
	if !exists {
		return nil, ErrBoardNotFound
	}
	return s.listThreads(`board_id = ?`, []any{boardID}, page, pageSize)
}

func (s *MySQLStore) ListUserThreads(userID string, page, pageSize int) ([]Thread, error) {
	return s.listThreads(`author_user_id = ?`, []any{userID}, page, pageSize)
}

func (s *MySQLStore) CreateThread(userID, boardID, title, content, optionalBookID string) (Thread, error) {
	var exists bool
	if err := s.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM community_boards WHERE id = ? AND status = ?)`, boardID, StatusActive).Scan(&exists); err != nil {
		return Thread{}, fmt.Errorf("check community board: %w", err)
	}
	if !exists {
		return Thread{}, ErrBoardNotFound
	}

	now := time.Now().UTC()
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
	_, err := s.db.Exec(`
INSERT INTO community_threads
    (id, board_id, author_user_id, title, content, optional_book_id, status, comment_count, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, 0, ?, ?)`,
		thread.ID, thread.BoardID, thread.UserID, thread.Title, thread.Content, thread.OptionalBookID, thread.Status, thread.CreatedAt, thread.UpdatedAt,
	)
	if err != nil {
		return Thread{}, fmt.Errorf("create community thread: %w", err)
	}
	return thread, nil
}

func (s *MySQLStore) GetThread(id string) (Thread, error) {
	thread, err := s.scanThread(s.db.QueryRow(`
SELECT id, board_id, author_user_id, title, content, optional_book_id, status, comment_count, created_at, updated_at
FROM community_threads
WHERE id = ? AND status = ?`, id, StatusActive))
	if errors.Is(err, sql.ErrNoRows) {
		return Thread{}, ErrThreadNotFound
	}
	if err != nil {
		return Thread{}, fmt.Errorf("get community thread: %w", err)
	}
	return s.hydrateThread(thread)
}

func (s *MySQLStore) AddComment(userID, threadID, parentCommentID, content string) (Comment, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return Comment{}, fmt.Errorf("begin community comment transaction: %w", err)
	}
	defer tx.Rollback()

	var exists bool
	if err := tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM community_threads WHERE id = ? AND status = ?)`, threadID, StatusActive).Scan(&exists); err != nil {
		return Comment{}, fmt.Errorf("check community thread: %w", err)
	}
	if !exists {
		return Comment{}, ErrThreadNotFound
	}
	if parentCommentID != "" {
		var parentParent string
		err := tx.QueryRow(`
SELECT parent_comment_id
FROM community_comments
WHERE id = ? AND thread_id = ? AND status = ?`, parentCommentID, threadID, StatusActive).Scan(&parentParent)
		if errors.Is(err, sql.ErrNoRows) {
			return Comment{}, ErrCommentNotFound
		}
		if err != nil {
			return Comment{}, fmt.Errorf("check parent community comment: %w", err)
		}
		if parentParent != "" {
			return Comment{}, fmt.Errorf("%w: second-level replies are not supported", ErrInvalidInput)
		}
	}

	now := time.Now().UTC()
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
	if _, err := tx.Exec(`
INSERT INTO community_comments
    (id, thread_id, author_user_id, parent_comment_id, content, status, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		comment.ID, comment.ThreadID, comment.UserID, comment.ParentCommentID, comment.Content, comment.Status, comment.CreatedAt, comment.UpdatedAt,
	); err != nil {
		return Comment{}, fmt.Errorf("create community comment: %w", err)
	}
	if _, err := tx.Exec(`
UPDATE community_threads
SET comment_count = comment_count + 1, updated_at = ?
WHERE id = ?`, now, threadID); err != nil {
		return Comment{}, fmt.Errorf("update community thread comment count: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Comment{}, fmt.Errorf("commit community comment transaction: %w", err)
	}
	return comment, nil
}

func (s *MySQLStore) React(userID, targetType, targetID, reactionType string) (Reaction, error) {
	if err := s.validateTarget(targetType, targetID); err != nil {
		return Reaction{}, err
	}

	now := time.Now().UTC()
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
	_, err := s.db.Exec(`
INSERT INTO community_reactions
    (id, author_user_id, target_type, target_id, reaction_type, status, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE
    reaction_type = VALUES(reaction_type),
    status = VALUES(status),
    updated_at = VALUES(updated_at)`,
		reaction.ID, reaction.UserID, reaction.TargetType, reaction.TargetID, reaction.ReactionType, reaction.Status, reaction.CreatedAt, reaction.UpdatedAt,
	)
	if err != nil {
		return Reaction{}, fmt.Errorf("upsert community reaction: %w", err)
	}
	row := s.db.QueryRow(`
SELECT id, author_user_id, target_type, target_id, reaction_type, status, created_at, updated_at
FROM community_reactions
WHERE author_user_id = ? AND target_type = ? AND target_id = ?`,
		userID, targetType, targetID,
	)
	if err := row.Scan(&reaction.ID, &reaction.UserID, &reaction.TargetType, &reaction.TargetID, &reaction.ReactionType, &reaction.Status, &reaction.CreatedAt, &reaction.UpdatedAt); err != nil {
		return Reaction{}, fmt.Errorf("get community reaction: %w", err)
	}
	return reaction, nil
}

func (s *MySQLStore) listThreads(where string, args []any, page, pageSize int) ([]Thread, error) {
	offset := (page - 1) * pageSize
	queryArgs := append(append([]any{}, args...), StatusActive, pageSize, offset)
	rows, err := s.db.Query(`
SELECT id, board_id, author_user_id, title, content, optional_book_id, status, comment_count, created_at, updated_at
FROM community_threads
WHERE `+where+` AND status = ?
ORDER BY updated_at DESC, created_at DESC
LIMIT ? OFFSET ?`, queryArgs...)
	if err != nil {
		return nil, fmt.Errorf("list community threads: %w", err)
	}
	defer rows.Close()

	threads := []Thread{}
	for rows.Next() {
		thread, err := s.scanThread(rows)
		if err != nil {
			return nil, fmt.Errorf("scan community thread: %w", err)
		}
		hydrated, err := s.hydrateThread(thread)
		if err != nil {
			return nil, err
		}
		threads = append(threads, hydrated)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate community threads: %w", err)
	}
	return threads, nil
}

func (s *MySQLStore) hydrateThread(thread Thread) (Thread, error) {
	comments, err := s.listComments(thread.ID)
	if err != nil {
		return Thread{}, err
	}
	thread.Comments = comments
	thread.CommentCount = len(comments)
	thread.ReactionCounts, err = s.reactionCounts(TargetTypeThread, thread.ID)
	if err != nil {
		return Thread{}, err
	}
	return thread, nil
}

func (s *MySQLStore) listComments(threadID string) ([]Comment, error) {
	rows, err := s.db.Query(`
SELECT id, thread_id, author_user_id, parent_comment_id, content, status, created_at, updated_at
FROM community_comments
WHERE thread_id = ? AND status = ?
ORDER BY created_at ASC`, threadID, StatusActive)
	if err != nil {
		return nil, fmt.Errorf("list community comments: %w", err)
	}
	defer rows.Close()

	comments := []Comment{}
	for rows.Next() {
		var comment Comment
		if err := rows.Scan(&comment.ID, &comment.ThreadID, &comment.UserID, &comment.ParentCommentID, &comment.Content, &comment.Status, &comment.CreatedAt, &comment.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan community comment: %w", err)
		}
		counts, err := s.reactionCounts(TargetTypeComment, comment.ID)
		if err != nil {
			return nil, err
		}
		comment.ReactionCounts = counts
		comments = append(comments, comment)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate community comments: %w", err)
	}
	return comments, nil
}

func (s *MySQLStore) reactionCounts(targetType, targetID string) (map[string]int, error) {
	rows, err := s.db.Query(`
SELECT reaction_type, COUNT(*)
FROM community_reactions
WHERE target_type = ? AND target_id = ? AND status = ?
GROUP BY reaction_type`, targetType, targetID, StatusActive)
	if err != nil {
		return nil, fmt.Errorf("count community reactions: %w", err)
	}
	defer rows.Close()

	counts := map[string]int{}
	for rows.Next() {
		var reactionType string
		var count int
		if err := rows.Scan(&reactionType, &count); err != nil {
			return nil, fmt.Errorf("scan community reaction count: %w", err)
		}
		counts[reactionType] = count
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate community reaction counts: %w", err)
	}
	return counts, nil
}

func (s *MySQLStore) validateTarget(targetType, targetID string) error {
	var query string
	var notFound error
	switch targetType {
	case TargetTypeThread:
		query = `SELECT EXISTS(SELECT 1 FROM community_threads WHERE id = ? AND status = ?)`
		notFound = ErrThreadNotFound
	case TargetTypeComment:
		query = `SELECT EXISTS(SELECT 1 FROM community_comments WHERE id = ? AND status = ?)`
		notFound = ErrCommentNotFound
	default:
		return ErrInvalidInput
	}
	var exists bool
	if err := s.db.QueryRow(query, targetID, StatusActive).Scan(&exists); err != nil {
		return fmt.Errorf("check community reaction target: %w", err)
	}
	if !exists {
		return notFound
	}
	return nil
}

type threadScanner interface {
	Scan(dest ...any) error
}

func (s *MySQLStore) scanThread(scanner threadScanner) (Thread, error) {
	var thread Thread
	if err := scanner.Scan(
		&thread.ID,
		&thread.BoardID,
		&thread.UserID,
		&thread.Title,
		&thread.Content,
		&thread.OptionalBookID,
		&thread.Status,
		&thread.CommentCount,
		&thread.CreatedAt,
		&thread.UpdatedAt,
	); err != nil {
		return Thread{}, err
	}
	return thread, nil
}
