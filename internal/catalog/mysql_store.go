package catalog

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

type MySQLStore struct {
	db *sql.DB
}

func NewMySQLStore(db *sql.DB) *MySQLStore {
	return &MySQLStore{db: db}
}

func (s *MySQLStore) ListBooks(ctx context.Context, query string, limit, offset int) ([]Book, int, error) {
	if limit < 1 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	where := "WHERE status = 1"
	args := []any{}
	query = strings.TrimSpace(query)
	if query != "" {
		where += " AND (LOWER(title) LIKE ? OR LOWER(author) LIKE ? OR LOWER(filename) LIKE ? OR LOWER(book_key) LIKE ?)"
		like := "%" + strings.ToLower(query) + "%"
		args = append(args, like, like, like, like)
	}

	var total int
	countSQL := "SELECT COUNT(*) FROM catalog_books " + where
	if err := s.db.QueryRowContext(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count catalog books: %w", err)
	}
	if total == 0 {
		return []Book{}, 0, nil
	}

	listArgs := append(append([]any{}, args...), limit, offset)
	rows, err := s.db.QueryContext(ctx, `
SELECT book_key, title, author, filename, storage_path, language, chapter_count, file_size, content_sha1
FROM catalog_books `+where+`
ORDER BY title ASC, book_key ASC
LIMIT ? OFFSET ?`, listArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list catalog books: %w", err)
	}
	defer rows.Close()

	books := []Book{}
	for rows.Next() {
		book, err := scanBook(rows)
		if err != nil {
			return nil, 0, err
		}
		books = append(books, book)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate catalog books: %w", err)
	}
	return books, total, nil
}

func (s *MySQLStore) GetBook(ctx context.Context, id string) (Book, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT book_key, title, author, filename, storage_path, language, chapter_count, file_size, content_sha1
FROM catalog_books
WHERE book_key = ? AND status = 1`, id)
	book, err := scanBook(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Book{}, ErrNotFound
	}
	if err != nil {
		return Book{}, fmt.Errorf("get catalog book: %w", err)
	}
	return book, nil
}

type bookScanner interface {
	Scan(dest ...any) error
}

func scanBook(scanner bookScanner) (Book, error) {
	var book Book
	var author, language, sha1 sql.NullString
	var chapterCount sql.NullInt64
	if err := scanner.Scan(
		&book.ID,
		&book.Title,
		&author,
		&book.Filename,
		&book.Path,
		&language,
		&chapterCount,
		&book.FileSize,
		&sha1,
	); err != nil {
		return Book{}, err
	}
	book.Author = author.String
	book.Language = language.String
	book.ChapterCount = int(chapterCount.Int64)
	book.ContentSHA1 = sha1.String
	return book, nil
}
