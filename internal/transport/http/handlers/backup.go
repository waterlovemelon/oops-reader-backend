package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/oops-reader/oops-reader-backend/internal/backup"
	"github.com/oops-reader/oops-reader-backend/internal/transport/http/middleware"
)

type BackupHandler struct {
	service *backup.Service
}

func NewBackupHandler(service *backup.Service) *BackupHandler {
	return &BackupHandler{service: service}
}

type uploadBackupRequest struct {
	Mode            backup.UploadMode `json:"mode"`
	DeviceID        string            `json:"device_id" binding:"required"`
	DeviceName      string            `json:"device_name"`
	SchemaVersion   uint              `json:"schema_version" binding:"required"`
	Payload         json.RawMessage   `json:"payload" binding:"required"`
	BookCount       uint              `json:"book_count"`
	NoteCount       uint              `json:"note_count"`
	ProgressCount   uint              `json:"progress_count"`
	PreferenceCount uint              `json:"preference_count"`
}

func (h *BackupHandler) Summary(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authorization required"})
		return
	}
	summary, err := h.service.Summary(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": summary})
}

func (h *BackupHandler) Upload(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authorization required"})
		return
	}
	var req uploadBackupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "details": err.Error()})
		return
	}
	if !json.Valid(req.Payload) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "payload must be valid JSON"})
		return
	}
	saved, err := h.service.Upload(c.Request.Context(), req.Mode, backup.Backup{
		UserID:           userID,
		SchemaVersion:    req.SchemaVersion,
		Payload:          req.Payload,
		BookCount:        req.BookCount,
		NoteCount:        req.NoteCount,
		ProgressCount:    req.ProgressCount,
		PreferenceCount:  req.PreferenceCount,
		SourceDeviceID:   req.DeviceID,
		SourceDeviceName: req.DeviceName,
	})
	if err != nil {
		if errors.Is(err, backup.ErrCloudDataExists) {
			summary, _ := h.service.Summary(c.Request.Context(), userID)
			c.JSON(http.StatusConflict, gin.H{
				"error": "cloud backup already exists",
				"code":  "cloud_data_exists",
				"data":  gin.H{"summary": summary},
			})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": backupJSON(saved)})
}

func (h *BackupHandler) Download(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authorization required"})
		return
	}
	latest, exists, err := h.service.Download(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "backup not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": backupJSON(latest)})
}

func backupJSON(item backup.Backup) gin.H {
	return gin.H{
		"id":                 item.ID,
		"schema_version":     item.SchemaVersion,
		"payload":            json.RawMessage(item.Payload),
		"book_count":         item.BookCount,
		"note_count":         item.NoteCount,
		"progress_count":     item.ProgressCount,
		"preference_count":   item.PreferenceCount,
		"source_device_id":   item.SourceDeviceID,
		"source_device_name": item.SourceDeviceName,
		"created_at":         item.CreatedAt,
		"updated_at":         item.UpdatedAt,
	}
}
