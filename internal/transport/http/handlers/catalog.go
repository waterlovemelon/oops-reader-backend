package handlers

import (
	"errors"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/oops-reader/oops-reader-backend/internal/catalog"
)

type CatalogHandler struct {
	service *catalog.Service
}

func NewCatalogHandler(service *catalog.Service) *CatalogHandler {
	return &CatalogHandler{service: service}
}

func (h *CatalogHandler) ListBooks(c *gin.Context) {
	page, pageSize := pagination(c)
	books, total, err := h.service.ListBooks(c.Query("q"), page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	items := make([]gin.H, 0, len(books))
	for _, book := range books {
		items = append(items, h.bookJSON(c, book))
	}
	c.JSON(http.StatusOK, gin.H{
		"data": items,
		"pagination": gin.H{
			"page": page, "page_size": pageSize, "total": total,
		},
	})
}

func (h *CatalogHandler) GetBook(c *gin.Context) {
	book, err := h.service.GetBook(c.Param("id"))
	if err != nil {
		writeCatalogError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": h.bookJSON(c, *book)})
}

func (h *CatalogHandler) Download(c *gin.Context) {
	assetPath, err := h.service.AssetPath(c.Param("id"))
	if err != nil {
		writeCatalogError(c, err)
		return
	}
	c.FileAttachment(assetPath, filepath.Base(assetPath))
}

func (h *CatalogHandler) Cover(c *gin.Context) {
	cover, err := h.service.GetCover(c.Param("id"))
	if err != nil {
		writeCatalogError(c, err)
		return
	}
	c.Data(http.StatusOK, cover.MediaType, cover.Data)
}

func (h *CatalogHandler) Manifest(c *gin.Context) {
	manifest, err := h.service.GetManifest(c.Param("id"))
	if err != nil {
		writeCatalogError(c, err)
		return
	}

	chapters := make([]gin.H, 0, len(manifest.Chapters))
	for index, chapter := range manifest.Chapters {
		chapters = append(chapters, gin.H{
			"id":         chapter.ID,
			"title":      chapter.Title,
			"order":      index + 1,
			"word_count": 0,
		})
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"book_id":  manifest.BookID,
		"version":  "local",
		"title":    manifest.Title,
		"author":   manifest.Author,
		"chapters": chapters,
	}})
}

func (h *CatalogHandler) Chapter(c *gin.Context) {
	bookID := c.Param("id")
	chapterID := c.Param("chapter_id")
	manifest, err := h.service.GetManifest(bookID)
	if err != nil {
		writeCatalogError(c, err)
		return
	}
	text, err := h.service.GetChapter(bookID, chapterID)
	if err != nil {
		writeCatalogError(c, err)
		return
	}

	var title, prevID, nextID string
	for index, chapter := range manifest.Chapters {
		if chapter.ID != chapterID {
			continue
		}
		title = chapter.Title
		if index > 0 {
			prevID = manifest.Chapters[index-1].ID
		}
		if index+1 < len(manifest.Chapters) {
			nextID = manifest.Chapters[index+1].ID
		}
		break
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"book_id":         bookID,
		"chapter_id":      chapterID,
		"title":           title,
		"content_type":    "text/plain",
		"text":            text,
		"next_chapter_id": nullableString(nextID),
		"prev_chapter_id": nullableString(prevID),
	}})
}

func (h *CatalogHandler) bookJSON(c *gin.Context, book catalog.Book) gin.H {
	return gin.H{
		"id":            book.ID,
		"title":         book.Title,
		"author":        book.Author,
		"description":   "",
		"cover_url":     absolutePath(c, "/v1/catalog/books/"+book.ID+"/cover"),
		"download_url":  absolutePath(c, "/v1/catalog/books/"+book.ID+"/download"),
		"language":      book.Language,
		"chapter_count": book.ChapterCount,
	}
}

func writeCatalogError(c *gin.Context, err error) {
	if errors.Is(err, catalog.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
}

func pagination(c *gin.Context) (int, int) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}

func absolutePath(c *gin.Context, path string) string {
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}
	host := firstHeaderValue(c.GetHeader("X-Forwarded-Host"))
	if host == "" {
		host = c.Request.Host
	}
	if host == "" {
		return path
	}

	scheme := firstHeaderValue(c.GetHeader("X-Forwarded-Proto"))
	if scheme == "" {
		scheme = "http"
		if c.Request.TLS != nil {
			scheme = "https"
		}
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return scheme + "://" + host + path
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func firstHeaderValue(value string) string {
	if index := strings.Index(value, ","); index >= 0 {
		value = value[:index]
	}
	return strings.TrimSpace(value)
}
