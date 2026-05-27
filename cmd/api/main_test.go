package main

import (
	"archive/zip"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oops-reader/oops-reader-backend/internal/platform/config"
	"go.uber.org/zap"
)

func TestSetupRouterExposesMVPAPI(t *testing.T) {
	root := t.TempDir()
	t.Setenv("OOPS_READER_CATALOG_ROOT", root)
	writeCatalogTestEPUB(t, filepath.Join(root, "Covered.epub"))

	router := setupRouter(&config.Config{
		JWT: config.JWTConfig{Secret: "test-secret"},
	}, zap.NewNop(), nil)

	guestRecorder := httptest.NewRecorder()
	guestRequest := httptest.NewRequest(
		http.MethodPost,
		"/v1/auth/guest",
		strings.NewReader(`{"device_id":"test-device"}`),
	)
	guestRequest.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(guestRecorder, guestRequest)
	if guestRecorder.Code != http.StatusCreated {
		t.Fatalf("guest status = %d, body = %s", guestRecorder.Code, guestRecorder.Body.String())
	}

	var guestBody struct {
		Data struct {
			AccessToken string `json:"access_token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(guestRecorder.Body.Bytes(), &guestBody); err != nil {
		t.Fatalf("decode guest response: %v", err)
	}
	if guestBody.Data.AccessToken == "" {
		t.Fatal("guest access token is empty")
	}

	for _, tc := range []struct {
		name string
		path string
	}{
		{name: "catalog", path: "/v1/catalog/books?page=1&page_size=1"},
		{name: "boards", path: "/v1/community/boards"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, tc.path, nil)
			router.ServeHTTP(recorder, request)
			if recorder.Code != http.StatusOK {
				t.Fatalf("%s status = %d, body = %s", tc.name, recorder.Code, recorder.Body.String())
			}
		})
	}

	meRecorder := httptest.NewRecorder()
	meRequest := httptest.NewRequest(http.MethodGet, "/v1/users/me", nil)
	meRequest.Header.Set("Authorization", "Bearer "+guestBody.Data.AccessToken)
	router.ServeHTTP(meRecorder, meRequest)
	if meRecorder.Code != http.StatusOK {
		t.Fatalf("me status = %d, body = %s", meRecorder.Code, meRecorder.Body.String())
	}

	createThreadRecorder := httptest.NewRecorder()
	createThreadRequest := httptest.NewRequest(
		http.MethodPost,
		"/v1/community/threads",
		strings.NewReader(`{"board_id":"general","title":"闲聊测试","content":"第一版闲聊发帖测试"}`),
	)
	createThreadRequest.Header.Set("Content-Type", "application/json")
	createThreadRequest.Header.Set("Authorization", "Bearer "+guestBody.Data.AccessToken)
	router.ServeHTTP(createThreadRecorder, createThreadRequest)
	if createThreadRecorder.Code != http.StatusCreated {
		t.Fatalf("create community thread status = %d, body = %s", createThreadRecorder.Code, createThreadRecorder.Body.String())
	}

	myThreadsRecorder := httptest.NewRecorder()
	myThreadsRequest := httptest.NewRequest(http.MethodGet, "/v1/community/threads/mine", nil)
	myThreadsRequest.Header.Set("Authorization", "Bearer "+guestBody.Data.AccessToken)
	router.ServeHTTP(myThreadsRecorder, myThreadsRequest)
	if myThreadsRecorder.Code != http.StatusOK {
		t.Fatalf("my community threads status = %d, body = %s", myThreadsRecorder.Code, myThreadsRecorder.Body.String())
	}
	var myThreadsBody struct {
		Data []struct {
			Title string `json:"title"`
		} `json:"data"`
	}
	if err := json.Unmarshal(myThreadsRecorder.Body.Bytes(), &myThreadsBody); err != nil {
		t.Fatalf("decode my community threads response: %v", err)
	}
	if len(myThreadsBody.Data) != 1 || myThreadsBody.Data[0].Title != "闲聊测试" {
		t.Fatalf("my community threads = %+v, want created thread", myThreadsBody.Data)
	}

	catalogRecorder := httptest.NewRecorder()
	catalogRequest := httptest.NewRequest(http.MethodGet, "/v1/catalog/books?page=1&page_size=1", nil)
	router.ServeHTTP(catalogRecorder, catalogRequest)
	if catalogRecorder.Code != http.StatusOK {
		t.Fatalf("catalog status = %d, body = %s", catalogRecorder.Code, catalogRecorder.Body.String())
	}

	var catalogBody struct {
		Data []struct {
			ID       string `json:"id"`
			CoverURL string `json:"cover_url"`
		} `json:"data"`
	}
	if err := json.Unmarshal(catalogRecorder.Body.Bytes(), &catalogBody); err != nil {
		t.Fatalf("decode catalog response: %v", err)
	}
	if len(catalogBody.Data) != 1 {
		t.Fatalf("catalog data length = %d, want 1", len(catalogBody.Data))
	}
	if catalogBody.Data[0].CoverURL != "http://example.com/v1/catalog/books/covered/cover" {
		t.Fatalf("cover_url = %q, want http://example.com/v1/catalog/books/covered/cover", catalogBody.Data[0].CoverURL)
	}

	coverRecorder := httptest.NewRecorder()
	coverRequest := httptest.NewRequest(http.MethodGet, catalogBody.Data[0].CoverURL, nil)
	router.ServeHTTP(coverRecorder, coverRequest)
	if coverRecorder.Code != http.StatusOK {
		t.Fatalf("cover status = %d, body = %s", coverRecorder.Code, coverRecorder.Body.String())
	}
	if got := coverRecorder.Header().Get("Content-Type"); got != "image/jpeg" {
		t.Fatalf("cover content type = %q, want image/jpeg", got)
	}
	if got := coverRecorder.Body.String(); got != "fake-jpeg-cover" {
		t.Fatalf("cover body = %q, want fake-jpeg-cover", got)
	}
}

func TestCatalogURLsRespectForwardedHost(t *testing.T) {
	root := t.TempDir()
	t.Setenv("OOPS_READER_CATALOG_ROOT", root)
	writeCatalogTestEPUB(t, filepath.Join(root, "Covered.epub"))

	router := setupRouter(&config.Config{
		JWT: config.JWTConfig{Secret: "test-secret"},
	}, zap.NewNop(), nil)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/catalog/books?page=1&page_size=1", nil)
	request.Header.Set("X-Forwarded-Proto", "https")
	request.Header.Set("X-Forwarded-Host", "books.example.test")
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("catalog status = %d, body = %s", recorder.Code, recorder.Body.String())
	}

	var body struct {
		Data []struct {
			CoverURL    string `json:"cover_url"`
			DownloadURL string `json:"download_url"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode catalog response: %v", err)
	}
	if len(body.Data) != 1 {
		t.Fatalf("catalog data length = %d, want 1", len(body.Data))
	}
	if body.Data[0].CoverURL != "https://books.example.test/v1/catalog/books/covered/cover" {
		t.Fatalf("cover_url = %q, want forwarded absolute URL", body.Data[0].CoverURL)
	}
	if body.Data[0].DownloadURL != "https://books.example.test/v1/catalog/books/covered/download" {
		t.Fatalf("download_url = %q, want forwarded absolute URL", body.Data[0].DownloadURL)
	}
}

func writeCatalogTestEPUB(t *testing.T, path string) {
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
	for name, body := range map[string]string{
		"META-INF/container.xml": `<?xml version="1.0"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles><rootfile full-path="OPS/package.opf"/></rootfiles>
</container>`,
		"OPS/package.opf": `<?xml version="1.0"?>
<package xmlns="http://www.idpf.org/2007/opf" version="2.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>Covered</dc:title>
    <dc:creator>Reader</dc:creator>
    <meta name="cover" content="cover-image"/>
  </metadata>
  <manifest>
    <item id="cover-image" href="images/cover.jpg" media-type="image/jpeg"/>
    <item id="c1" href="chapter.xhtml" media-type="application/xhtml+xml"/>
  </manifest>
  <spine><itemref idref="c1"/></spine>
</package>`,
		"OPS/chapter.xhtml":    `<html><body><h1>Hello</h1><p>World</p></body></html>`,
		"OPS/images/cover.jpg": `fake-jpeg-cover`,
	} {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("create zip entry %s: %v", name, err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatalf("write zip entry %s: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
}
