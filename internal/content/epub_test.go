package content

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func TestParseEPUBUsesOPFMetadataAndSpineChapters(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Example Book.epub")
	writeTestEPUB(t, path, map[string]string{
		"META-INF/container.xml": `<?xml version="1.0"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles>
    <rootfile full-path="OPS/package.opf" media-type="application/oebps-package+xml"/>
  </rootfiles>
</container>`,
		"OPS/package.opf": `<?xml version="1.0"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>Example Title</dc:title>
    <dc:creator>A. Writer</dc:creator>
  </metadata>
  <manifest>
    <item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>
    <item id="c1" href="chapter-one.xhtml" media-type="application/xhtml+xml"/>
    <item id="c2" href="nested/chapter-two.html" media-type="text/html"/>
  </manifest>
  <spine>
    <itemref idref="c1"/>
    <itemref idref="c2"/>
  </spine>
</package>`,
		"OPS/chapter-one.xhtml":       `<html><head><title>Ignored</title></head><body><h1>One</h1><p>Hello &amp; welcome.</p></body></html>`,
		"OPS/nested/chapter-two.html": `<html><body><h2>Two</h2><p>Second&nbsp;chapter<br/>line.</p></body></html>`,
	})

	book, err := ParseEPUB(path)
	if err != nil {
		t.Fatalf("ParseEPUB() error = %v", err)
	}

	if book.Title != "Example Title" {
		t.Fatalf("Title = %q, want Example Title", book.Title)
	}
	if book.Author != "A. Writer" {
		t.Fatalf("Author = %q, want A. Writer", book.Author)
	}
	if len(book.Chapters) != 2 {
		t.Fatalf("chapter count = %d, want 2", len(book.Chapters))
	}
	if book.Chapters[0].ID != "c1" || book.Chapters[0].Title != "One" {
		t.Fatalf("first chapter = %#v, want ID c1 and title One", book.Chapters[0])
	}
	if got := book.Chapters[0].Text; got != "One\n\nHello & welcome." {
		t.Fatalf("first chapter text = %q", got)
	}
	if got := book.Chapters[1].Text; got != "Two\n\nSecond chapter\nline." {
		t.Fatalf("second chapter text = %q", got)
	}
}

func TestParseEPUBFallsBackToSortedReadableEntries(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Loose.epub")
	writeTestEPUB(t, path, map[string]string{
		"text/b.xhtml": `<html><body><p>Beta</p></body></html>`,
		"text/a.xhtml": `<html><body><p>Alpha</p></body></html>`,
	})

	book, err := ParseEPUB(path)
	if err != nil {
		t.Fatalf("ParseEPUB() error = %v", err)
	}

	if len(book.Chapters) != 2 {
		t.Fatalf("chapter count = %d, want 2", len(book.Chapters))
	}
	if book.Chapters[0].ID != "text-a-xhtml" || book.Chapters[0].Text != "Alpha" {
		t.Fatalf("first fallback chapter = %#v", book.Chapters[0])
	}
	if book.Chapters[1].ID != "text-b-xhtml" || book.Chapters[1].Text != "Beta" {
		t.Fatalf("second fallback chapter = %#v", book.Chapters[1])
	}
}

func TestParseEPUBUsesNavWhenSpineIsMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Nav Only.epub")
	writeTestEPUB(t, path, map[string]string{
		"META-INF/container.xml": `<?xml version="1.0"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles><rootfile full-path="OPS/package.opf"/></rootfiles>
</container>`,
		"OPS/package.opf": `<?xml version="1.0"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
  <manifest>
    <item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>
    <item id="second" href="second.xhtml" media-type="application/xhtml+xml"/>
    <item id="first" href="first.xhtml" media-type="application/xhtml+xml"/>
  </manifest>
</package>`,
		"OPS/nav.xhtml":    `<html><body><nav epub:type="toc"><ol><li><a href="first.xhtml">From Nav First</a></li><li><a href="second.xhtml">From Nav Second</a></li></ol></nav></body></html>`,
		"OPS/first.xhtml":  `<html><body><p>First body.</p></body></html>`,
		"OPS/second.xhtml": `<html><body><p>Second body.</p></body></html>`,
	})

	book, err := ParseEPUB(path)
	if err != nil {
		t.Fatalf("ParseEPUB() error = %v", err)
	}

	if len(book.Chapters) != 2 {
		t.Fatalf("chapter count = %d, want 2", len(book.Chapters))
	}
	if book.Chapters[0].ID != "first" || book.Chapters[0].Title != "From Nav First" {
		t.Fatalf("first nav chapter = %#v", book.Chapters[0])
	}
	if book.Chapters[1].ID != "second" || book.Chapters[1].Title != "From Nav Second" {
		t.Fatalf("second nav chapter = %#v", book.Chapters[1])
	}
}

func TestParseEPUBUsesNCXWhenSpineIsMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "NCX Only.epub")
	writeTestEPUB(t, path, map[string]string{
		"META-INF/container.xml": `<?xml version="1.0"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles><rootfile full-path="OPS/package.opf"/></rootfiles>
</container>`,
		"OPS/package.opf": `<?xml version="1.0"?>
<package xmlns="http://www.idpf.org/2007/opf" version="2.0">
  <manifest>
    <item id="toc" href="toc.ncx" media-type="application/x-dtbncx+xml"/>
    <item id="first" href="first.xhtml" media-type="application/xhtml+xml"/>
  </manifest>
</package>`,
		"OPS/toc.ncx":     `<ncx><navMap><navPoint><navLabel><text>NCX First</text></navLabel><content src="first.xhtml"/></navPoint></navMap></ncx>`,
		"OPS/first.xhtml": `<html><body><p>First body.</p></body></html>`,
	})

	book, err := ParseEPUB(path)
	if err != nil {
		t.Fatalf("ParseEPUB() error = %v", err)
	}

	if len(book.Chapters) != 1 {
		t.Fatalf("chapter count = %d, want 1", len(book.Chapters))
	}
	if book.Chapters[0].ID != "first" || book.Chapters[0].Title != "NCX First" {
		t.Fatalf("ncx chapter = %#v", book.Chapters[0])
	}
}

func TestParseEPUBTracksAndExtractsCover(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Covered.epub")
	coverBytes := []byte("fake-jpeg-cover")
	writeTestEPUBBytes(t, path, map[string][]byte{
		"META-INF/container.xml": []byte(`<?xml version="1.0"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles><rootfile full-path="OPS/package.opf"/></rootfiles>
</container>`),
		"OPS/package.opf": []byte(`<?xml version="1.0"?>
<package xmlns="http://www.idpf.org/2007/opf" version="2.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>Covered</dc:title>
    <meta name="cover" content="cover-image"/>
  </metadata>
  <manifest>
    <item id="cover-image" href="images/cover.jpg" media-type="image/jpeg"/>
    <item id="c1" href="chapter.xhtml" media-type="application/xhtml+xml"/>
  </manifest>
  <spine><itemref idref="c1"/></spine>
</package>`),
		"OPS/chapter.xhtml":    []byte(`<html><body><p>Hello</p></body></html>`),
		"OPS/images/cover.jpg": coverBytes,
	})

	book, err := ParseEPUB(path)
	if err != nil {
		t.Fatalf("ParseEPUB() error = %v", err)
	}
	if book.CoverPath != "OPS/images/cover.jpg" {
		t.Fatalf("CoverPath = %q, want OPS/images/cover.jpg", book.CoverPath)
	}
	if book.CoverMediaType != "image/jpeg" {
		t.Fatalf("CoverMediaType = %q, want image/jpeg", book.CoverMediaType)
	}

	cover, err := ExtractEPUBCover(path)
	if err != nil {
		t.Fatalf("ExtractEPUBCover() error = %v", err)
	}
	if string(cover.Data) != string(coverBytes) {
		t.Fatalf("cover bytes = %q, want %q", string(cover.Data), string(coverBytes))
	}
	if cover.MediaType != "image/jpeg" {
		t.Fatalf("cover media type = %q, want image/jpeg", cover.MediaType)
	}
}

func writeTestEPUB(t *testing.T, path string, files map[string]string) {
	t.Helper()

	byteFiles := make(map[string][]byte, len(files))
	for name, body := range files {
		byteFiles[name] = []byte(body)
	}
	writeTestEPUBBytes(t, path, byteFiles)
}

func writeTestEPUBBytes(t *testing.T, path string, files map[string][]byte) {
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
	for name, body := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("create zip entry %s: %v", name, err)
		}
		if _, err := w.Write(body); err != nil {
			t.Fatalf("write zip entry %s: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
}
