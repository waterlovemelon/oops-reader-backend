package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/oops-reader/oops-reader-backend/internal/catalog"
)

func TestCoverServesSelectedVariantWithImmutableCacheHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	root := t.TempDir()
	coverPath := writeHandlerCoverVariants(t, root)
	service := catalog.NewServiceWithStore(root, &coverHandlerStore{book: catalog.Book{
		ID:               "book",
		CoverStoragePath: coverPath,
	}})
	handler := NewCatalogHandler(service, nil, nil)
	router := gin.New()
	router.GET("/v1/catalog/books/:id/cover", handler.Cover)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/catalog/books/book/cover?width_px=500", nil)
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if got := recorder.Body.String(); got != "medium-image" {
		t.Fatalf("body = %q, want medium-image", got)
	}
	if got := recorder.Header().Get("Cache-Control"); got != "public, max-age=31536000, immutable" {
		t.Fatalf("Cache-Control = %q", got)
	}
	if got := recorder.Header().Get("Content-Location"); got != "/v1/catalog/books/book/cover?width_px=640" {
		t.Fatalf("Content-Location = %q", got)
	}
	if got := recorder.Header().Get("X-Image-Variant"); got != "medium" {
		t.Fatalf("X-Image-Variant = %q", got)
	}
	if got := recorder.Header().Get("X-Image-Width"); got != "640" {
		t.Fatalf("X-Image-Width = %q", got)
	}
	if got := recorder.Header().Get("Content-Type"); got != "image/jpeg" {
		t.Fatalf("Content-Type = %q", got)
	}
	etag := recorder.Header().Get("ETag")
	if etag == "" {
		t.Fatal("ETag is empty")
	}

	conditionalRecorder := httptest.NewRecorder()
	conditionalRequest := httptest.NewRequest(http.MethodGet, "/v1/catalog/books/book/cover?width_px=500", nil)
	conditionalRequest.Header.Set("If-None-Match", etag)
	router.ServeHTTP(conditionalRecorder, conditionalRequest)
	if conditionalRecorder.Code != http.StatusNotModified {
		t.Fatalf("conditional status = %d, want 304", conditionalRecorder.Code)
	}

	rangeRecorder := httptest.NewRecorder()
	rangeRequest := httptest.NewRequest(http.MethodGet, "/v1/catalog/books/book/cover?width_px=500", nil)
	rangeRequest.Header.Set("Range", "bytes=0-5")
	router.ServeHTTP(rangeRecorder, rangeRequest)
	if rangeRecorder.Code != http.StatusPartialContent {
		t.Fatalf("range status = %d, want 206", rangeRecorder.Code)
	}
	if got := rangeRecorder.Body.String(); got != "medium" {
		t.Fatalf("range body = %q, want medium", got)
	}
}

func TestCoverRequiresPositiveWidth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := catalog.NewServiceWithStore(t.TempDir(), &coverHandlerStore{book: catalog.Book{ID: "book"}})
	handler := NewCatalogHandler(service, nil, nil)
	router := gin.New()
	router.GET("/v1/catalog/books/:id/cover", handler.Cover)

	for _, path := range []string{
		"/v1/catalog/books/book/cover",
		"/v1/catalog/books/book/cover?width_px=0",
		"/v1/catalog/books/book/cover?width_px=320px",
	} {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("%s status = %d, want 400", path, recorder.Code)
		}
	}
}

func writeHandlerCoverVariants(t *testing.T, root string) string {
	t.Helper()
	coverPath := filepath.ToSlash(filepath.Join("covers", "epub", "ab", "cd", "book", "small-cover.jpg"))
	directory := filepath.Join(root, filepath.Dir(coverPath))
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	entries := ""
	for index, fixture := range []struct {
		key    string
		width  int
		height int
		data   string
	}{
		{key: "small", width: 320, height: 480, data: "small-image"},
		{key: "medium", width: 640, height: 960, data: "medium-image"},
		{key: "large", width: 1280, height: 1920, data: "large-image"},
	} {
		filename := fixture.key + ".jpg"
		if err := os.WriteFile(filepath.Join(directory, filename), []byte(fixture.data), 0o644); err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256([]byte(fixture.data))
		if index > 0 {
			entries += ","
		}
		entries += `{"key":"` + fixture.key + `","width":` + itoaForCoverTest(fixture.width) + `,"height":` + itoaForCoverTest(fixture.height) + `,"path":"` + filename + `","media_type":"image/jpeg","sha256":"` + hex.EncodeToString(digest[:]) + `"}`
	}
	manifest := `{"version":1,"variants":[` + entries + `]}`
	if err := os.WriteFile(filepath.Join(directory, "variants.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	return coverPath
}

func itoaForCoverTest(value int) string {
	if value == 0 {
		return "0"
	}
	var digits [20]byte
	i := len(digits)
	for value > 0 {
		i--
		digits[i] = byte('0' + value%10)
		value /= 10
	}
	return string(digits[i:])
}

type coverHandlerStore struct {
	book catalog.Book
}

func (s *coverHandlerStore) ListBooks(context.Context, string, int, int) ([]catalog.Book, int, error) {
	return []catalog.Book{s.book}, 1, nil
}

func (s *coverHandlerStore) GetBook(context.Context, string) (catalog.Book, error) {
	return s.book, nil
}
