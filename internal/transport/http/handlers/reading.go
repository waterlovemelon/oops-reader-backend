package handlers

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/oops-reader/oops-reader-backend/internal/reading"
	"github.com/oops-reader/oops-reader-backend/internal/transport/http/middleware"
)

// ReadingHandler serves the account's stored reading position.
type ReadingHandler struct {
	service *reading.ProgressService
}

// NewReadingHandler creates a ReadingHandler.
func NewReadingHandler(service *reading.ProgressService) *ReadingHandler {
	return &ReadingHandler{service: service}
}

type upsertProgressRequest struct {
	BookKey         string   `json:"book_key"`
	ProgressType    string   `json:"progress_type"`
	ProgressValue   string   `json:"progress_value"`
	ProgressPercent *float64 `json:"progress_percent"`
	ChapterTitle    string   `json:"chapter_title"`
	PositionCFI     string   `json:"position_cfi"`
	ContentVersion  string   `json:"content_version"`
	DeviceID        string   `json:"device_id"`
	RecordedAt      string   `json:"recorded_at"`
	OperationID     string   `json:"operation_id"`
}

// UpsertProgress merges the reading position of one book into the account's
// stored position. An upload that loses the merge is answered with the winning
// row and applied=false, so the client can write it back locally.
//
//	PUT /v1/reading/progress
func (h *ReadingHandler) UpsertProgress(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authorization required"})
		return
	}

	var req upsertProgressRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	var recordedAt *time.Time
	if raw := strings.TrimSpace(req.RecordedAt); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "recorded_at must be an RFC3339 timestamp"})
			return
		}
		recordedAt = &parsed
	}

	result, err := h.service.Upsert(c.Request.Context(), userID, reading.ProgressInput{
		BookKey:         req.BookKey,
		ProgressType:    req.ProgressType,
		ProgressValue:   req.ProgressValue,
		ProgressPercent: req.ProgressPercent,
		ChapterTitle:    req.ChapterTitle,
		PositionCFI:     req.PositionCFI,
		ContentVersion:  req.ContentVersion,
		DeviceID:        req.DeviceID,
		RecordedAt:      recordedAt,
		OperationID:     req.OperationID,
	})
	if err != nil {
		writeProgressError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"data":       readingProgressJSON(result.Progress),
		"applied":    result.Applied,
		"duplicate":  result.Duplicate,
		"clock_skew": result.ClockSkew,
	})
}

// ListProgress returns one book's progress when book_key is supplied, otherwise
// a page of the account's progress ordered by most recently updated.
//
//	GET /v1/reading/progress?book_key=<key>
//	GET /v1/reading/progress?page=1&page_size=20
func (h *ReadingHandler) ListProgress(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authorization required"})
		return
	}

	if bookKey := strings.TrimSpace(c.Query("book_key")); bookKey != "" {
		progress, err := h.service.Get(c.Request.Context(), userID, bookKey)
		if err != nil {
			writeProgressError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": readingProgressJSON(progress)})
		return
	}

	page, pageSize := pagination(c)
	items, total, err := h.service.List(c.Request.Context(), userID, page, pageSize)
	if err != nil {
		writeProgressError(c, err)
		return
	}
	progress := make([]gin.H, 0, len(items))
	for _, item := range items {
		progress = append(progress, readingProgressJSON(item))
	}
	c.JSON(http.StatusOK, gin.H{
		"data": progress,
		"pagination": gin.H{
			"page": page, "page_size": pageSize, "total": total,
		},
	})
}

func readingProgressJSON(progress reading.Progress) gin.H {
	return gin.H{
		"book_key":         progress.BookKey,
		"progress_type":    progress.ProgressType,
		"progress_value":   progress.ProgressValue,
		"progress_percent": progress.ProgressPercent,
		"chapter_title":    progress.ChapterTitle,
		"position_cfi":     progress.PositionCFI,
		"content_version":  progress.ContentVersion,
		"device_id":        progress.DeviceID,
		"recorded_at":      progress.RecordedAt,
		"updated_at":       progress.UpdatedAt,
	}
}

func writeProgressError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, reading.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "reading progress not found"})
	case errors.Is(err, reading.ErrUnknownBook):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, reading.ErrInvalidBookKey),
		errors.Is(err, reading.ErrUnsupportedBookKey),
		errors.Is(err, reading.ErrInvalidProgressType),
		errors.Is(err, reading.ErrInvalidProgressValue),
		errors.Is(err, reading.ErrInvalidPercent),
		errors.Is(err, reading.ErrInvalidChapterTitle),
		errors.Is(err, reading.ErrInvalidPositionCFI),
		errors.Is(err, reading.ErrInvalidContentVersion),
		errors.Is(err, reading.ErrInvalidDeviceID),
		errors.Is(err, reading.ErrInvalidOperationID):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}
