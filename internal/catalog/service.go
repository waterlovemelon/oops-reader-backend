package catalog

import (
	"context"
	"crypto/sha1"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/oops-reader/oops-reader-backend/internal/content"
)

const defaultRoot = "/Users/jason/Workspace/Code/oops/reader/book-city/epub-books"
const catalogRootEnv = "OOPS_READER_CATALOG_ROOT"

var ErrNotFound = errors.New("not found")

type Service struct {
	root  string
	store Store
}

type Book struct {
	ID               string
	Title            string
	Author           string
	Description      string
	Format           string
	Filename         string
	Path             string
	CoverStoragePath string
	Language         string
	ChapterCount     int
	WordCount        int64
	FileSize         int64
	ContentSHA1      string
}

type Store interface {
	ListBooks(ctx context.Context, query string, limit, offset int) ([]Book, int, error)
	GetBook(ctx context.Context, id string) (Book, error)
}

type Manifest struct {
	BookID   string
	Title    string
	Author   string
	Chapters []Chapter
}

type Cover struct {
	MediaType string
	Data      []byte
}

type Chapter struct {
	ID    string
	Title string
	Href  string
}

func DefaultRoot() string {
	if root := strings.TrimSpace(os.Getenv(catalogRootEnv)); root != "" {
		return root
	}
	return defaultRoot
}

func NewService(root string) *Service {
	return NewServiceWithStore(root, nil)
}

func NewServiceWithDB(root string, db *sql.DB) *Service {
	var store Store
	if db != nil {
		store = NewMySQLStore(db)
	}
	return NewServiceWithStore(root, store)
}

func NewServiceWithStore(root string, store Store) *Service {
	if strings.TrimSpace(root) == "" {
		root = defaultRoot
	}
	return &Service{root: root, store: store}
}

func (s *Service) ListBooks(query string, page, pageSize int) ([]Book, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if s.store != nil {
		books, total, err := s.store.ListBooks(context.Background(), query, pageSize, (page-1)*pageSize)
		if err == nil && total > 0 {
			return books, total, nil
		}
		if err != nil && !errors.Is(err, ErrNotFound) {
			return nil, 0, err
		}
	}

	books, err := s.scan()
	if err != nil {
		return nil, 0, err
	}

	query = strings.ToLower(strings.TrimSpace(query))
	if query != "" {
		filtered := books[:0]
		for _, book := range books {
			haystack := strings.ToLower(book.Title + " " + book.Author + " " + book.Filename + " " + book.ID)
			if strings.Contains(haystack, query) {
				filtered = append(filtered, book)
			}
		}
		books = filtered
	}

	total := len(books)
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	start := (page - 1) * pageSize
	if start >= total {
		return []Book{}, total, nil
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	return books[start:end], total, nil
}

func (s *Service) GetBook(id string) (*Book, error) {
	book, err := s.findBook(id)
	if err != nil {
		return nil, err
	}
	return &book, nil
}

func (s *Service) AssetPath(id string) (string, error) {
	book, err := s.findBook(id)
	if err != nil {
		return "", err
	}
	return book.Path, nil
}

func (s *Service) OpenAssetPath(id string) (string, error) {
	return s.AssetPath(id)
}

func (s *Service) GetManifest(id string) (*Manifest, error) {
	book, parsed, err := s.parseBook(id)
	if err != nil {
		return nil, err
	}
	manifest := &Manifest{
		BookID: book.ID,
		Title:  parsed.Title,
		Author: parsed.Author,
	}
	if manifest.Title == "" {
		manifest.Title = book.Title
	}
	if manifest.Author == "" {
		manifest.Author = book.Author
	}
	for _, chapter := range parsed.Chapters {
		manifest.Chapters = append(manifest.Chapters, Chapter{
			ID:    chapter.ID,
			Title: chapter.Title,
			Href:  chapter.Href,
		})
	}
	return manifest, nil
}

func (s *Service) GetChapter(bookID, chapterID string) (string, error) {
	_, parsed, err := s.parseBook(bookID)
	if err != nil {
		return "", err
	}
	for _, chapter := range parsed.Chapters {
		if chapter.ID == chapterID {
			return chapter.Text, nil
		}
	}
	return "", fmt.Errorf("%w: chapter %s", ErrNotFound, chapterID)
}

func (s *Service) GetCover(id string) (*Cover, error) {
	book, err := s.findBook(id)
	if err != nil {
		return nil, err
	}
	cover, err := content.ExtractEPUBCover(book.Path)
	if err != nil {
		return nil, err
	}
	if cover == nil {
		return nil, fmt.Errorf("%w: cover for %s", ErrNotFound, id)
	}
	return &Cover{
		MediaType: cover.MediaType,
		Data:      cover.Data,
	}, nil
}

func (s *Service) parseBook(id string) (Book, *content.Book, error) {
	book, err := s.findBook(id)
	if err != nil {
		return Book{}, nil, err
	}
	parsed, err := content.ParseEPUB(book.Path)
	if err != nil {
		return Book{}, nil, err
	}
	return book, parsed, nil
}

func (s *Service) findBook(id string) (Book, error) {
	if s.store != nil {
		book, err := s.store.GetBook(context.Background(), id)
		if err == nil {
			book.Path = s.resolvePath(book.Path)
			return book, nil
		}
		if !errors.Is(err, ErrNotFound) {
			return Book{}, err
		}
	}

	books, err := s.scan()
	if err != nil {
		return Book{}, err
	}
	for _, book := range books {
		if book.ID == id {
			return book, nil
		}
	}
	return Book{}, fmt.Errorf("%w: book %s", ErrNotFound, id)
}

func (s *Service) resolvePath(value string) string {
	if value == "" || filepath.IsAbs(value) {
		return value
	}
	return filepath.Join(s.root, value)
}

func (s *Service) scan() ([]Book, error) {
	entries, err := os.ReadDir(s.root)
	if err != nil {
		return nil, err
	}

	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".epub") {
			continue
		}
		paths = append(paths, filepath.Join(s.root, entry.Name()))
	}
	sort.Strings(paths)

	usedIDs := make(map[string]int, len(paths))
	books := make([]Book, 0, len(paths))
	for _, filePath := range paths {
		filename := filepath.Base(filePath)
		id := IDFromFilename(filename)
		if usedIDs[id] > 0 {
			id = id + "-" + shortHash(filename)
		}
		usedIDs[id]++

		title := strings.TrimSuffix(filename, filepath.Ext(filename))
		author := ""
		parsed, err := content.ParseEPUB(filePath)
		if err == nil {
			if parsed.Title != "" {
				title = parsed.Title
			}
			author = parsed.Author
		}
		chapterCount := 0
		if parsed != nil {
			chapterCount = len(parsed.Chapters)
		}
		books = append(books, Book{
			ID:           id,
			Title:        title,
			Author:       author,
			Filename:     filename,
			Path:         filePath,
			ChapterCount: chapterCount,
		})
	}

	sort.SliceStable(books, func(i, j int) bool {
		return strings.ToLower(books[i].Title) < strings.ToLower(books[j].Title)
	})
	return books, nil
}

func IDFromFilename(filename string) string {
	base := strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
	id := slug(base)
	if id == "" {
		return "book-" + shortHash(base)
	}
	return id
}

func shortHash(value string) string {
	sum := sha1.Sum([]byte(value))
	return hex.EncodeToString(sum[:])[:10]
}

func slug(value string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(value) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash && b.Len() > 0 {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}
