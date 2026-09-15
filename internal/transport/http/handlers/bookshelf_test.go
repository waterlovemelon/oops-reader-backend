package handlers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/oops-reader/oops-reader-backend/internal/catalog"
)

const testShelfUserID uint64 = 7

// fakeShelfStore keeps shelf rows in memory; a key present in rows is on the
// shelf, a missing key reports ErrShelfNotFound.
type fakeShelfStore struct {
	rows       map[string]catalog.ShelfItem
	lastLimit  int
	lastOffset int
}

func newFakeShelfStore(items ...catalog.ShelfItem) *fakeShelfStore {
	store := &fakeShelfStore{rows: map[string]catalog.ShelfItem{}}
	for _, item := range items {
		store.rows[item.BookKey] = item
	}
	return store
}

func (f *fakeShelfStore) GetShelf(_ context.Context, _ uint64, bookKey string) (catalog.ShelfState, error) {
	item, ok := f.rows[bookKey]
	if !ok {
		return catalog.ShelfState{InLibrary: false}, nil
	}
	return catalog.ShelfState{
		BookID:      item.BookKey,
		InLibrary:   true,
		ShelfStatus: item.ShelfStatus,
		LastReadAt:  item.LastReadAt,
	}, nil
}

func (f *fakeShelfStore) GetShelfItem(_ context.Context, _ uint64, bookKey string) (catalog.ShelfItem, error) {
	item, ok := f.rows[bookKey]
	if !ok {
		return catalog.ShelfItem{}, catalog.ErrShelfNotFound
	}
	return item, nil
}

func (f *fakeShelfStore) UpsertShelf(_ context.Context, _ uint64, bookKey, localBookID string) (catalog.ShelfState, error) {
	item, ok := f.rows[bookKey]
	if !ok {
		item = catalog.ShelfItem{
			BookKey:     bookKey,
			ShelfStatus: string(catalog.ShelfStatusWantToRead),
			AddedAt:     time.Date(2026, 9, 15, 10, 0, 0, 0, time.UTC),
		}
	}
	if localBookID != "" {
		item.LocalBookID = localBookID
	}
	f.rows[bookKey] = item
	return f.GetShelf(context.Background(), 0, bookKey)
}

func (f *fakeShelfStore) ListShelves(_ context.Context, _ uint64, limit, offset int) ([]catalog.ShelfItem, int, error) {
	f.lastLimit, f.lastOffset = limit, offset
	items := make([]catalog.ShelfItem, 0, len(f.rows))
	for _, item := range f.rows {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].AddedAt.After(items[j].AddedAt) })
	if offset >= len(items) {
		return []catalog.ShelfItem{}, len(items), nil
	}
	end := offset + limit
	if end > len(items) {
		end = len(items)
	}
	return items[offset:end], len(items), nil
}

func (f *fakeShelfStore) UpdateShelf(_ context.Context, _ uint64, bookKey, shelfStatus, localBookID string) (catalog.ShelfState, error) {
	item, ok := f.rows[bookKey]
	if !ok {
		return catalog.ShelfState{}, catalog.ErrShelfNotFound
	}
	if shelfStatus != "" {
		item.ShelfStatus = shelfStatus
	}
	if localBookID != "" {
		item.LocalBookID = localBookID
	}
	f.rows[bookKey] = item
	return f.GetShelf(context.Background(), 0, bookKey)
}

func (f *fakeShelfStore) DeleteShelf(_ context.Context, _ uint64, bookKey string) error {
	if _, ok := f.rows[bookKey]; !ok {
		return catalog.ErrShelfNotFound
	}
	delete(f.rows, bookKey)
	return nil
}

type shelfCatalogStore struct{ books map[string]catalog.Book }

func (s shelfCatalogStore) ListBooks(context.Context, string, int, int) ([]catalog.Book, int, error) {
	books := make([]catalog.Book, 0, len(s.books))
	for _, book := range s.books {
		books = append(books, book)
	}
	return books, len(books), nil
}

func (s shelfCatalogStore) GetBook(_ context.Context, id string) (catalog.Book, error) {
	book, ok := s.books[id]
	if !ok {
		return catalog.Book{}, catalog.ErrNotFound
	}
	return book, nil
}

// The routes are registered the way main.go does, minus the JWT middleware:
// the account id is put in the context directly.
func newBookshelfTestServer(t *testing.T, store catalog.ShelfStore) *httptest.Server {
	t.Helper()
	service := catalog.NewServiceWithStore("", shelfCatalogStore{books: map[string]catalog.Book{
		"remote_walden": {ID: "remote_walden", Title: "Walden"},
	}})
	handler := NewBookshelfHandler(store, service)
	router := gin.New()
	routes := router.Group("/v1/bookshelf")
	routes.Use(func(c *gin.Context) { c.Set("user_id", testShelfUserID) })
	routes.GET("", handler.List)
	routes.POST("", handler.Add)
	routes.PATCH("/:key", handler.Update)
	routes.DELETE("/:key", handler.Delete)
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)
	return server
}

func doShelfRequest(t *testing.T, server *httptest.Server, method, path, body string) (int, map[string]any) {
	t.Helper()
	var payload io.Reader
	if body != "" {
		payload = strings.NewReader(body)
	}
	request, err := http.NewRequest(method, server.URL+path, payload)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()

	decoded := map[string]any{}
	if err := json.NewDecoder(response.Body).Decode(&decoded); err != nil {
		t.Fatalf("decode %s %s response: %v", method, path, err)
	}
	return response.StatusCode, decoded
}

func shelfEntry(t *testing.T, decoded map[string]any) map[string]any {
	t.Helper()
	entry, ok := decoded["data"].(map[string]any)
	if !ok {
		t.Fatalf("response has no data object: %v", decoded)
	}
	return entry
}

func TestBookshelfHandlerListsOnePageMostRecentlyAddedFirst(t *testing.T) {
	lastRead := time.Date(2026, 9, 14, 20, 0, 0, 0, time.UTC)
	store := newFakeShelfStore(
		catalog.ShelfItem{BookKey: "catalog:oldest", ShelfStatus: "finished", AddedAt: time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)},
		catalog.ShelfItem{BookKey: "catalog:middle", ShelfStatus: "reading", LocalBookID: "1780289594444443", AddedAt: time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC), LastReadAt: &lastRead},
		catalog.ShelfItem{BookKey: "catalog:newest", ShelfStatus: "want_to_read", AddedAt: time.Date(2026, 9, 15, 10, 0, 0, 0, time.UTC)},
	)
	server := newBookshelfTestServer(t, store)

	status, decoded := doShelfRequest(t, server, http.MethodGet, "/v1/bookshelf?page=2&page_size=2", "")
	if status != http.StatusOK {
		t.Fatalf("GET /v1/bookshelf status = %d, want 200 (%v)", status, decoded)
	}
	if store.lastLimit != 2 || store.lastOffset != 2 {
		t.Fatalf("store pagination = limit %d offset %d, want limit 2 offset 2", store.lastLimit, store.lastOffset)
	}
	pagination, ok := decoded["pagination"].(map[string]any)
	if !ok || pagination["total"] != float64(3) || pagination["page"] != float64(2) || pagination["page_size"] != float64(2) {
		t.Fatalf("pagination = %v, want page 2 of 3 rows", decoded["pagination"])
	}
	items, ok := decoded["data"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("data = %v, want the single row of page 2", decoded["data"])
	}
	entry := items[0].(map[string]any)
	if entry["catalog_book_key"] != "catalog:oldest" || entry["shelf_status"] != "finished" {
		t.Fatalf("entry = %v, want catalog:oldest finished", entry)
	}
	if _, present := entry["local_book_id"]; !present {
		t.Fatalf("entry = %v, want a local_book_id field", entry)
	}
	if entry["added_at"] != "2026-09-01T10:00:00Z" || entry["last_read_at"] != nil {
		t.Fatalf("entry timestamps = %v/%v, want the stored added_at and a null last_read_at", entry["added_at"], entry["last_read_at"])
	}
}

func TestBookshelfHandlerListReportsStoredLocalBookIDAndLastRead(t *testing.T) {
	lastRead := time.Date(2026, 9, 14, 20, 0, 0, 0, time.UTC)
	store := newFakeShelfStore(catalog.ShelfItem{
		BookKey:     "catalog:remote_walden",
		ShelfStatus: "reading",
		LocalBookID: "1780289594444443",
		AddedAt:     time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC),
		LastReadAt:  &lastRead,
	})
	server := newBookshelfTestServer(t, store)

	status, decoded := doShelfRequest(t, server, http.MethodGet, "/v1/bookshelf", "")
	if status != http.StatusOK {
		t.Fatalf("GET /v1/bookshelf status = %d, want 200 (%v)", status, decoded)
	}
	entry := decoded["data"].([]any)[0].(map[string]any)
	if entry["catalog_book_key"] != "catalog:remote_walden" ||
		entry["local_book_id"] != "1780289594444443" ||
		entry["shelf_status"] != "reading" ||
		entry["in_library"] != true ||
		entry["last_read_at"] != "2026-09-14T20:00:00Z" {
		t.Fatalf("entry = %v", entry)
	}
}

func TestBookshelfHandlerAddStoresTheCanonicalKey(t *testing.T) {
	store := newFakeShelfStore()
	server := newBookshelfTestServer(t, store)

	status, decoded := doShelfRequest(t, server, http.MethodPost, "/v1/bookshelf",
		`{"catalog_book_key":"remote_walden","local_book_id":"1780289594444443"}`)
	if status != http.StatusOK {
		t.Fatalf("POST /v1/bookshelf status = %d, want 200 (%v)", status, decoded)
	}
	stored, err := store.GetShelfItem(context.Background(), testShelfUserID, "catalog:remote_walden")
	if err != nil {
		t.Fatalf("shelf was not written under the canonical key: %v", err)
	}
	if stored.LocalBookID != "1780289594444443" {
		t.Fatalf("local_book_id = %q, want the request value", stored.LocalBookID)
	}
	if len(store.rows) != 1 {
		t.Fatalf("shelf has %d rows, want 1", len(store.rows))
	}
	entry := shelfEntry(t, decoded)
	if entry["catalog_book_key"] != "catalog:remote_walden" || entry["shelf_status"] != "want_to_read" {
		t.Fatalf("entry = %v, want the canonical key and want_to_read", entry)
	}

	// Re-adding the same book, already canonical, stays one row.
	status, _ = doShelfRequest(t, server, http.MethodPost, "/v1/bookshelf", `{"catalog_book_key":"catalog:remote_walden"}`)
	if status != http.StatusOK || len(store.rows) != 1 {
		t.Fatalf("re-add status = %d with %d rows, want 200 and 1", status, len(store.rows))
	}
}

func TestBookshelfHandlerAddUnknownBookIsNotFound(t *testing.T) {
	store := newFakeShelfStore()
	server := newBookshelfTestServer(t, store)

	status, decoded := doShelfRequest(t, server, http.MethodPost, "/v1/bookshelf", `{"catalog_book_key":"catalog:ghost"}`)
	if status != http.StatusNotFound {
		t.Fatalf("POST unknown book status = %d, want 404 (%v)", status, decoded)
	}
	if len(store.rows) != 0 {
		t.Fatalf("shelf has %d rows, want none", len(store.rows))
	}
}

func TestBookshelfHandlerAddWithoutKeyIsBadRequest(t *testing.T) {
	server := newBookshelfTestServer(t, newFakeShelfStore())

	status, decoded := doShelfRequest(t, server, http.MethodPost, "/v1/bookshelf", `{"local_book_id":"1780289594444443"}`)
	if status != http.StatusBadRequest {
		t.Fatalf("POST without catalog_book_key status = %d, want 400 (%v)", status, decoded)
	}
}

func TestBookshelfHandlerUpdateReplacesOnlyTheGivenFields(t *testing.T) {
	store := newFakeShelfStore(catalog.ShelfItem{
		BookKey:     "catalog:remote_walden",
		ShelfStatus: "want_to_read",
		LocalBookID: "1780289594444443",
		AddedAt:     time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC),
	})
	server := newBookshelfTestServer(t, store)

	status, decoded := doShelfRequest(t, server, http.MethodPatch, "/v1/bookshelf/catalog:remote_walden",
		`{"shelf_status":"reading"}`)
	if status != http.StatusOK {
		t.Fatalf("PATCH status = %d, want 200 (%v)", status, decoded)
	}
	entry := shelfEntry(t, decoded)
	if entry["shelf_status"] != "reading" || entry["local_book_id"] != "1780289594444443" {
		t.Fatalf("entry = %v, want reading with the local book id kept", entry)
	}

	// A bare key updates the same row.
	status, decoded = doShelfRequest(t, server, http.MethodPatch, "/v1/bookshelf/remote_walden",
		`{"local_book_id":"1780289594444999"}`)
	if status != http.StatusOK {
		t.Fatalf("PATCH with bare key status = %d, want 200 (%v)", status, decoded)
	}
	entry = shelfEntry(t, decoded)
	if entry["local_book_id"] != "1780289594444999" || entry["shelf_status"] != "reading" {
		t.Fatalf("entry = %v, want the new local book id with the status kept", entry)
	}
}

func TestBookshelfHandlerUpdateRejectsBadRequests(t *testing.T) {
	store := newFakeShelfStore(catalog.ShelfItem{
		BookKey:     "catalog:remote_walden",
		ShelfStatus: "want_to_read",
		AddedAt:     time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC),
	})
	server := newBookshelfTestServer(t, store)

	tests := []struct {
		name string
		path string
		body string
	}{
		{"unsupported status", "/v1/bookshelf/catalog:remote_walden", `{"shelf_status":"done"}`},
		{"no field to replace", "/v1/bookshelf/catalog:remote_walden", `{}`},
		{"empty key", "/v1/bookshelf/%20", `{"shelf_status":"reading"}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			status, decoded := doShelfRequest(t, server, http.MethodPatch, test.path, test.body)
			if status != http.StatusBadRequest {
				t.Fatalf("PATCH %s status = %d, want 400 (%v)", test.path, status, decoded)
			}
		})
	}
	if store.rows["catalog:remote_walden"].ShelfStatus != "want_to_read" {
		t.Fatalf("rejected update changed the row: %v", store.rows["catalog:remote_walden"])
	}
}

func TestBookshelfHandlerUpdateAndDeleteMissingRowAreNotFound(t *testing.T) {
	server := newBookshelfTestServer(t, newFakeShelfStore())

	status, decoded := doShelfRequest(t, server, http.MethodPatch, "/v1/bookshelf/catalog:ghost", `{"shelf_status":"reading"}`)
	if status != http.StatusNotFound {
		t.Fatalf("PATCH missing row status = %d, want 404 (%v)", status, decoded)
	}
	status, decoded = doShelfRequest(t, server, http.MethodDelete, "/v1/bookshelf/catalog:ghost", "")
	if status != http.StatusNotFound {
		t.Fatalf("DELETE missing row status = %d, want 404 (%v)", status, decoded)
	}
}

func TestBookshelfHandlerDeleteRemovesTheCanonicalRow(t *testing.T) {
	store := newFakeShelfStore(catalog.ShelfItem{
		BookKey:     "catalog:remote_walden",
		ShelfStatus: "reading",
		AddedAt:     time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC),
	})
	server := newBookshelfTestServer(t, store)

	status, decoded := doShelfRequest(t, server, http.MethodDelete, "/v1/bookshelf/remote_walden", "")
	if status != http.StatusOK {
		t.Fatalf("DELETE status = %d, want 200 (%v)", status, decoded)
	}
	entry := shelfEntry(t, decoded)
	if entry["catalog_book_key"] != "catalog:remote_walden" || entry["in_library"] != false {
		t.Fatalf("entry = %v, want the canonical key and in_library false", entry)
	}
	if len(store.rows) != 0 {
		t.Fatalf("shelf still has %v", store.rows)
	}
}

func TestBookshelfHandlerWithoutStoreIsUnavailable(t *testing.T) {
	server := newBookshelfTestServer(t, nil)

	requests := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodGet, "/v1/bookshelf", ""},
		{http.MethodPost, "/v1/bookshelf", `{"catalog_book_key":"catalog:remote_walden"}`},
		{http.MethodPatch, "/v1/bookshelf/catalog:remote_walden", `{"shelf_status":"reading"}`},
		{http.MethodDelete, "/v1/bookshelf/catalog:remote_walden", ""},
	}
	for _, request := range requests {
		t.Run(request.method, func(t *testing.T) {
			status, decoded := doShelfRequest(t, server, request.method, request.path, request.body)
			if status != http.StatusServiceUnavailable {
				t.Fatalf("%s %s status = %d, want 503 (%v)", request.method, request.path, status, decoded)
			}
			if decoded["error"] != "shelf service unavailable" {
				t.Fatalf("error = %v, want shelf service unavailable", decoded["error"])
			}
		})
	}
}
