package handlers

import (
	"errors"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/oops-reader/oops-reader-backend/internal/catalog"
	"github.com/oops-reader/oops-reader-backend/internal/transport/http/middleware"
)

type CatalogHandler struct {
	service       *catalog.Service
	shelfStore    catalog.ShelfStore
	commentsStore catalog.CommentsStore
}

func NewCatalogHandler(service *catalog.Service, shelfStore catalog.ShelfStore, commentsStore catalog.CommentsStore) *CatalogHandler {
	return &CatalogHandler{service: service, shelfStore: shelfStore, commentsStore: commentsStore}
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
	data := h.bookJSON(c, *book)
	if userID, ok := middleware.CurrentUserID(c); ok && h.shelfStore != nil {
		shelf, err := h.shelfStore.GetShelf(c.Request.Context(), userID, book.ID)
		if err == nil {
			data["shelf"] = shelfJSON(shelf)
		} else {
			data["shelf"] = gin.H{
				"in_library":   false,
				"shelf_status": nil,
				"last_read_at": nil,
			}
		}
	} else {
		data["shelf"] = gin.H{
			"in_library":   false,
			"shelf_status": nil,
			"last_read_at": nil,
		}
	}
	commentTotal := 0
	if h.commentsStore != nil {
		if total, err := h.commentsStore.CountPublished(c.Request.Context(), book.ID); err == nil {
			commentTotal = total
		}
	}
	data["comment_summary"] = gin.H{"total": commentTotal}
	c.JSON(http.StatusOK, gin.H{"data": data})
}

type addToShelfRequest struct {
	LocalBookID string `json:"local_book_id"`
}

func (h *CatalogHandler) AddToShelf(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authorization required"})
		return
	}
	if h.shelfStore == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "shelf service unavailable"})
		return
	}
	bookID := c.Param("id")
	// Verify book exists and is active.
	if _, err := h.service.GetBook(bookID); err != nil {
		writeCatalogError(c, err)
		return
	}
	var req addToShelfRequest
	if err := c.ShouldBindJSON(&req); err != nil && err.Error() != "EOF" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	state, err := h.shelfStore.UpsertShelf(c.Request.Context(), userID, bookID, req.LocalBookID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"book_id":      bookID,
		"in_library":   state.InLibrary,
		"shelf_status": nullableString(state.ShelfStatus),
	}})
}

func shelfJSON(state catalog.ShelfState) gin.H {
	return gin.H{
		"in_library":   state.InLibrary,
		"shelf_status": nullableString(state.ShelfStatus),
		"last_read_at": state.LastReadAt,
	}
}

func (h *CatalogHandler) ListComments(c *gin.Context) {
	bookID := c.Param("id")
	// Verify book exists.
	if _, err := h.service.GetBook(bookID); err != nil {
		writeCatalogError(c, err)
		return
	}
	page, pageSize := pagination(c)
	if h.commentsStore == nil {
		c.JSON(http.StatusOK, gin.H{"data": []any{}, "pagination": gin.H{"page": page, "page_size": pageSize, "total": 0}})
		return
	}
	comments, total, err := h.commentsStore.ListComments(c.Request.Context(), bookID, pageSize, (page-1)*pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	items := make([]gin.H, 0, len(comments))
	for _, cc := range comments {
		items = append(items, catalogCommentJSON(cc))
	}
	c.JSON(http.StatusOK, gin.H{
		"data": items,
		"pagination": gin.H{
			"page": page, "page_size": pageSize, "total": total,
		},
	})
}

type createCommentRequest struct {
	Content string `json:"content"`
}

func (h *CatalogHandler) CreateComment(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authorization required"})
		return
	}
	if h.commentsStore == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "comments service unavailable"})
		return
	}
	bookID := c.Param("id")
	// Verify book exists and is active.
	if _, err := h.service.GetBook(bookID); err != nil {
		writeCatalogError(c, err)
		return
	}
	var req createCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	content := strings.TrimSpace(req.Content)
	if content == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "content is required"})
		return
	}
	if len([]rune(content)) > 1000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "content must be 1000 characters or less"})
		return
	}
	comment, err := h.commentsStore.CreateComment(c.Request.Context(), catalog.BookComment{
		BookID:  bookID,
		UserID:  userID,
		Content: content,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": catalogCommentJSON(comment)})
}

func catalogCommentJSON(c catalog.BookComment) gin.H {
	authorID := ""
	if c.UserID > 0 {
		authorID = strconv.FormatUint(c.UserID, 10)
	}
	return gin.H{
		"id":      c.ID,
		"book_id": c.BookID,
		"author": gin.H{
			"id":           authorID,
			"display_name": c.DisplayName,
		},
		"source":     c.Source,
		"content":    c.Content,
		"like_count": c.LikeCount,
		"created_at": c.CreatedAt,
	}
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
		"description":   book.Description,
		"cover_url":     absolutePath(c, "/v1/catalog/books/"+book.ID+"/cover"),
		"download_url":  absolutePath(c, "/v1/catalog/books/"+book.ID+"/download"),
		"language":      book.Language,
		"chapter_count": book.ChapterCount,
		"word_count":    nullablePositiveInt64(book.WordCount),
	}
}

func nullablePositiveInt64(value int64) any {
	if value <= 0 {
		return nil
	}
	return value
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
