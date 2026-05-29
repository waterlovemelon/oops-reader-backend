package community

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// MySQLStore implements Store using MariaDB/MySQL.
type MySQLStore struct {
	db *sql.DB
}

// NewMySQLStore creates a MySQLStore.
func NewMySQLStore(db *sql.DB) *MySQLStore {
	return &MySQLStore{db: db}
}

// ── Boards ───────────────────────────────────────────────────────────────────

func (s *MySQLStore) ListBoards(ctx context.Context) ([]Board, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, description, status, sort_order, created_at, updated_at
		FROM community_boards
		WHERE status = 'active'
		ORDER BY sort_order, created_at
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var boards []Board
	for rows.Next() {
		var b Board
		var description sql.NullString
		if err := rows.Scan(&b.ID, &b.Name, &description, &b.Status, &b.SortOrder, &b.CreatedAt, &b.UpdatedAt); err != nil {
			return nil, err
		}
		b.Description = description.String
		boards = append(boards, b)
	}
	return boards, rows.Err()
}

func (s *MySQLStore) EnsureDefaultBoards(ctx context.Context, boards []Board) error {
	for _, board := range boards {
		_, err := s.db.ExecContext(ctx, `
			INSERT IGNORE INTO community_boards (id, name, description, status, sort_order, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)
		`, board.ID, board.Name, nullableString(board.Description), board.Status, board.SortOrder, board.CreatedAt, board.UpdatedAt)
		if err != nil {
			return err
		}
	}
	return nil
}

// ── Threads ──────────────────────────────────────────────────────────────────

func (s *MySQLStore) ListThreads(ctx context.Context, boardID string, limit, offset int) ([]Thread, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, board_id, user_id, title, content, optional_book_id, status,
		       comment_count, reaction_count, last_commented_at, created_at, updated_at
		FROM community_threads
		WHERE board_id = ? AND status = 'active'
		ORDER BY updated_at DESC
		LIMIT ? OFFSET ?
	`, boardID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var threads []Thread
	for rows.Next() {
		t, err := scanThread(rows)
		if err != nil {
			return nil, err
		}
		threads = append(threads, t)
	}
	return threads, rows.Err()
}

func (s *MySQLStore) CreateThread(ctx context.Context, thread Thread, attachmentIDs []string) (Thread, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Thread{}, err
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `
		INSERT INTO community_threads (id, board_id, user_id, title, content, optional_book_id, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, thread.ID, thread.BoardID, thread.UserID, thread.Title, thread.Content,
		nullableString(thread.OptionalBookID), thread.Status, thread.CreatedAt, thread.UpdatedAt)
	if err != nil {
		return Thread{}, err
	}

	// Bind attachments
	if len(attachmentIDs) > 0 {
		if err := bindAttachmentsTx(ctx, tx, thread.UserID, TargetTypeThread, thread.ID, attachmentIDs); err != nil {
			return Thread{}, err
		}
	}

	if err := tx.Commit(); err != nil {
		return Thread{}, err
	}

	return s.GetThread(ctx, thread.ID)
}

func (s *MySQLStore) GetThread(ctx context.Context, id string) (Thread, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, board_id, user_id, title, content, optional_book_id, status,
		       comment_count, reaction_count, last_commented_at, created_at, updated_at
		FROM community_threads
		WHERE id = ? AND status = 'active'
	`, id)
	return scanThreadRow(row)
}

// ── Comments ─────────────────────────────────────────────────────────────────

func (s *MySQLStore) ListComments(ctx context.Context, threadID string) ([]Comment, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, thread_id, user_id, parent_comment_id, content, status,
		       reaction_count, created_at, updated_at
		FROM community_comments
		WHERE thread_id = ? AND status = 'active'
		ORDER BY created_at ASC
	`, threadID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var comments []Comment
	for rows.Next() {
		c, err := scanComment(rows)
		if err != nil {
			return nil, err
		}
		comments = append(comments, c)
	}
	return comments, rows.Err()
}

func (s *MySQLStore) CreateComment(ctx context.Context, comment Comment, attachmentIDs []string) (Comment, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Comment{}, err
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `
		INSERT INTO community_comments (id, thread_id, user_id, parent_comment_id, content, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, comment.ID, comment.ThreadID, comment.UserID, nullableString(comment.ParentCommentID),
		comment.Content, comment.Status, comment.CreatedAt, comment.UpdatedAt)
	if err != nil {
		return Comment{}, err
	}

	// Update thread comment_count and last_commented_at
	now := time.Now().UTC()
	_, err = tx.ExecContext(ctx, `
		UPDATE community_threads
		SET comment_count = comment_count + 1, updated_at = ?, last_commented_at = ?
		WHERE id = ?
	`, now, now, comment.ThreadID)
	if err != nil {
		return Comment{}, err
	}

	// Bind attachments
	if len(attachmentIDs) > 0 {
		if err := bindAttachmentsTx(ctx, tx, comment.UserID, TargetTypeComment, comment.ID, attachmentIDs); err != nil {
			return Comment{}, err
		}
	}

	if err := tx.Commit(); err != nil {
		return Comment{}, err
	}

	// Re-read the created comment
	return s.GetComment(ctx, comment.ID)
}

func (s *MySQLStore) GetComment(ctx context.Context, id string) (Comment, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, thread_id, user_id, parent_comment_id, content, status,
		       reaction_count, created_at, updated_at
		FROM community_comments
		WHERE id = ?
	`, id)
	return scanCommentRow(row)
}

// ── Reactions ────────────────────────────────────────────────────────────────

func (s *MySQLStore) UpsertReaction(ctx context.Context, reaction Reaction) (Reaction, error) {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO community_reactions (id, user_id, target_type, target_id, reaction_type, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE reaction_type = VALUES(reaction_type), status = 'active', updated_at = VALUES(updated_at)
	`, reaction.ID, reaction.UserID, reaction.TargetType, reaction.TargetID,
		reaction.ReactionType, reaction.Status, reaction.CreatedAt, reaction.UpdatedAt)
	if err != nil {
		return Reaction{}, err
	}

	// Update the target's reaction_count
	if err := updateReactionCount(ctx, s.db, reaction.TargetType, reaction.TargetID); err != nil {
		return Reaction{}, err
	}

	// Re-read the reaction
	row := s.db.QueryRowContext(ctx, `
		SELECT id, user_id, target_type, target_id, reaction_type, status, created_at, updated_at
		FROM community_reactions
		WHERE user_id = ? AND target_type = ? AND target_id = ?
	`, reaction.UserID, reaction.TargetType, reaction.TargetID)

	var r Reaction
	var status string
	if err := row.Scan(&r.ID, &r.UserID, &r.TargetType, &r.TargetID, &r.ReactionType, &status, &r.CreatedAt, &r.UpdatedAt); err != nil {
		return Reaction{}, err
	}
	r.Status = status
	return r, nil
}

func (s *MySQLStore) CountReactions(ctx context.Context, targets []ReactionTarget) (map[ReactionTarget]map[string]int, error) {
	result := make(map[ReactionTarget]map[string]int)
	for _, t := range targets {
		result[t] = map[string]int{}
	}

	if len(targets) == 0 {
		return result, nil
	}

	// Build query with OR conditions
	query := "SELECT target_type, target_id, reaction_type, COUNT(*) FROM community_reactions WHERE status = 'active' AND ("
	args := make([]interface{}, 0, len(targets)*2)
	for i, t := range targets {
		if i > 0 {
			query += " OR "
		}
		query += "(target_type = ? AND target_id = ?)"
		args = append(args, t.TargetType, t.TargetID)
	}
	query += ") GROUP BY target_type, target_id, reaction_type"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var targetType, targetID, reactionType string
		var count int
		if err := rows.Scan(&targetType, &targetID, &reactionType, &count); err != nil {
			return nil, err
		}
		t := ReactionTarget{TargetType: targetType, TargetID: targetID}
		if result[t] == nil {
			result[t] = map[string]int{}
		}
		result[t][reactionType] = count
	}
	return result, rows.Err()
}

// ── Attachments ──────────────────────────────────────────────────────────────

func (s *MySQLStore) CreateAttachment(ctx context.Context, attachment Attachment) (Attachment, error) {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO community_attachments
			(id, owner_user_id, target_type, target_id, file_type, storage_provider, storage_key,
			 public_url, mime_type, file_size, width, height, checksum_sha256, status, sort_order, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, attachment.ID, attachment.OwnerUserID, nullableString(attachment.TargetType), nullableString(attachment.TargetID),
		attachment.FileType, attachment.StorageProvider, attachment.StorageKey,
		nullableString(attachment.PublicURL), attachment.MIMEType, attachment.FileSize,
		uintNullable(attachment.Width), uintNullable(attachment.Height),
		attachment.ChecksumSHA256, attachment.Status, attachment.SortOrder,
		attachment.CreatedAt, attachment.UpdatedAt)
	if err != nil {
		return Attachment{}, err
	}

	return s.getAttachment(ctx, attachment.ID)
}

func (s *MySQLStore) getAttachment(ctx context.Context, id string) (Attachment, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, owner_user_id, target_type, target_id, file_type, storage_provider, storage_key,
		       public_url, mime_type, file_size, width, height, checksum_sha256, status, sort_order, created_at, updated_at
		FROM community_attachments
		WHERE id = ?
	`, id)
	return scanAttachmentRow(row)
}

func (s *MySQLStore) BindAttachments(ctx context.Context, ownerUserID, targetType, targetID string, attachmentIDs []string) error {
	return bindAttachmentsTx(ctx, s.db, ownerUserID, targetType, targetID, attachmentIDs)
}

func (s *MySQLStore) ListAttachments(ctx context.Context, targets []AttachmentTarget) (map[AttachmentTarget][]Attachment, error) {
	result := make(map[AttachmentTarget][]Attachment)
	for _, t := range targets {
		result[t] = []Attachment{}
	}

	if len(targets) == 0 {
		return result, nil
	}

	query := `
		SELECT id, owner_user_id, target_type, target_id, file_type, storage_provider, storage_key,
		       public_url, mime_type, file_size, width, height, checksum_sha256, status, sort_order, created_at, updated_at
		FROM community_attachments
		WHERE status = 'active' AND (`
	args := make([]interface{}, 0, len(targets)*2)
	for i, t := range targets {
		if i > 0 {
			query += " OR "
		}
		query += "(target_type = ? AND target_id = ?)"
		args = append(args, t.TargetType, t.TargetID)
	}
	query += ") ORDER BY sort_order, created_at"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		att, err := scanAttachment(rows)
		if err != nil {
			return nil, err
		}
		t := AttachmentTarget{TargetType: att.TargetType, TargetID: att.TargetID}
		if _, ok := result[t]; ok {
			result[t] = append(result[t], att)
		}
	}
	return result, rows.Err()
}

// ── Transaction helpers ──────────────────────────────────────────────────────

// execer is satisfied by both *sql.DB and *sql.Tx.
type execer interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
}

func bindAttachmentsTx(ctx context.Context, ex execer, ownerUserID, targetType, targetID string, attachmentIDs []string) error {
	for _, attID := range attachmentIDs {
		// Verify ownership and status
		var attOwner string
		var attStatus string
		err := ex.QueryRowContext(ctx,
			`SELECT owner_user_id, status FROM community_attachments WHERE id = ?`, attID,
		).Scan(&attOwner, &attStatus)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("%w: attachment %s", ErrAttachmentNotFound, attID)
			}
			return err
		}
		if attOwner != ownerUserID {
			return fmt.Errorf("%w: attachment %s", ErrAttachmentNotOwned, attID)
		}
		if attStatus != StatusPending {
			return fmt.Errorf("%w: attachment %s already bound", ErrAttachmentAlreadyBound, attID)
		}

		_, err = ex.ExecContext(ctx, `
			UPDATE community_attachments
			SET target_type = ?, target_id = ?, status = 'active', updated_at = ?
			WHERE id = ?
		`, targetType, targetID, time.Now().UTC(), attID)
		if err != nil {
			return err
		}
	}
	return nil
}

func updateReactionCount(ctx context.Context, db *sql.DB, targetType, targetID string) error {
	var count int
	err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM community_reactions WHERE target_type = ? AND target_id = ? AND status = 'active'`,
		targetType, targetID,
	).Scan(&count)
	if err != nil {
		return err
	}

	switch targetType {
	case TargetTypeThread:
		_, err = db.ExecContext(ctx, `UPDATE community_threads SET reaction_count = ? WHERE id = ?`, count, targetID)
	case TargetTypeComment:
		_, err = db.ExecContext(ctx, `UPDATE community_comments SET reaction_count = ? WHERE id = ?`, count, targetID)
	}
	return err
}

// ── Scan helpers ─────────────────────────────────────────────────────────────

func scanThread(rows *sql.Rows) (Thread, error) {
	var t Thread
	var optionalBookID sql.NullString
	var lastCommentedAt sql.NullTime
	if err := rows.Scan(
		&t.ID, &t.BoardID, &t.UserID, &t.Title, &t.Content, &optionalBookID,
		&t.Status, &t.CommentCount, &t.ReactionCount, &lastCommentedAt,
		&t.CreatedAt, &t.UpdatedAt,
	); err != nil {
		return Thread{}, err
	}
	t.OptionalBookID = optionalBookID.String
	if lastCommentedAt.Valid {
		t.LastCommentedAt = &lastCommentedAt.Time
	}
	t.ReactionCounts = map[string]int{}
	return t, nil
}

func scanThreadRow(row *sql.Row) (Thread, error) {
	var t Thread
	var optionalBookID sql.NullString
	var lastCommentedAt sql.NullTime
	if err := row.Scan(
		&t.ID, &t.BoardID, &t.UserID, &t.Title, &t.Content, &optionalBookID,
		&t.Status, &t.CommentCount, &t.ReactionCount, &lastCommentedAt,
		&t.CreatedAt, &t.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Thread{}, ErrThreadNotFound
		}
		return Thread{}, err
	}
	t.OptionalBookID = optionalBookID.String
	if lastCommentedAt.Valid {
		t.LastCommentedAt = &lastCommentedAt.Time
	}
	t.ReactionCounts = map[string]int{}
	return t, nil
}

func scanComment(rows *sql.Rows) (Comment, error) {
	var c Comment
	var parentCommentID sql.NullString
	if err := rows.Scan(
		&c.ID, &c.ThreadID, &c.UserID, &parentCommentID, &c.Content,
		&c.Status, &c.ReactionCount, &c.CreatedAt, &c.UpdatedAt,
	); err != nil {
		return Comment{}, err
	}
	c.ParentCommentID = parentCommentID.String
	c.ReactionCounts = map[string]int{}
	return c, nil
}

func scanCommentRow(row *sql.Row) (Comment, error) {
	var c Comment
	var parentCommentID sql.NullString
	if err := row.Scan(
		&c.ID, &c.ThreadID, &c.UserID, &parentCommentID, &c.Content,
		&c.Status, &c.ReactionCount, &c.CreatedAt, &c.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Comment{}, ErrCommentNotFound
		}
		return Comment{}, err
	}
	c.ParentCommentID = parentCommentID.String
	c.ReactionCounts = map[string]int{}
	return c, nil
}

func scanAttachment(rows *sql.Rows) (Attachment, error) {
	var a Attachment
	var targetType, targetID, publicURL sql.NullString
	if err := rows.Scan(
		&a.ID, &a.OwnerUserID, &targetType, &targetID, &a.FileType,
		&a.StorageProvider, &a.StorageKey, &publicURL, &a.MIMEType,
		&a.FileSize, &a.Width, &a.Height, &a.ChecksumSHA256,
		&a.Status, &a.SortOrder, &a.CreatedAt, &a.UpdatedAt,
	); err != nil {
		return Attachment{}, err
	}
	a.TargetType = targetType.String
	a.TargetID = targetID.String
	a.PublicURL = publicURL.String
	return a, nil
}

func scanAttachmentRow(row *sql.Row) (Attachment, error) {
	var a Attachment
	var targetType, targetID, publicURL sql.NullString
	if err := row.Scan(
		&a.ID, &a.OwnerUserID, &targetType, &targetID, &a.FileType,
		&a.StorageProvider, &a.StorageKey, &publicURL, &a.MIMEType,
		&a.FileSize, &a.Width, &a.Height, &a.ChecksumSHA256,
		&a.Status, &a.SortOrder, &a.CreatedAt, &a.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Attachment{}, ErrAttachmentNotFound
		}
		return Attachment{}, err
	}
	a.TargetType = targetType.String
	a.TargetID = targetID.String
	a.PublicURL = publicURL.String
	return a, nil
}

// ── SQL helpers ──────────────────────────────────────────────────────────────

func nullableString(value string) sql.NullString {
	return sql.NullString{String: value, Valid: value != ""}
}

func uintNullable(v uint) sql.NullInt64 {
	if v == 0 {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(v), Valid: true}
}
