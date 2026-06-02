package catalog

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// MySQLRecommendationStore implements RecommendationStore using MySQL.
type MySQLRecommendationStore struct {
	db *sql.DB
}

// NewMySQLRecommendationStore creates a new MySQLRecommendationStore.
func NewMySQLRecommendationStore(db *sql.DB) *MySQLRecommendationStore {
	return &MySQLRecommendationStore{db: db}
}

// CurrentRecommendation returns the most recent published recommendation, or nil if none.
func (s *MySQLRecommendationStore) CurrentRecommendation(ctx context.Context, now time.Time) (*Recommendation, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT r.id, r.book_key, r.comment, r.scheduled_publish_at, r.created_at, r.updated_at,
       b.book_key, b.title, b.author, b.description, b.format, b.filename, b.storage_path,
       b.cover_storage_path, b.language, b.chapter_count, b.word_count, b.file_size, b.content_sha1
FROM catalog_book_recommendations r
JOIN catalog_books b ON b.book_key = r.book_key
WHERE r.status = 'active'
  AND r.scheduled_publish_at <= ?
  AND r.deleted_at IS NULL
  AND (b.status = 'active' OR b.status = '1')
ORDER BY r.scheduled_publish_at DESC, r.id DESC
LIMIT 1`, now)

	rec, err := scanRecommendation(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("current recommendation: %w", err)
	}
	return &rec, nil
}

// ListPublishedRecommendations returns a paginated list of published recommendations.
func (s *MySQLRecommendationStore) ListPublishedRecommendations(ctx context.Context, now time.Time, limit, offset int) ([]Recommendation, int, error) {
	if limit < 1 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	var total int
	err := s.db.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM catalog_book_recommendations r
JOIN catalog_books b ON b.book_key = r.book_key
WHERE r.status = 'active'
  AND r.scheduled_publish_at <= ?
  AND r.deleted_at IS NULL
  AND (b.status = 'active' OR b.status = '1')`, now).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count recommendations: %w", err)
	}
	if total == 0 {
		return []Recommendation{}, 0, nil
	}

	rows, err := s.db.QueryContext(ctx, `
SELECT r.id, r.book_key, r.comment, r.scheduled_publish_at, r.created_at, r.updated_at,
       b.book_key, b.title, b.author, b.description, b.format, b.filename, b.storage_path,
       b.cover_storage_path, b.language, b.chapter_count, b.word_count, b.file_size, b.content_sha1
FROM catalog_book_recommendations r
JOIN catalog_books b ON b.book_key = r.book_key
WHERE r.status = 'active'
  AND r.scheduled_publish_at <= ?
  AND r.deleted_at IS NULL
  AND (b.status = 'active' OR b.status = '1')
ORDER BY r.scheduled_publish_at DESC, r.id DESC
LIMIT ? OFFSET ?`, now, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list recommendations: %w", err)
	}
	defer rows.Close()

	recs := []Recommendation{}
	for rows.Next() {
		rec, err := scanRecommendationRow(rows)
		if err != nil {
			return nil, 0, err
		}
		recs = append(recs, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate recommendations: %w", err)
	}
	return recs, total, nil
}

type recommendationScanner interface {
	Scan(dest ...any) error
}

func scanRecommendation(scanner recommendationScanner) (Recommendation, error) {
	var rec Recommendation
	var book Book
	var author, description, format, coverPath, language, sha1 sql.NullString
	var chapterCount, wordCount sql.NullInt64
	if err := scanner.Scan(
		&rec.ID, &rec.BookKey, &rec.Comment, &rec.ScheduledPublishAt, &rec.CreatedAt, &rec.UpdatedAt,
		&book.ID, &book.Title, &author, &description, &format, &book.Filename, &book.Path,
		&coverPath, &language, &chapterCount, &wordCount, &book.FileSize, &sha1,
	); err != nil {
		return Recommendation{}, err
	}
	book.Author = author.String
	book.Description = description.String
	book.Format = format.String
	book.CoverStoragePath = coverPath.String
	book.Language = language.String
	book.ChapterCount = int(chapterCount.Int64)
	book.WordCount = wordCount.Int64
	book.ContentSHA1 = sha1.String
	rec.Book = book
	return rec, nil
}

func scanRecommendationRow(rows *sql.Rows) (Recommendation, error) {
	return scanRecommendation(rows)
}
