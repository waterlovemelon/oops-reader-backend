package handlers

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
)

type HealthHandler struct {
	db *sql.DB
}

func NewHealthHandler(db *sql.DB) *HealthHandler {
	return &HealthHandler{db: db}
}

func (h *HealthHandler) Check(c *gin.Context) {
	ctx := context.Background()

	if err := h.db.PingContext(ctx); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "error",
			"message": "database connection failed",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"message": "service is healthy",
	})
}

func Register(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{
		"error": "Not implemented",
	})
}

func Login(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{
		"error": "Not implemented",
	})
}

func RefreshToken(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{
		"error": "Not implemented",
	})
}

func GetCurrentUser(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{
		"error": "Not implemented",
	})
}

func UpdateCurrentUser(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{
		"error": "Not implemented",
	})
}

func SearchBooks(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{
		"error": "Not implemented",
	})
}

func GetBookByID(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{
		"error": "Not implemented",
	})
}

func ListBookshelf(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{
		"error": "Not implemented",
	})
}

func AddToBookshelf(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{
		"error": "Not implemented",
	})
}

func UpdateBookshelf(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{
		"error": "Not implemented",
	})
}

func DeleteFromBookshelf(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{
		"error": "Not implemented",
	})
}

func UpdateReadingProgress(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{
		"error": "Not implemented",
	})
}

func CreateReadingSession(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{
		"error": "Not implemented",
	})
}

func GetDailyReadingStats(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{
		"error": "Not implemented",
	})
}

func ListNotes(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{
		"error": "Not implemented",
	})
}

func CreateNote(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{
		"error": "Not implemented",
	})
}

func UpdateNote(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{
		"error": "Not implemented",
	})
}

func DeleteNote(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{
		"error": "Not implemented",
	})
}

func GetPreferences(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{
		"error": "Not implemented",
	})
}

func UpdatePreferences(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{
		"error": "Not implemented",
	})
}

func SyncPush(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{
		"error": "Not implemented",
	})
}

func SyncPull(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{
		"error": "Not implemented",
	})
}

type BookParseRequest struct {
	Input string `json:"input" binding:"required"`
}

func ParseBookInfo(c *gin.Context) {
	var req BookParseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request body",
			"details": err.Error(),
		})
		return
	}

	wd, err := filepath.Abs("..")
	if err != nil {
		wd = "/Users/jason/Workspace/Code/oops/reader/oops-reader-backend"
	}

	utilsPath := filepath.Join(wd, "utils")
	scriptPath := filepath.Join(utilsPath, "book_parser.py")

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "python3", scriptPath, req.Input)
	cmd.Dir = utilsPath

	output, err := cmd.CombinedOutput()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to parse book info",
			"details": err.Error(),
			"output": string(output),
		})
		return
	}

	var result map[string]interface{}
	if err := json.Unmarshal(output, &result); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"raw_output": string(output),
			"note": "Could not parse JSON output",
		})
		return
	}

	c.JSON(http.StatusOK, result)
}

func GetBookCover(c *gin.Context) {
	bookName := c.Query("book_name")
	if bookName == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "book_name query parameter is required",
		})
		return
	}

	wd, err := filepath.Abs("..")
	if err != nil {
		wd = "/Users/jason/Workspace/Code/oops/reader/oops-reader-backend"
	}

	utilsPath := filepath.Join(wd, "utils")
	bookCoverPath := filepath.Join(utilsPath, "bookCover.py")

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "python3", bookCoverPath, bookName)
	cmd.Dir = utilsPath

	output, err := cmd.CombinedOutput()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get book cover",
			"details": err.Error(),
			"output": string(output),
		})
		return
	}

	c.Header("Content-Type", "application/json")
	c.String(http.StatusOK, string(output))
}
