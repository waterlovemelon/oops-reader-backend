package handlers

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/oops-reader/oops-reader-backend/internal/catalog"
)

type handlerReadingStore struct{ book catalog.Book }

func (s handlerReadingStore) ListBooks(context.Context, string, int, int) ([]catalog.Book, int, error) {
	return []catalog.Book{s.book}, 1, nil
}
func (s handlerReadingStore) GetBook(context.Context, string) (catalog.Book, error) {
	return s.book, nil
}

func newReadingTestServer(t *testing.T) (*httptest.Server, string) {
	t.Helper()
	root := t.TempDir()
	bookPath := filepath.Join(root, "book.epub")
	if err := os.WriteFile(bookPath, []byte("epub"), 0600); err != nil {
		t.Fatal(err)
	}
	base := filepath.Join(root, ".reading", "book")
	for _, version := range []string{"v1", "v2"} {
		versionRoot := filepath.Join(base, version)
		if err := os.MkdirAll(filepath.Join(versionRoot, "segments"), 0750); err != nil {
			t.Fatal(err)
		}
		label := "old"
		if version == "v2" {
			label = "new"
		}
		manifest := `{"schema_version":2,"normalizer_version":2,"book_id":"book","content_version":"` + version + `","title":"T","author":"A","chapters":[{"id":"c1","href":"one.xhtml","title":"One","order":0,"spine_index":4,"start_unit":0,"end_unit":50,"segment_ids":["s1"]},{"id":"c2","href":"two.xhtml","title":"Two","order":1,"spine_index":5,"start_unit":50,"end_unit":100,"segment_ids":["s2"]}],"total_units":100,"reading_order":["c1","c2"],"toc":[{"label":"One"}],"segment_index":[{"segment_id":"s1","chapter_id":"c1","spine_index":4,"start_unit":0,"end_unit":50},{"segment_id":"s2","chapter_id":"c2","spine_index":5,"start_unit":50,"end_unit":100}],"resources":{"cover.jpg":{"resource_id":"cover.jpg","media_type":"image/jpeg"}},"future_field":{"kept":true}}`
		if err := os.WriteFile(filepath.Join(versionRoot, "manifest.json"), []byte(manifest), 0600); err != nil {
			t.Fatal(err)
		}
		for _, seg := range []struct {
			id, chapter string
			start, end  int
			block       int
		}{
			{"s1", "c1", 0, 50, 9},
			{"s2", "c2", 50, 100, 12},
		} {
			body := `{"book_id":"book","content_version":"` + version + `","segment_id":"` + seg.id + `","chapter_id":"` + seg.chapter + `","start_unit":` + itoa(seg.start) + `,"end_unit":` + itoa(seg.end) + `,"blocks":[{"id":"` + seg.id + `-block","source_block_id":"source-` + seg.id + `","source_block_index":` + itoa(seg.block) + `,"start_unit":` + itoa(seg.start) + `,"end_unit":` + itoa(seg.end) + `,"type":"paragraph","children":[{"type":"text","text":"` + label + ` 😀"}]}]}`
			if err := os.WriteFile(filepath.Join(versionRoot, "segments", seg.id+".json"), []byte(body), 0600); err != nil {
				t.Fatal(err)
			}
		}
		if err := os.MkdirAll(filepath.Join(versionRoot, "resources"), 0750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(versionRoot, "resources.json"), []byte(`{"cover.jpg":{"resource_id":"cover.jpg","media_type":"image/jpeg"}}`), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(versionRoot, "resources", "cover.jpg"), []byte("image"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink("v1", filepath.Join(base, "current")); err != nil {
		t.Fatal(err)
	}
	service := catalog.NewServiceWithStore(root, handlerReadingStore{book: catalog.Book{ID: "book", Path: bookPath}})
	h := NewCatalogHandler(service, nil, nil)
	r := gin.New()
	r.GET("/books/:id/download", h.Download)
	r.HEAD("/books/:id/download", h.Download)
	r.GET("/books/:id/reading/open", h.ReadingOpen)
	r.HEAD("/books/:id/reading/open", h.ReadingOpen)
	r.GET("/books/:id/reading/versions/:version/index", h.ReadingIndex)
	r.HEAD("/books/:id/reading/versions/:version/index", h.ReadingIndex)
	r.GET("/books/:id/reading/versions/:version/segments/:segment_id", h.ReadingSegment)
	r.HEAD("/books/:id/reading/versions/:version/segments/:segment_id", h.ReadingSegment)
	r.GET("/books/:id/reading/versions/:version/resources/:resource_id", h.ReadingResource)
	r.HEAD("/books/:id/reading/versions/:version/resources/:resource_id", h.ReadingResource)
	r.GET("/books/:id/reading/versions/:version/resources-index", h.ReadingResourcesIndex)
	r.HEAD("/books/:id/reading/versions/:version/resources-index", h.ReadingResourcesIndex)
	return httptest.NewServer(r), root
}

func itoa(n int) string {
	return strconv.Itoa(n)
}

func openJSON(t *testing.T, server, query string) (int, map[string]any, *http.Response) {
	t.Helper()
	resp, err := http.Get(server + "/books/book/reading/open" + query)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var envelope map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	data, _ := envelope["data"].(map[string]any)
	return resp.StatusCode, data, resp
}

func TestReadingHandlerResolvesStableLocatorsAndPreservesManifest(t *testing.T) {
	server, _ := newReadingTestServer(t)
	defer server.Close()
	for _, tc := range []struct {
		name, query, segment string
		unitOffset           float64
	}{
		{"unit zero", "?unit=0", "s1", 0},
		{"chapter only", "?chapter_id=c2", "s2", 0},
		{"segment only", "?segment_id=s2", "s2", 0},
		{"seventy percent", "?unit=70", "s2", 20},
		{"end", "?unit=100", "s2", 50},
	} {
		t.Run(tc.name, func(t *testing.T) {
			status, data, _ := openJSON(t, server.URL, tc.query)
			if status != http.StatusOK {
				t.Fatalf("status = %d", status)
			}
			segment, _ := data["segment"].(map[string]any)
			if segment["segment_id"] != tc.segment {
				t.Fatalf("segment = %v, want %s", segment["segment_id"], tc.segment)
			}
			locator, _ := data["resolved_locator"].(map[string]any)
			if locator["inline_offset"] != tc.unitOffset {
				t.Fatalf("inline_offset = %v, want %v", locator["inline_offset"], tc.unitOffset)
			}
			if locator["block_id"] == nil || locator["block_index"] != locator["source_block_index"] {
				t.Fatalf("locator lost stable block identity: %#v", locator)
			}
			manifest, _ := data["manifest"].(map[string]any)
			if manifest["resources"] == nil || manifest["future_field"] == nil {
				t.Fatalf("manifest fields were dropped: %#v", manifest)
			}
		})
	}
	if status, _, _ := openJSON(t, server.URL, "?chapter_id=c1&segment_id=s2"); status != http.StatusBadRequest {
		t.Fatalf("contradictory params status = %d, want 400", status)
	}
	if status, _, _ := openJSON(t, server.URL, "?unit=nope"); status != http.StatusBadRequest {
		t.Fatalf("invalid unit status = %d, want 400", status)
	}
}

func TestReadingHandlerVersionsGzipETagHEADAndResourceSafety(t *testing.T) {
	server, _ := newReadingTestServer(t)
	defer server.Close()
	client := &http.Client{}
	req, _ := http.NewRequest(http.MethodGet, server.URL+"/books/book/reading/versions/v1/index", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	compressed, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || resp.Header.Get("Content-Encoding") != "gzip" {
		t.Fatalf("gzip index = %d/%q", resp.StatusCode, resp.Header.Get("Content-Encoding"))
	}
	zr, err := gzip.NewReader(strings.NewReader(string(compressed)))
	if err != nil {
		t.Fatal(err)
	}
	decoded, _ := io.ReadAll(zr)
	if !strings.Contains(string(decoded), `"resources"`) {
		t.Fatalf("gzip payload lost resources: %s", decoded)
	}
	etag := resp.Header.Get("ETag")
	req, _ = http.NewRequest(http.MethodGet, server.URL+"/books/book/reading/versions/v1/segments/s1", nil)
	req.Header.Set("If-None-Match", etag)
	// Segment and index have different ETags; obtain the segment's first.
	resp, _ = client.Get(server.URL + "/books/book/reading/versions/v1/segments/s1")
	segmentETag := resp.Header.Get("ETag")
	resp.Body.Close()
	req.Header.Set("If-None-Match", segmentETag)
	resp, _ = client.Do(req)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotModified {
		t.Fatalf("304 status = %d", resp.StatusCode)
	}
	req, _ = http.NewRequest(http.MethodHead, server.URL+"/books/book/reading/versions/v1/segments/s1", nil)
	resp, _ = client.Do(req)
	if resp.StatusCode != http.StatusOK || resp.ContentLength <= 0 {
		t.Fatalf("HEAD = %d length %d", resp.StatusCode, resp.ContentLength)
	}
	resp.Body.Close()
	if status, _, _ := openJSON(t, server.URL, "?version=v2&unit=0"); status != http.StatusOK {
		t.Fatalf("fixed version status = %d", status)
	}
	if status, _, _ := openJSON(t, server.URL, "?version=../v1&unit=0"); status != http.StatusBadRequest {
		t.Fatalf("path traversal status = %d", status)
	}
}

func TestDownloadRangeAndForegroundReadingAreNotBlockedByLargeDownload(t *testing.T) {
	server, root := newReadingTestServer(t)
	defer server.Close()
	large := make([]byte, 256*1024)
	if err := os.WriteFile(filepath.Join(root, "book.epub"), large, 0600); err != nil {
		t.Fatal(err)
	}
	client := &http.Client{}
	result := make(chan error, 1)
	go func() {
		resp, err := client.Get(server.URL + "/books/book/download")
		if err == nil {
			_, err = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
		}
		result <- err
	}()
	time.Sleep(30 * time.Millisecond)
	started := time.Now()
	resp, err := client.Get(server.URL + "/books/book/reading/versions/v1/segments/s1")
	if err != nil {
		t.Fatal(err)
	}
	io.ReadAll(resp.Body)
	resp.Body.Close()
	if time.Since(started) > time.Second {
		t.Fatalf("foreground segment waited behind download: %s", time.Since(started))
	}
	<-result
	req, _ := http.NewRequest(http.MethodGet, server.URL+"/books/book/download", nil)
	req.Header.Set("Range", "bytes=0-3")
	resp, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusPartialContent || len(body) != 4 {
		t.Fatalf("range = %d, %d bytes", resp.StatusCode, len(body))
	}
}
