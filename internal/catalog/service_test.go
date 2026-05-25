package catalog

import (
	"archive/zip"
	"context"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestServiceListsBooksWithStableIDsAndPagination(t *testing.T) {
	root := t.TempDir()
	writeCatalogEPUB(t, filepath.Join(root, "Beta Book.epub"), "Beta Book", "B. Author", map[string]string{
		"OPS/chapter.xhtml": "<html><body><h1>Start</h1><p>Beta text.</p></body></html>",
	})
	writeCatalogEPUB(t, filepath.Join(root, "Alpha_Book!.epub"), "Alpha Title", "A. Author", map[string]string{
		"OPS/chapter.xhtml": "<html><body><h1>Opening</h1><p>Alpha text.</p></body></html>",
	})

	service := NewService(root)

	items, total, err := service.ListBooks("book", 1, 1)
	if err != nil {
		t.Fatalf("ListBooks() error = %v", err)
	}
	if total != 2 {
		t.Fatalf("total = %d, want 2", total)
	}
	if len(items) != 1 {
		t.Fatalf("items length = %d, want 1", len(items))
	}
	if items[0].ID != "alpha-book" {
		t.Fatalf("first ID = %q, want alpha-book", items[0].ID)
	}
	if items[0].Title != "Alpha Title" || items[0].Author != "A. Author" {
		t.Fatalf("first item = %#v", items[0])
	}
}

func TestServiceReturnsBookAssetManifestAndChapter(t *testing.T) {
	root := t.TempDir()
	bookPath := filepath.Join(root, "Example Book.epub")
	writeCatalogEPUB(t, bookPath, "Example Title", "A. Writer", map[string]string{
		"OPS/chapter-one.xhtml": "<html><body><h1>One</h1><p>Hello &amp; welcome.</p></body></html>",
		"OPS/chapter-two.xhtml": "<html><body><h1>Two</h1><p>Second.</p></body></html>",
	})

	service := NewService(root)

	book, err := service.GetBook("example-book")
	if err != nil {
		t.Fatalf("GetBook() error = %v", err)
	}
	if book.Path != bookPath {
		t.Fatalf("book path = %q, want %q", book.Path, bookPath)
	}

	assetPath, err := service.AssetPath("example-book")
	if err != nil {
		t.Fatalf("AssetPath() error = %v", err)
	}
	if assetPath != bookPath {
		t.Fatalf("asset path = %q, want %q", assetPath, bookPath)
	}

	manifest, err := service.GetManifest("example-book")
	if err != nil {
		t.Fatalf("GetManifest() error = %v", err)
	}
	if manifest.BookID != "example-book" || len(manifest.Chapters) != 2 {
		t.Fatalf("manifest = %#v", manifest)
	}
	if manifest.Chapters[0].ID == "" || manifest.Chapters[0].Title != "One" {
		t.Fatalf("first manifest chapter = %#v", manifest.Chapters[0])
	}

	text, err := service.GetChapter("example-book", manifest.Chapters[0].ID)
	if err != nil {
		t.Fatalf("GetChapter() error = %v", err)
	}
	if text != "One\n\nHello & welcome." {
		t.Fatalf("chapter text = %q", text)
	}
}

func TestServiceUsesStoreWhenCatalogIndexIsAvailable(t *testing.T) {
	root := t.TempDir()
	bookPath := filepath.Join(root, "Indexed Book.epub")
	writeCatalogEPUB(t, bookPath, "Indexed Title", "Indexed Author", map[string]string{
		"OPS/chapter.xhtml": "<html><body><h1>Indexed</h1><p>From disk.</p></body></html>",
	})
	store := &fakeStore{
		books: []Book{{
			ID:           "indexed-book",
			Title:        "Indexed Title",
			Author:       "Indexed Author",
			Filename:     "Indexed Book.epub",
			Path:         bookPath,
			ChapterCount: 1,
		}},
	}

	service := NewServiceWithStore(root, store)

	items, total, err := service.ListBooks("indexed", 1, 20)
	if err != nil {
		t.Fatalf("ListBooks() error = %v", err)
	}
	if total != 1 || len(items) != 1 {
		t.Fatalf("total/items = %d/%d, want 1/1", total, len(items))
	}
	if !store.listCalled {
		t.Fatal("expected indexed store to be used for ListBooks")
	}
	if items[0].ID != "indexed-book" || items[0].ChapterCount != 1 {
		t.Fatalf("indexed item = %#v", items[0])
	}

	assetPath, err := service.AssetPath("indexed-book")
	if err != nil {
		t.Fatalf("AssetPath() error = %v", err)
	}
	if assetPath != bookPath {
		t.Fatalf("asset path = %q, want %q", assetPath, bookPath)
	}
	if !store.getCalled {
		t.Fatal("expected indexed store to be used for AssetPath")
	}
}

func TestServiceFallsBackToDirectoryWhenCatalogIndexIsEmpty(t *testing.T) {
	root := t.TempDir()
	writeCatalogEPUB(t, filepath.Join(root, "Fallback Book.epub"), "Fallback Title", "Fallback Author", map[string]string{
		"OPS/chapter.xhtml": "<html><body><h1>Fallback</h1><p>From scan.</p></body></html>",
	})
	service := NewServiceWithStore(root, &fakeStore{})

	items, total, err := service.ListBooks("fallback", 1, 20)
	if err != nil {
		t.Fatalf("ListBooks() error = %v", err)
	}
	if total != 1 || len(items) != 1 {
		t.Fatalf("total/items = %d/%d, want 1/1", total, len(items))
	}
	if items[0].ID != "fallback-book" {
		t.Fatalf("fallback ID = %q, want fallback-book", items[0].ID)
	}
}

func TestDefaultRootPointsAtBookCityEPUBs(t *testing.T) {
	if got := DefaultRoot(); got != "/Users/jason/Workspace/Code/oops/reader/book-city/epub-books" {
		t.Fatalf("DefaultRoot() = %q", got)
	}
}

type fakeStore struct {
	books      []Book
	listCalled bool
	getCalled  bool
}

func (s *fakeStore) ListBooks(ctx context.Context, query string, limit, offset int) ([]Book, int, error) {
	s.listCalled = true
	return s.books, len(s.books), nil
}

func (s *fakeStore) GetBook(ctx context.Context, id string) (Book, error) {
	s.getCalled = true
	for _, book := range s.books {
		if book.ID == id {
			return book, nil
		}
	}
	return Book{}, ErrNotFound
}

func writeCatalogEPUB(t *testing.T, path, title, author string, chapters map[string]string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	out, err := os.Create(path)
	if err != nil {
		t.Fatalf("create epub: %v", err)
	}
	defer out.Close()

	zw := zip.NewWriter(out)
	mustCreateEntry(t, zw, "META-INF/container.xml", `<?xml version="1.0"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles>
    <rootfile full-path="OPS/package.opf" media-type="application/oebps-package+xml"/>
  </rootfiles>
</container>`)

	itemRefs := ""
	manifest := `<item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>`
	hrefs := make([]string, 0, len(chapters))
	for href := range chapters {
		hrefs = append(hrefs, href)
	}
	sort.Strings(hrefs)
	i := 1
	for _, href := range hrefs {
		id := "c" + string(rune('0'+i))
		manifest += `<item id="` + id + `" href="` + filepath.Base(href) + `" media-type="application/xhtml+xml"/>`
		itemRefs += `<itemref idref="` + id + `"/>`
		i++
	}
	mustCreateEntry(t, zw, "OPS/package.opf", `<?xml version="1.0"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>`+title+`</dc:title>
    <dc:creator>`+author+`</dc:creator>
  </metadata>
  <manifest>`+manifest+`</manifest>
  <spine>`+itemRefs+`</spine>
</package>`)
	for _, href := range hrefs {
		mustCreateEntry(t, zw, href, chapters[href])
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
}

func mustCreateEntry(t *testing.T, zw *zip.Writer, name, body string) {
	t.Helper()
	w, err := zw.Create(name)
	if err != nil {
		t.Fatalf("create zip entry %s: %v", name, err)
	}
	if _, err := w.Write([]byte(body)); err != nil {
		t.Fatalf("write zip entry %s: %v", name, err)
	}
}
