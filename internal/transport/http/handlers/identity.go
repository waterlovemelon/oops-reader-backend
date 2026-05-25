package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/oops-reader/oops-reader-backend/internal/identity"
	"github.com/oops-reader/oops-reader-backend/internal/transport/http/middleware"
)

type IdentityHandler struct {
	service *identity.Service
}

func NewIdentityHandler(service *identity.Service) *IdentityHandler {
	return &IdentityHandler{service: service}
}

type createGuestRequest struct {
	DeviceID string `json:"device_id" binding:"required"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

type bindRequest struct {
	Email    string `json:"email"`
	Phone    string `json:"phone"`
	Nickname string `json:"nickname"`
}

func (h *IdentityHandler) CreateGuest(c *gin.Context) {
	var req createGuestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "details": err.Error()})
		return
	}

	user, accessToken, refreshToken, err := h.service.CreateGuest(req.DeviceID)
	if err != nil {
		writeIdentityError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": gin.H{
		"user":          userJSON(user),
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"token_type":    "Bearer",
	}})
}

func (h *IdentityHandler) Refresh(c *gin.Context) {
	var req refreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "details": err.Error()})
		return
	}

	accessToken, refreshToken, err := h.service.Refresh(req.RefreshToken)
	if err != nil {
		writeIdentityError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"token_type":    "Bearer",
	}})
}

func (h *IdentityHandler) Bind(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authorization required"})
		return
	}

	var req bindRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "details": err.Error()})
		return
	}
	user, err := h.service.Bind(userID, req.Email, req.Phone, req.Nickname)
	if err != nil {
		writeIdentityError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": userJSON(user)})
}

func (h *IdentityHandler) GetCurrentUser(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authorization required"})
		return
	}
	user, err := h.service.GetUser(userID)
	if err != nil {
		writeIdentityError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": userJSON(user)})
}

func userJSON(user identity.User) gin.H {
	return gin.H{
		"id":         user.ID,
		"device_id":  user.DeviceID,
		"status":     user.Status,
		"email":      user.Email,
		"phone":      user.Phone,
		"nickname":   user.Nickname,
		"created_at": user.CreatedAt,
		"updated_at": user.UpdatedAt,
	}
}

func writeIdentityError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, identity.ErrInvalidInput):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, identity.ErrInvalidToken), errors.Is(err, identity.ErrExpiredToken), errors.Is(err, identity.ErrRefreshRevoked):
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
	case errors.Is(err, identity.ErrUserNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}
