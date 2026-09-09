package catalog

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetCoverVariantSelectsNearestAndPrefersLargerOnTie(t *testing.T) {
	root := t.TempDir()
	coverPath := writeCoverVariants(t, root, map[string]variantFixture{
		"small":  {width: 320, height: 480, data: "small"},
		"medium": {width: 640, height: 960, data: "medium"},
		"large":  {width: 1280, height: 1920, data: "large"},
	})
	service := NewServiceWithStore(root, &imageVariantFakeStore{book: Book{ID: "book", CoverStoragePath: coverPath}})

	for _, tc := range []struct {
		width int
		want  string
	}{
		{width: 294, want: "small"},
		{width: 500, want: "medium"},
		{width: 960, want: "large"},
		{width: 2000, want: "large"},
	} {
		t.Run(tc.want, func(t *testing.T) {
			variant, err := service.GetCoverVariant("book", tc.width)
			if err != nil {
				t.Fatalf("GetCoverVariant() error = %v", err)
			}
			if variant.Key != tc.want {
				t.Fatalf("GetCoverVariant(%d) = %q, want %q", tc.width, variant.Key, tc.want)
			}
		})
	}
}

func TestGetCoverVariantDoesNotUseEPUBFallbackWhenManifestMissing(t *testing.T) {
	root := t.TempDir()
	service := NewServiceWithStore(root, &imageVariantFakeStore{book: Book{
		ID:               "book",
		CoverStoragePath: "covers/epub/ab/cd/book/small-cover.jpg",
	}})

	_, err := service.GetCoverVariant("book", 320)
	if !strings.Contains(err.Error(), ErrImageVariantNotFound.Error()) {
		t.Fatalf("GetCoverVariant() error = %v, want image variant not found", err)
	}
}

func TestGetCoverVariantRejectsManifestPathTraversal(t *testing.T) {
	root := t.TempDir()
	coverPath := filepath.ToSlash(filepath.Join("covers", "epub", "ab", "cd", "book", "small-cover.jpg"))
	directory := filepath.Join(root, filepath.Dir(coverPath))
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `{"version":1,"variants":[{"key":"small","width":320,"height":480,"path":"../outside.jpg","media_type":"image/jpeg","sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}]}`
	if err := os.WriteFile(filepath.Join(directory, coverVariantsManifestName), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	service := NewServiceWithStore(root, &imageVariantFakeStore{book: Book{ID: "book", CoverStoragePath: coverPath}})

	_, err := service.GetCoverVariant("book", 320)
	if !strings.Contains(err.Error(), ErrInvalidImageVariant.Error()) {
		t.Fatalf("GetCoverVariant() error = %v, want invalid image variant", err)
	}
}

type variantFixture struct {
	width  int
	height int
	data   string
}

func writeCoverVariants(t *testing.T, root string, variants map[string]variantFixture) string {
	t.Helper()
	coverPath := filepath.ToSlash(filepath.Join("covers", "epub", "ab", "cd", "book", "small-cover.jpg"))
	directory := filepath.Join(root, filepath.Dir(coverPath))
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	entries := make([]string, 0, len(variants))
	for index, key := range []string{"small", "medium", "large"} {
		variant, ok := variants[key]
		if !ok {
			continue
		}
		filename := key + "-cover.jpg"
		if err := os.WriteFile(filepath.Join(directory, filename), []byte(variant.data), 0o644); err != nil {
			t.Fatal(err)
		}
		entries = append(entries, `{"key":"`+key+`","width":`+itoa(variant.width)+`,"height":`+itoa(variant.height)+`,"path":"`+filename+`","media_type":"image/jpeg","sha256":"`+strings.Repeat(string(rune('a'+index)), 64)+`"}`)
	}
	manifest := `{"version":1,"variants":[` + strings.Join(entries, ",") + `]}`
	if err := os.WriteFile(filepath.Join(directory, coverVariantsManifestName), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	return coverPath
}

func itoa(value int) string {
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

type imageVariantFakeStore struct {
	book Book
}

func (s *imageVariantFakeStore) ListBooks(context.Context, string, int, int) ([]Book, int, error) {
	return []Book{s.book}, 1, nil
}

func (s *imageVariantFakeStore) GetBook(context.Context, string) (Book, error) {
	return s.book, nil
}
