package main

import (
	"context"
	"crypto/sha1"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/oops-reader/oops-reader-backend/internal/catalog"
	"github.com/oops-reader/oops-reader-backend/internal/content"
	"github.com/oops-reader/oops-reader-backend/internal/platform/config"
	"github.com/oops-reader/oops-reader-backend/internal/platform/db"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "catalog-indexer: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	root := catalog.DefaultRoot()
	if len(os.Args) > 1 && strings.TrimSpace(os.Args[1]) != "" {
		root = os.Args[1]
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	dbPool, err := db.NewMySQL(cfg)
	if err != nil {
		return err
	}
	defer dbPool.Close()

	paths, err := epubPaths(root)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	imported := 0
	for _, path := range paths {
		if err := upsertBook(ctx, dbPool, root, path); err != nil {
			return err
		}
		imported++
	}
	fmt.Printf("indexed %d EPUB books from %s\n", imported, root)
	return nil
}

func epubPaths(root string) ([]string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".epub") {
			continue
		}
		paths = append(paths, filepath.Join(root, entry.Name()))
	}
	sort.Strings(paths)
	return paths, nil
}

func upsertBook(ctx context.Context, dbPool *sql.DB, root, path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	sum, err := fileSHA1(path)
	if err != nil {
		return err
	}

	filename := filepath.Base(path)
	title := strings.TrimSuffix(filename, filepath.Ext(filename))
	author := ""
	chapterCount := 0
	if parsed, err := content.ParseEPUB(path); err == nil {
		if parsed.Title != "" {
			title = parsed.Title
		}
		author = parsed.Author
		chapterCount = len(parsed.Chapters)
	}

	storagePath, err := filepath.Rel(root, path)
	if err != nil || strings.HasPrefix(storagePath, "..") {
		storagePath = path
	}

	_, err = dbPool.ExecContext(ctx, `
INSERT INTO catalog_books
    (book_key, title, author, filename, storage_path, file_size, content_sha1, chapter_count, status, indexed_at)
VALUES
    (?, ?, ?, ?, ?, ?, ?, ?, 1, NOW())
ON DUPLICATE KEY UPDATE
    title = VALUES(title),
    author = VALUES(author),
    filename = VALUES(filename),
    storage_path = VALUES(storage_path),
    file_size = VALUES(file_size),
    chapter_count = VALUES(chapter_count),
    status = 1,
    indexed_at = NOW()`,
		catalog.IDFromFilename(filename),
		title,
		nullable(author),
		filename,
		storagePath,
		info.Size(),
		sum,
		chapterCount,
	)
	if err != nil {
		return fmt.Errorf("index %s: %w", filename, err)
	}
	return nil
}

func fileSHA1(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha1.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func nullable(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}
