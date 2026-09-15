package handlers

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/oops-reader/oops-reader-backend/internal/catalog"
	"github.com/oops-reader/oops-reader-backend/internal/reading"
	"github.com/oops-reader/oops-reader-backend/internal/transport/http/middleware"
)

// shelfStatuses are the reading states a client may set on a shelf entry.
var shelfStatuses = map[string]bool{
	string(catalog.ShelfStatusWantToRead): true,
	"reading":                             true,
	"finished":                            true,
}

// BookshelfHandler serves the account's cloud bookshelf of online catalog
// books. The shelf holds catalog:<id> keys, the same identifier reading progress
// uses, so "on my shelf" and "recently read" can never drift apart.
type BookshelfHandler struct {
	store   catalog.ShelfStore
	catalog *catalog.Service
}

// NewBookshelfHandler creates a BookshelfHandler. store is nil when the
// database is unavailable; every endpoint then answers 503.
func NewBookshelfHandler(store catalog.ShelfStore, service *catalog.Service) *BookshelfHandler {
	return &BookshelfHandler{store: store, catalog: service}
}

// List returns one page of the account's shelf, most recently added first.
//
//	GET /v1/bookshelf?page=1&page_size=20
func (h *BookshelfHandler) List(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authorization required"})
		return
	}
	if h.store == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "shelf service unavailable"})
		return
	}

	page, pageSize := pagination(c)
	items, total, err := h.store.ListShelves(c.Request.Context(), userID, pageSize, (page-1)*pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	entries := make([]gin.H, 0, len(items))
	for _, item := range items {
		entries = append(entries, bookshelfEntryJSON(item))
	}
	c.JSON(http.StatusOK, gin.H{
		"data": entries,
		"pagination": gin.H{
			"page": page, "page_size": pageSize, "total": total,
		},
	})
}

type bookshelfAddRequest struct {
	CatalogBookKey string `json:"catalog_book_key"`
	LocalBookID    string `json:"local_book_id"`
}

// Add puts an online catalog book on the account's shelf. It is idempotent: a
// book already on the shelf stays there, and a removed book comes back.
//
//	POST /v1/bookshelf
//	Body: { "catalog_book_key": "catalog:<id>", "local_book_id": "..." }
func (h *BookshelfHandler) Add(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authorization required"})
		return
	}
	if h.store == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "shelf service unavailable"})
		return
	}

	var req bookshelfAddRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	bookKey := reading.CatalogShelfBookKey(req.CatalogBookKey)
	if bookKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "catalog_book_key is required"})
		return
	}
	// Only books the catalog can serve are worth keeping on the shelf.
	if _, err := h.catalog.GetBook(reading.CatalogBookKey(bookKey)); err != nil {
		if errors.Is(err, catalog.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "catalog book not found"})
			return
		}
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}
	if _, err := h.store.UpsertShelf(c.Request.Context(), userID, bookKey, req.LocalBookID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	item, err := h.store.GetShelfItem(c.Request.Context(), userID, bookKey)
	if err != nil {
		writeShelfError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": bookshelfEntryJSON(item)})
}

type bookshelfUpdateRequest struct {
	ShelfStatus string `json:"shelf_status"`
	LocalBookID string `json:"local_book_id"`
}

// Update replaces the reading state or the local book id of a shelf entry; a
// field left out keeps its stored value.
//
//	PATCH /v1/bookshelf/:key
//	Body: { "shelf_status": "reading", "local_book_id": "..." }
func (h *BookshelfHandler) Update(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authorization required"})
		return
	}
	if h.store == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "shelf service unavailable"})
		return
	}

	var req bookshelfUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	shelfStatus := strings.TrimSpace(req.ShelfStatus)
	if shelfStatus != "" && !shelfStatuses[shelfStatus] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "shelf_status must be want_to_read, reading, or finished"})
		return
	}
	localBookID := strings.TrimSpace(req.LocalBookID)
	if shelfStatus == "" && localBookID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "shelf_status or local_book_id is required"})
		return
	}
	bookKey := reading.CatalogShelfBookKey(c.Param("key"))
	if bookKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "catalog_book_key is required"})
		return
	}

	if _, err := h.store.UpdateShelf(c.Request.Context(), userID, bookKey, shelfStatus, localBookID); err != nil {
		writeShelfError(c, err)
		return
	}
	item, err := h.store.GetShelfItem(c.Request.Context(), userID, bookKey)
	if err != nil {
		writeShelfError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": bookshelfEntryJSON(item)})
}

// Delete removes an online book from the account's shelf. The row is
// soft-deleted, so re-adding the book later restores it.
//
//	DELETE /v1/bookshelf/:key
func (h *BookshelfHandler) Delete(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authorization required"})
		return
	}
	if h.store == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "shelf service unavailable"})
		return
	}

	bookKey := reading.CatalogShelfBookKey(c.Param("key"))
	if bookKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "catalog_book_key is required"})
		return
	}
	if err := h.store.DeleteShelf(c.Request.Context(), userID, bookKey); err != nil {
		writeShelfError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"catalog_book_key": bookKey,
		"in_library":       false,
	}})
}

// bookshelfEntryJSON renders one shelf row.
func bookshelfEntryJSON(item catalog.ShelfItem) gin.H {
	return gin.H{
		"catalog_book_key": item.BookKey,
		"local_book_id":    nullableString(item.LocalBookID),
		"shelf_status":     item.ShelfStatus,
		"in_library":       true,
		"added_at":         item.AddedAt,
		"last_read_at":     item.LastReadAt,
	}
}

func writeShelfError(c *gin.Context, err error) {
	if errors.Is(err, catalog.ErrShelfNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "bookshelf entry not found"})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
}
