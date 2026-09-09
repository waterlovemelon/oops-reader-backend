package handlers

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/oops-reader/oops-reader-backend/internal/catalog"
	"github.com/oops-reader/oops-reader-backend/internal/transport/http/middleware"
)

type CatalogHandler struct {
	service       *catalog.Service
	shelfStore    catalog.ShelfStore
	commentsStore catalog.CommentsStore
}

// The limiter reserves at most one small packet per Write. This keeps a large
// background download from reserving the entire output window and lets a
// foreground reading request acquire the next packet promptly.
const (
	// 3 Mbps is 375,000 bytes/s; retain a little headroom for HTTP/TCP
	// overhead so the physical link is not saturated by payload alone.
	readingEgressBytesPerSecond = 360000
	readingEgressChunkSize      = 16 * 1024
)

var readingEgress struct {
	mu                sync.Mutex
	tokens            float64
	lastRefill        time.Time
	foregroundWaiters int
}

func waitReadingEgress(ctx context.Context, n int, background bool) error {
	if n <= 0 {
		return nil
	}
	registered := false
	for {
		readingEgress.mu.Lock()
		now := time.Now()
		if readingEgress.lastRefill.IsZero() {
			readingEgress.lastRefill = now
			readingEgress.tokens = readingEgressChunkSize
		}
		elapsed := now.Sub(readingEgress.lastRefill).Seconds()
		if elapsed > 0 {
			readingEgress.tokens += elapsed * readingEgressBytesPerSecond
			if readingEgress.tokens > readingEgressChunkSize {
				readingEgress.tokens = readingEgressChunkSize
			}
			readingEgress.lastRefill = now
		}
		if (!background || readingEgress.foregroundWaiters == 0) && readingEgress.tokens >= float64(n) {
			readingEgress.tokens -= float64(n)
			if registered {
				readingEgress.foregroundWaiters--
			}
			readingEgress.mu.Unlock()
			return nil
		}
		if !background && !registered {
			readingEgress.foregroundWaiters++
			registered = true
		}
		wait := 10 * time.Millisecond
		if !(background && readingEgress.foregroundWaiters > 0) {
			deficit := float64(n) - readingEgress.tokens
			if deficit > 0 {
				wait = time.Duration(deficit / readingEgressBytesPerSecond * float64(time.Second))
				if wait < time.Millisecond {
					wait = time.Millisecond
				}
			}
		}
		readingEgress.mu.Unlock()
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			if registered {
				readingEgress.mu.Lock()
				readingEgress.foregroundWaiters--
				readingEgress.mu.Unlock()
			}
			return ctx.Err()
		case <-timer.C:
		}
	}
}

func streamReadingBody(ctx context.Context, w io.Writer, body []byte, background bool) error {
	for len(body) > 0 {
		n := len(body)
		if n > readingEgressChunkSize {
			n = readingEgressChunkSize
		}
		chunk := body[:n]
		if err := waitReadingEgress(ctx, len(chunk), background); err != nil {
			return err
		}
		written, err := w.Write(chunk)
		if err != nil {
			return err
		}
		if written != len(chunk) {
			return io.ErrShortWrite
		}
		body = body[n:]
	}
	return nil
}

type readingRateLimitedWriter struct {
	w          io.Writer
	ctx        context.Context
	background bool
}

func (w readingRateLimitedWriter) Write(body []byte) (int, error) {
	if err := streamReadingBody(w.ctx, w.w, body, w.background); err != nil {
		return 0, err
	}
	return len(body), nil
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

func (h *CatalogHandler) ListPopularBooks(c *gin.Context) {
	_, pageSize := pagination(c)
	books, total, err := h.service.ListPopularBooks(pageSize)
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
			"page": 1, "page_size": pageSize, "total": total,
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
	file, err := os.Open(assetPath)
	if err != nil {
		writeCatalogError(c, err)
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		writeCatalogError(c, err)
		return
	}
	c.Header("Content-Disposition", `attachment; filename="`+strings.ReplaceAll(filepath.Base(assetPath), `"`, "")+`"`)
	// ServeContent retains the old endpoint's HEAD and Range semantics. Its
	// writes pass through the packetized limiter, so a 100 MB download cannot
	// block a reader by reserving 100 MB up front.
	limited := &readingRateLimitedResponseWriter{ResponseWriter: c.Writer, ctx: c.Request.Context(), background: true}
	http.ServeContent(limited, c.Request, filepath.Base(assetPath), info.ModTime(), file)
}

type readingRateLimitedResponseWriter struct {
	http.ResponseWriter
	ctx        context.Context
	background bool
}

func (w *readingRateLimitedResponseWriter) Write(body []byte) (int, error) {
	return len(body), streamReadingBody(w.ctx, w.ResponseWriter, body, w.background)
}

func (h *CatalogHandler) Cover(c *gin.Context) {
	width, err := requestedImageWidth(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "width_px must be a positive integer"})
		return
	}
	cover, err := h.service.GetCoverVariant(c.Param("id"), width)
	if err != nil {
		writeCatalogError(c, err)
		return
	}

	file, err := os.Open(cover.Path)
	if err != nil {
		writeCatalogError(c, err)
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		writeCatalogError(c, err)
		return
	}
	if !info.Mode().IsRegular() {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cover variant is not a regular file"})
		return
	}

	etag := `"` + cover.ContentSHA256 + `"`
	c.Header("Cache-Control", "public, max-age=31536000, immutable")
	c.Header("Content-Type", cover.MediaType)
	c.Header("ETag", etag)
	c.Header("Content-Location", canonicalCoverLocation(c, cover.Width))
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("X-Image-Variant", cover.Key)
	c.Header("X-Image-Width", strconv.Itoa(cover.Width))
	c.Header("X-Image-Height", strconv.Itoa(cover.Height))
	if requestMatchesETag(c.Request, etag) {
		c.Status(http.StatusNotModified)
		return
	}
	http.ServeContent(c.Writer, c.Request, filepath.Base(cover.Path), info.ModTime(), file)
}

func requestedImageWidth(c *gin.Context) (int, error) {
	value := c.Query("width_px")
	if value == "" {
		return 0, catalog.ErrInvalidImageWidth
	}
	width, err := strconv.Atoi(value)
	if err != nil || width <= 0 {
		return 0, catalog.ErrInvalidImageWidth
	}
	return width, nil
}

func canonicalCoverLocation(c *gin.Context, width int) string {
	return c.Request.URL.Path + "?width_px=" + strconv.Itoa(width)
}

func requestMatchesETag(request *http.Request, etag string) bool {
	for _, value := range request.Header.Values("If-None-Match") {
		for _, candidate := range strings.Split(value, ",") {
			candidate = strings.TrimSpace(candidate)
			if candidate == "*" || strings.TrimPrefix(candidate, "W/") == etag {
				return true
			}
		}
	}
	return false
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

func (h *CatalogHandler) ReadingOpen(c *gin.Context) {
	started := time.Now()
	id := c.Param("id")
	readingLog(c, "open start book=%s chapter=%s segment=%s unit=%s", id, c.Query("chapter_id"), c.Query("segment_id"), c.Query("unit"))
	version := c.DefaultQuery("version", "current")
	if version == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "version is required"})
		return
	}
	m, _, err := h.service.LoadReadingManifest(id, version)
	if err != nil {
		writeReadingError(c, err)
		return
	}
	segmentID := c.Query("segment_id")
	chapterID := c.Query("chapter_id")
	unitRaw, hasUnit := c.GetQuery("unit")
	var unit int64
	if hasUnit {
		parsed, parseErr := strconv.ParseInt(unitRaw, 10, 64)
		if parseErr != nil || parsed < 0 || parsed > m.TotalUnits {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid reading unit"})
			return
		}
		unit = parsed
	}
	refs, err := readingSegmentRefs(m)
	if err != nil {
		writeReadingError(c, err)
		return
	}
	chapters := make(map[string]catalog.ReadingChapter, len(m.Chapters))
	for _, chapter := range m.Chapters {
		chapters[chapter.ID] = chapter
	}
	if chapterID != "" {
		if _, ok := chapters[chapterID]; !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": "unknown chapter_id"})
			return
		}
	}
	if segmentID != "" {
		ref, ok := refs[segmentID]
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": "unknown segment_id"})
			return
		}
		if chapterID != "" && ref.ChapterID != chapterID {
			c.JSON(http.StatusBadRequest, gin.H{"error": "chapter_id and segment_id disagree"})
			return
		}
		if hasUnit && unit != m.TotalUnits && (unit < ref.StartUnit || unit >= ref.EndUnit) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "unit and segment_id disagree"})
			return
		}
	}
	var target readingSegmentRef
	if segmentID != "" {
		target = refs[segmentID]
	} else if hasUnit {
		if unit == m.TotalUnits {
			// The end locator belongs to the last segment, rather than the first
			// segment of the last chapter.
			for _, ref := range readingRefsInOrder(m, refs) {
				if ref.EndUnit == m.TotalUnits {
					target = ref
					break
				}
			}
		} else {
			target = findReadingSegment(refs, unit)
		}
		if target.SegmentID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "unit is not readable"})
			return
		}
		if chapterID != "" && target.ChapterID != chapterID {
			c.JSON(http.StatusBadRequest, gin.H{"error": "unit and chapter_id disagree"})
			return
		}
	} else if chapterID != "" {
		chapter := chapters[chapterID]
		for _, ref := range readingRefsInOrder(m, refs) {
			if ref.ChapterID == chapterID {
				target = ref
				break
			}
		}
		if target.SegmentID == "" {
			c.JSON(http.StatusNotFound, gin.H{"error": "chapter has no readable segment"})
			return
		}
		unit = target.StartUnit
		_ = chapter
	} else {
		ordered := readingRefsInOrder(m, refs)
		if len(ordered) == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "reading content has no readable segment"})
			return
		}
		target = ordered[0]
		unit = target.StartUnit
	}
	if !hasUnit && segmentID != "" {
		unit = target.StartUnit
	}
	if hasUnit && unit == m.TotalUnits && target.EndUnit != m.TotalUnits {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unit is outside segment"})
		return
	}
	body, _, err := h.service.LoadReadingFile(id, m.ContentVersion, "segments/"+target.SegmentID+".json")
	if err != nil {
		writeReadingError(c, err)
		return
	}
	var value map[string]any
	if err := json.Unmarshal(body, &value); err != nil {
		writeReadingError(c, err)
		return
	}
	if err := validateReadingSegment(value, id, m.ContentVersion, target.SegmentID); err != nil {
		writeReadingError(c, err)
		return
	}
	resolved := resolveReadingLocator(value, target, unit, m.TotalUnits)
	response := gin.H{"manifest": m, "segment": value, "resolved_locator": resolved}
	readingLog(c, "open ready book=%s segment=%s chapter=%s blocks=%d elapsed_ms=%d", id, target.SegmentID, target.ChapterID, readingBlockCount(value), time.Since(started).Milliseconds())
	writeReadingJSON(c, http.StatusOK, response, false)
}

type readingSegmentRef struct {
	SegmentID, ChapterID string
	StartUnit, EndUnit   int64
}

func readingSegmentRefs(m catalog.ReadingManifest) (map[string]readingSegmentRef, error) {
	refs := make(map[string]readingSegmentRef)
	if len(m.SegmentIndex) > 0 {
		var items []struct {
			SegmentID string `json:"segment_id"`
			ChapterID string `json:"chapter_id"`
			StartUnit int64  `json:"start_unit"`
			EndUnit   int64  `json:"end_unit"`
		}
		if err := json.Unmarshal(m.SegmentIndex, &items); err != nil {
			return nil, fmt.Errorf("decode segment index: %w", err)
		}
		for _, item := range items {
			if item.SegmentID == "" || item.EndUnit < item.StartUnit || item.StartUnit < 0 {
				return nil, fmt.Errorf("invalid segment index")
			}
			refs[item.SegmentID] = readingSegmentRef{item.SegmentID, item.ChapterID, item.StartUnit, item.EndUnit}
		}
	}
	for _, chapter := range m.Chapters {
		for _, id := range chapter.SegmentIDs {
			if _, ok := refs[id]; !ok {
				refs[id] = readingSegmentRef{id, chapter.ID, chapter.StartUnit, chapter.EndUnit}
			} else if refs[id].ChapterID == "" {
				r := refs[id]
				r.ChapterID = chapter.ID
				refs[id] = r
			}
		}
	}
	return refs, nil
}

func readingRefsInOrder(m catalog.ReadingManifest, refs map[string]readingSegmentRef) []readingSegmentRef {
	ordered := make([]readingSegmentRef, 0, len(refs))
	seen := make(map[string]bool)
	for _, item := range m.Chapters {
		for _, id := range item.SegmentIDs {
			if ref, ok := refs[id]; ok && !seen[id] {
				ordered = append(ordered, ref)
				seen[id] = true
			}
		}
	}
	for _, item := range refs {
		if !seen[item.SegmentID] {
			ordered = append(ordered, item)
		}
	}
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].StartUnit < ordered[j].StartUnit })
	return ordered
}

func findReadingSegment(refs map[string]readingSegmentRef, unit int64) readingSegmentRef {
	items := make([]readingSegmentRef, 0, len(refs))
	for _, ref := range refs {
		items = append(items, ref)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].StartUnit < items[j].StartUnit })
	lo, hi := 0, len(items)
	for lo < hi {
		mid := lo + (hi-lo)/2
		if unit < items[mid].StartUnit {
			hi = mid
		} else if unit >= items[mid].EndUnit {
			lo = mid + 1
		} else {
			return items[mid]
		}
	}
	return readingSegmentRef{}
}

func validateReadingSegment(value map[string]any, bookID, version, segmentID string) error {
	for key, expected := range map[string]string{"book_id": bookID, "content_version": version, "segment_id": segmentID} {
		if actual, ok := value[key].(string); ok && actual != expected {
			return fmt.Errorf("reading segment %s mismatch", key)
		}
	}
	return nil
}

func resolveReadingLocator(value map[string]any, target readingSegmentRef, unit, total int64) gin.H {
	chapterID := target.ChapterID
	if v, ok := value["chapter_id"].(string); ok && v != "" {
		chapterID = v
	}
	locator := gin.H{"book_id": value["book_id"], "content_version": value["content_version"], "chapter_id": chapterID, "segment_id": target.SegmentID, "inline_offset": int64(0), "bias": "leading"}
	blocks, _ := value["blocks"].([]any)
	if len(blocks) == 0 {
		return locator
	}
	chosen := -1
	for i, raw := range blocks {
		block, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		start, end, ok := readingBlockRange(block)
		if !ok {
			continue
		}
		if unit >= start && unit < end {
			chosen = i
			break
		}
	}
	if unit >= target.EndUnit || unit >= total {
		chosen = len(blocks) - 1
	}
	if chosen < 0 {
		chosen = 0
	}
	if block, ok := blocks[chosen].(map[string]any); ok {
		start, end, valid := readingBlockRange(block)
		offset := int64(0)
		if valid {
			offset = unit - start
			if offset < 0 {
				offset = 0
			}
			if offset > end-start {
				offset = end - start
			}
		}
		if unit >= target.EndUnit || unit >= total {
			if valid {
				offset = end - start
			}
			locator["bias"] = "trailing"
		}
		locator["inline_offset"] = offset
		locator["block_id"] = block["id"]
		locator["source_block_id"] = block["source_block_id"]
		locator["source_block_index"] = block["source_block_index"]
		// block_index is the stable source block index. A segment may contain
		// only a slice of a chapter, so its local array position is not a
		// locator and must not be returned here.
		locator["block_index"] = block["source_block_index"]
	}
	return locator
}

func readingBlockRange(block map[string]any) (int64, int64, bool) {
	start, okStart := jsonInt64(block["start_unit"])
	end, okEnd := jsonInt64(block["end_unit"])
	return start, end, okStart && okEnd && end >= start
}

func jsonInt64(value any) (int64, bool) {
	switch n := value.(type) {
	case float64:
		return int64(n), n == float64(int64(n))
	case json.Number:
		v, err := n.Int64()
		return v, err == nil
	case int64:
		return n, true
	case int:
		return int64(n), true
	default:
		return 0, false
	}
}

func (h *CatalogHandler) ReadingIndex(c *gin.Context) {
	readingLog(c, "index start book=%s version=%s", c.Param("id"), c.Param("version"))
	m, _, err := h.service.LoadReadingManifest(c.Param("id"), c.Param("version"))
	if err != nil {
		writeReadingError(c, err)
		return
	}
	writeReadingJSON(c, http.StatusOK, m, c.Param("version") != "current")
}

func (h *CatalogHandler) ReadingSegment(c *gin.Context) {
	id, version, segmentID := c.Param("id"), c.Param("version"), c.Param("segment_id")
	readingLog(c, "segment start book=%s version=%s segment=%s", id, version, segmentID)
	m, _, err := h.service.LoadReadingManifest(id, version)
	if err != nil {
		writeReadingError(c, err)
		return
	}
	refs, err := readingSegmentRefs(m)
	if err != nil {
		writeReadingError(c, err)
		return
	}
	if _, ok := refs[segmentID]; !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	body, _, err := h.service.LoadReadingFile(id, m.ContentVersion, "segments/"+segmentID+".json")
	if err != nil {
		writeReadingError(c, err)
		return
	}
	var value map[string]any
	if err := json.Unmarshal(body, &value); err != nil {
		writeReadingError(c, err)
		return
	}
	if err := validateReadingSegment(value, id, m.ContentVersion, segmentID); err != nil {
		writeReadingError(c, err)
		return
	}
	writeReadingBytes(c, http.StatusOK, body, version != "current", "application/json")
	readingLog(c, "segment ready book=%s segment=%s bytes=%d", id, segmentID, len(body))
}

func (h *CatalogHandler) ReadingResource(c *gin.Context) {
	id, version := c.Param("id"), c.Param("version")
	resourceID := strings.TrimPrefix(c.Param("resource_id"), "/")
	readingLog(c, "resource start book=%s version=%s resource=%s", id, version, resourceID)
	// Only resources published in resources.json are addressable. This keeps
	// an otherwise safe relative path from becoming an arbitrary artifact
	// listing endpoint.
	indexBody, _, indexErr := h.service.LoadReadingFile(id, version, "resources.json")
	if indexErr != nil {
		writeReadingError(c, indexErr)
		return
	}
	var resourceIndex map[string]any
	if err := json.Unmarshal(indexBody, &resourceIndex); err != nil {
		writeReadingError(c, err)
		return
	}
	allowed := false
	if _, ok := resourceIndex[resourceID]; ok {
		allowed = true
	} else {
		for _, raw := range resourceIndex {
			if item, ok := raw.(map[string]any); ok && item["resource_id"] == resourceID {
				allowed = true
				break
			}
		}
	}
	if !allowed {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	width, parseErr := requestedImageWidth(c)
	if parseErr != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "width_px must be a positive integer"})
		return
	}
	variant, variantErr := h.readReadingResourceVariant(id, version, resourceID, width)
	if variantErr != nil {
		writeReadingError(c, variantErr)
		return
	}
	file, err := os.Open(variant.Path)
	if err != nil {
		writeReadingError(c, err)
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		writeReadingError(c, err)
		return
	}
	etag := `"` + variant.ETag + `"`
	if version != "current" {
		c.Header("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		c.Header("Cache-Control", "private, no-cache")
	}
	c.Header("ETag", etag)
	c.Header("Content-Type", variant.MediaType)
	c.Header("Content-Location", c.Request.URL.Path+"?width_px="+strconv.Itoa(variant.Width))
	c.Header("X-Image-Variant", variant.VariantKey)
	c.Header("X-Image-Width", strconv.Itoa(variant.Width))
	c.Header("X-Image-Height", strconv.Itoa(variant.Height))
	if etagMatches(c.GetHeader("If-None-Match"), etag) {
		c.Status(http.StatusNotModified)
		return
	}
	limited := &readingRateLimitedResponseWriter{ResponseWriter: c.Writer, ctx: c.Request.Context(), background: true}
	http.ServeContent(limited, c.Request, filepath.Base(variant.Path), info.ModTime(), file)
	readingLog(c, "resource ready book=%s resource=%s variant=%s bytes=%d", id, resourceID, variant.VariantKey, info.Size())
}

type readingImageVariant struct {
	Path       string
	MediaType  string
	ETag       string
	VariantKey string
	Width      int
	Height     int
}

func (h *CatalogHandler) readReadingResourceVariant(bookID, version, resourceID string, width int) (readingImageVariant, error) {
	manifestBody, _, err := h.service.LoadReadingFile(bookID, version, "resources/variants/"+resourceID+"/variants.json")
	if err != nil {
		return readingImageVariant{}, err
	}
	var manifest struct {
		Version  int `json:"version"`
		Variants []struct {
			Key       string `json:"key"`
			Width     int    `json:"width"`
			Height    int    `json:"height"`
			Path      string `json:"path"`
			MediaType string `json:"media_type"`
			SHA256    string `json:"sha256"`
		} `json:"variants"`
	}
	if err := json.Unmarshal(manifestBody, &manifest); err != nil || manifest.Version != 1 {
		return readingImageVariant{}, fmt.Errorf("%w: invalid image variants manifest", catalog.ErrNotFound)
	}
	best := -1
	seenKeys := make(map[string]struct{}, len(manifest.Variants))
	for i, variant := range manifest.Variants {
		if (variant.Key != "small" && variant.Key != "medium" && variant.Key != "large") || variant.Width < 1 || variant.Height < 1 || filepath.Base(variant.Path) != variant.Path || variant.MediaType == "" || len(variant.SHA256) != 64 {
			return readingImageVariant{}, fmt.Errorf("%w: invalid image variant", catalog.ErrNotFound)
		}
		if _, err := hex.DecodeString(variant.SHA256); err != nil {
			return readingImageVariant{}, fmt.Errorf("%w: invalid image variant", catalog.ErrNotFound)
		}
		if _, exists := seenKeys[variant.Key]; exists {
			return readingImageVariant{}, fmt.Errorf("%w: duplicate image variant", catalog.ErrNotFound)
		}
		seenKeys[variant.Key] = struct{}{}
		if best < 0 || absInt(variant.Width-width) < absInt(manifest.Variants[best].Width-width) || (absInt(variant.Width-width) == absInt(manifest.Variants[best].Width-width) && variant.Width > manifest.Variants[best].Width) {
			best = i
		}
	}
	if best < 0 {
		return readingImageVariant{}, fmt.Errorf("%w: no image variants", catalog.ErrNotFound)
	}
	variant := manifest.Variants[best]
	_, path, err := h.service.LoadReadingFile(bookID, version, "resources/variants/"+resourceID+"/"+variant.Path)
	if err != nil {
		return readingImageVariant{}, err
	}
	return readingImageVariant{Path: path, MediaType: variant.MediaType, ETag: strings.ToLower(variant.SHA256), VariantKey: variant.Key, Width: variant.Width, Height: variant.Height}, nil
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func (h *CatalogHandler) ReadingResourcesIndex(c *gin.Context) {
	readingLog(c, "resources-index start book=%s version=%s", c.Param("id"), c.Param("version"))
	body, _, err := h.service.LoadReadingFile(c.Param("id"), c.Param("version"), "resources.json")
	if err != nil {
		writeReadingError(c, err)
		return
	}
	writeReadingBytes(c, http.StatusOK, body, c.Param("version") != "current", "application/json")
	readingLog(c, "resources-index ready book=%s bytes=%d", c.Param("id"), len(body))
}

func readingLog(c *gin.Context, format string, args ...any) {
	trace := c.GetHeader("X-Reader-Trace")
	if trace == "" {
		trace = "-"
	}
	log.Printf("[reading][trace=%s] "+format, append([]any{trace}, args...)...)
}

func readingBlockCount(value map[string]any) int {
	blocks, ok := value["blocks"].([]any)
	if !ok {
		return 0
	}
	return len(blocks)
}

func writeReadingJSON(c *gin.Context, status int, value any, immutable bool) {
	body, err := json.Marshal(gin.H{"data": value})
	if err != nil {
		writeReadingError(c, err)
		return
	}
	writeReadingBytes(c, status, body, immutable, "application/json; charset=utf-8")
}

func writeReadingBytes(c *gin.Context, status int, body []byte, immutable bool, mediaType string) {
	writeReadingBytesPriority(c, status, body, immutable, mediaType, false)
}

func writeReadingBytesPriority(c *gin.Context, status int, body []byte, immutable bool, mediaType string, background bool) {
	writeReadingPayload(c, status, body, immutable, mediaType, background)
}

func writeReadingPayload(c *gin.Context, status int, body []byte, immutable bool, mediaType string, background bool) {
	etag := fmt.Sprintf("\"%x\"", sha256.Sum256(body))
	c.Header("ETag", etag)
	if immutable {
		c.Header("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		c.Header("Cache-Control", "private, no-cache")
	}
	if etagMatches(c.GetHeader("If-None-Match"), etag) {
		c.Status(http.StatusNotModified)
		return
	}
	if strings.Contains(c.GetHeader("Accept-Encoding"), "gzip") && compressibleMediaType(mediaType) {
		var compressed bytes.Buffer
		zw := gzip.NewWriter(&compressed)
		_, _ = zw.Write(body)
		_ = zw.Close()
		body = compressed.Bytes()
		c.Header("Content-Encoding", "gzip")
		c.Header("Vary", "Accept-Encoding")
	}
	c.Header("Content-Type", mediaType)
	c.Header("Content-Length", strconv.Itoa(len(body)))
	if c.Request.Method == http.MethodHead {
		c.Status(status)
		return
	}
	if len(body) == 0 {
		c.Status(status)
		return
	}
	c.Status(status)
	_ = streamReadingBody(c.Request.Context(), c.Writer, body, background)
}

func compressibleMediaType(mediaType string) bool {
	return strings.HasPrefix(mediaType, "application/json") || strings.HasPrefix(mediaType, "text/") || mediaType == "image/svg+xml"
}

func etagMatches(header, etag string) bool {
	for _, candidate := range strings.Split(header, ",") {
		candidate = strings.TrimSpace(candidate)
		if candidate == "*" || candidate == etag || candidate == "W/"+etag {
			return true
		}
	}
	return false
}

func writeReadingError(c *gin.Context, err error) {
	readingLog(c, "request failed status=%d error=%v", readingErrorStatus(err), err)
	if errors.Is(err, catalog.ErrInvalidReadingParam) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid reading parameter"})
		return
	}
	if errors.Is(err, catalog.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	if errors.Is(err, catalog.ErrReadingNotReady) {
		c.JSON(http.StatusConflict, gin.H{"error": "reading content is not ready"})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
}

func readingErrorStatus(err error) int {
	if errors.Is(err, catalog.ErrInvalidReadingParam) {
		return http.StatusBadRequest
	}
	if errors.Is(err, catalog.ErrNotFound) {
		return http.StatusNotFound

	}
	if errors.Is(err, catalog.ErrReadingNotReady) {
		return http.StatusConflict
	}
	return http.StatusInternalServerError
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
	if errors.Is(err, catalog.ErrInvalidImageWidth) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "width_px must be a positive integer"})
		return
	}
	if errors.Is(err, catalog.ErrImageVariantNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "image variant not found"})
		return
	}
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
