package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/oops-reader/oops-reader-backend/internal/catalog"
)

// RecommendationHandler handles HTTP requests for catalog book recommendations.
type RecommendationHandler struct {
	service *catalog.RecommendationService
}

// NewRecommendationHandler creates a new RecommendationHandler.
func NewRecommendationHandler(service *catalog.RecommendationService) *RecommendationHandler {
	return &RecommendationHandler{service: service}
}

// Current returns the current active recommendation.
func (h *RecommendationHandler) Current(c *gin.Context) {
	rec, err := h.service.Current()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if rec == nil {
		c.JSON(http.StatusOK, gin.H{"data": nil})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": h.recommendationJSON(c, *rec)})
}

// History returns a paginated list of published recommendations.
func (h *RecommendationHandler) History(c *gin.Context) {
	page, pageSize := pagination(c)
	recs, total, err := h.service.History(page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	items := make([]gin.H, 0, len(recs))
	for _, rec := range recs {
		items = append(items, h.recommendationJSON(c, rec))
	}
	c.JSON(http.StatusOK, gin.H{
		"data": items,
		"pagination": gin.H{
			"page": page, "page_size": pageSize, "total": total,
		},
	})
}

func (h *RecommendationHandler) recommendationJSON(c *gin.Context, rec catalog.Recommendation) gin.H {
	return gin.H{
		"id":                   rec.ID,
		"book_key":             rec.BookKey,
		"comment":              rec.Comment,
		"scheduled_publish_at": rec.ScheduledPublishAt,
		"created_at":           rec.CreatedAt,
		"book": gin.H{
			"id":            rec.Book.ID,
			"title":         rec.Book.Title,
			"author":        rec.Book.Author,
			"description":   rec.Book.Description,
			"cover_url":     absolutePath(c, "/v1/catalog/books/"+rec.Book.ID+"/cover"),
			"download_url":  absolutePath(c, "/v1/catalog/books/"+rec.Book.ID+"/download"),
			"language":      rec.Book.Language,
			"chapter_count": rec.Book.ChapterCount,
			"word_count":    nullablePositiveInt64(rec.Book.WordCount),
		},
	}
}
