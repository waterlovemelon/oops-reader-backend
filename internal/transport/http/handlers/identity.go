package handlers

import (
	"errors"
	"io"
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

type registerRequest struct {
	Email      string `json:"email" binding:"required"`
	Password   string `json:"password" binding:"required"`
	Nickname   string `json:"nickname"`
	DeviceID   string `json:"device_id" binding:"required"`
	DeviceName string `json:"device_name"`
	Platform   string `json:"platform" binding:"required"`
}

type loginRequest struct {
	Email      string `json:"email" binding:"required"`
	Password   string `json:"password" binding:"required"`
	DeviceID   string `json:"device_id" binding:"required"`
	DeviceName string `json:"device_name"`
	Platform   string `json:"platform" binding:"required"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

type logoutRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type passwordResetRequest struct {
	Email string `json:"email" binding:"required"`
}

type passwordResetConfirmRequest struct {
	Token       string `json:"token" binding:"required"`
	NewPassword string `json:"new_password" binding:"required"`
}

type updateProfileRequest struct {
	Nickname  string `json:"nickname" binding:"required"`
	AvatarURL string `json:"avatar_url"`
}

func (h *IdentityHandler) Register(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "details": err.Error()})
		return
	}
	result, err := h.service.Register(c.Request.Context(), identity.RegisterInput{
		Email:      req.Email,
		Password:   req.Password,
		Nickname:   req.Nickname,
		DeviceID:   req.DeviceID,
		DeviceName: req.DeviceName,
		Platform:   req.Platform,
	})
	if err != nil {
		writeIdentityError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": authResultJSON(result)})
}

func (h *IdentityHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "details": err.Error()})
		return
	}
	result, err := h.service.Login(c.Request.Context(), identity.LoginInput{
		Email:      req.Email,
		Password:   req.Password,
		DeviceID:   req.DeviceID,
		DeviceName: req.DeviceName,
		Platform:   req.Platform,
	})
	if err != nil {
		writeIdentityError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": authResultJSON(result)})
}

func (h *IdentityHandler) Refresh(c *gin.Context) {
	var req refreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "details": err.Error()})
		return
	}
	result, err := h.service.Refresh(c.Request.Context(), req.RefreshToken)
	if err != nil {
		writeIdentityError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": authResultJSON(result)})
}

func (h *IdentityHandler) Logout(c *gin.Context) {
	var req logoutRequest
	if c.Request.Body != nil {
		if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "details": err.Error()})
			return
		}
	}
	if err := h.service.Logout(c.Request.Context(), req.RefreshToken); err != nil {
		writeIdentityError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *IdentityHandler) RequestPasswordReset(c *gin.Context) {
	var req passwordResetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "details": err.Error()})
		return
	}
	if err := h.service.RequestPasswordReset(c.Request.Context(), req.Email); err != nil {
		writeIdentityError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"message": "If the account exists, a reset email will be sent."}})
}

func (h *IdentityHandler) ConfirmPasswordReset(c *gin.Context) {
	var req passwordResetConfirmRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "details": err.Error()})
		return
	}
	if err := h.service.ConfirmPasswordReset(c.Request.Context(), req.Token, req.NewPassword); err != nil {
		writeIdentityError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"message": "Password has been reset."}})
}

func (h *IdentityHandler) GetCurrentUser(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authorization required"})
		return
	}
	user, err := h.service.GetUser(c.Request.Context(), userID)
	if err != nil {
		writeIdentityError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": accountJSON(user)})
}

func (h *IdentityHandler) UpdateCurrentUser(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authorization required"})
		return
	}
	var req updateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "details": err.Error()})
		return
	}
	user, err := h.service.UpdateProfile(c.Request.Context(), userID, req.Nickname, req.AvatarURL)
	if err != nil {
		writeIdentityError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": accountJSON(user)})
}

func authResultJSON(result identity.AuthResult) gin.H {
	return gin.H{
		"user":          accountJSON(result.User),
		"access_token":  result.AccessToken,
		"refresh_token": result.RefreshToken,
		"token_type":    result.TokenType,
	}
}

func accountJSON(account identity.Account) gin.H {
	return gin.H{
		"id":                account.ID,
		"email":             account.Email,
		"phone":             account.Phone,
		"nickname":          account.Nickname,
		"avatar_url":        account.AvatarURL,
		"account_status":    account.AccountStatus,
		"account_type":      account.AccountType,
		"email_verified_at": account.EmailVerifiedAt,
		"last_login_at":     account.LastLoginAt,
		"created_at":        account.CreatedAt,
		"updated_at":        account.UpdatedAt,
	}
}

func writeIdentityError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, identity.ErrInvalidInput):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, identity.ErrDuplicateEmail):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, identity.ErrInvalidCredentials), errors.Is(err, identity.ErrInvalidToken), errors.Is(err, identity.ErrExpiredToken), errors.Is(err, identity.ErrRefreshRevoked):
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
	case errors.Is(err, identity.ErrAccountForbidden):
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
	case errors.Is(err, identity.ErrUserNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}
