package catalog

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"
)

// MySQLCommentsStore implements CommentsStore using MySQL.
type MySQLCommentsStore struct {
	db *sql.DB
}

// NewMySQLCommentsStore creates a new MySQLCommentsStore.
func NewMySQLCommentsStore(db *sql.DB) *MySQLCommentsStore {
	return &MySQLCommentsStore{db: db}
}

// ListComments returns published comments for a catalog book, ordered by creation time descending.
func (s *MySQLCommentsStore) ListComments(ctx context.Context, bookKey string, limit, offset int) ([]BookComment, int, error) {
	if limit < 1 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	var total int
	err := s.db.QueryRowContext(ctx, `
SELECT COUNT(*) FROM catalog_book_comments
WHERE catalog_book_key = ? AND status = 'published' AND deleted_at IS NULL`,
		bookKey,
	).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count comments: %w", err)
	}
	if total == 0 {
		return []BookComment{}, 0, nil
	}

	rows, err := s.db.QueryContext(ctx, `
SELECT c.comment_id, c.catalog_book_key, c.user_id, c.source, c.external_source,
       c.author_name, c.content, c.like_count, c.status, c.created_at,
       COALESCE(u.nickname, c.author_name, '') AS display_name
FROM catalog_book_comments c
LEFT JOIN users u ON u.id = c.user_id
WHERE c.catalog_book_key = ? AND c.status = 'published' AND c.deleted_at IS NULL
ORDER BY c.created_at DESC
LIMIT ? OFFSET ?`,
		bookKey, limit, offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list comments: %w", err)
	}
	defer rows.Close()

	comments := []BookComment{}
	for rows.Next() {
		var c BookComment
		var userID sql.NullInt64
		var extSource, authorName sql.NullString
		var createdAt time.Time
		if err := rows.Scan(
			&c.ID, &c.BookID, &userID, &c.Source, &extSource,
			&authorName, &c.Content, &c.LikeCount, &c.Status, &createdAt,
			&c.DisplayName,
		); err != nil {
			return nil, 0, fmt.Errorf("scan comment: %w", err)
		}
		c.UserID = uint64(userID.Int64)
		c.ExternalSource = extSource.String
		c.AuthorName = authorName.String
		c.CreatedAt = createdAt
		comments = append(comments, c)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate comments: %w", err)
	}
	return comments, total, nil
}

// CreateComment creates a new user comment for a catalog book.
func (s *MySQLCommentsStore) CreateComment(ctx context.Context, comment BookComment) (BookComment, error) {
	commentID := generateCommentID()
	now := time.Now()
	_, err := s.db.ExecContext(ctx, `
INSERT INTO catalog_book_comments
    (comment_id, catalog_book_key, user_id, source, content, status, created_at, updated_at)
VALUES (?, ?, ?, 'user', ?, 'published', ?, ?)`,
		commentID, comment.BookID, comment.UserID, comment.Content, now, now,
	)
	if err != nil {
		return BookComment{}, fmt.Errorf("create comment: %w", err)
	}
	// Query back the display name from users table.
	var displayName sql.NullString
	_ = s.db.QueryRowContext(ctx,
		`SELECT nickname FROM users WHERE id = ?`, comment.UserID,
	).Scan(&displayName)

	comment.ID = commentID
	comment.Source = "user"
	comment.Status = "published"
	comment.DisplayName = displayName.String
	comment.CreatedAt = now
	return comment, nil
}

// CountPublished returns the count of published comments for a catalog book.
func (s *MySQLCommentsStore) CountPublished(ctx context.Context, bookKey string) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `
SELECT COUNT(*) FROM catalog_book_comments
WHERE catalog_book_key = ? AND status = 'published' AND deleted_at IS NULL`,
		bookKey,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count published comments: %w", err)
	}
	return count, nil
}

func generateCommentID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// Fallback: use timestamp-based ID if crypto/rand fails.
		return fmt.Sprintf("cmt-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}
