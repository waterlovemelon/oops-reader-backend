package catalog

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

type readingStore struct{ book Book }

func (s readingStore) ListBooks(context.Context, string, int, int) ([]Book, int, error) {
	return []Book{s.book}, 1, nil
}
func (s readingStore) GetBook(context.Context, string) (Book, error) { return s.book, nil }

func TestReadingArtifactsAcceptNestedSegmentsAndRejectTraversal(t *testing.T) {
	root := t.TempDir()
	bookPath := filepath.Join(root, "book.epub")
	if err := os.WriteFile(bookPath, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	reading := filepath.Join(root, ".reading", "b", "v1", "segments")
	if err := os.MkdirAll(reading, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".reading", "b", "v1", "manifest.json"), []byte(`{"book_id":"b","content_version":"v1","chapters":[]}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(reading, "s.json"), []byte(`{"segment_id":"s"}`), 0600); err != nil {
		t.Fatal(err)
	}
	svc := NewServiceWithStore(root, readingStore{book: Book{ID: "b", Path: bookPath}})
	if body, _, err := svc.LoadReadingFile("b", "v1", "segments/s.json"); err != nil || string(body) != "{\"segment_id\":\"s\"}" {
		t.Fatalf("nested asset: %v %s", err, body)
	}
	if _, _, err := svc.LoadReadingFile("b", "v1", "segments/../manifest.json"); err == nil {
		t.Fatal("traversal accepted")
	}
}
