package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/oops-reader/oops-reader-backend/internal/device"
	"github.com/oops-reader/oops-reader-backend/internal/transport/http/middleware"
)

// DeviceHandler serves the account's device list.
type DeviceHandler struct {
	service *device.Service
}

// NewDeviceHandler creates a DeviceHandler.
func NewDeviceHandler(service *device.Service) *DeviceHandler {
	return &DeviceHandler{service: service}
}

// List returns the devices this account has used, most recently active first.
//
//	GET /v1/account/devices
func (h *DeviceHandler) List(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authorization required"})
		return
	}
	devices, err := h.service.List(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	items := make([]gin.H, 0, len(devices))
	for _, item := range devices {
		items = append(items, gin.H{
			"device_id":     item.DeviceID,
			"device_name":   item.DeviceName,
			"platform":      item.Platform,
			"first_seen_at": item.FirstSeenAt,
			"last_seen_at":  item.LastSeenAt,
		})
	}
	c.JSON(http.StatusOK, gin.H{"data": items, "total": len(items)})
}
