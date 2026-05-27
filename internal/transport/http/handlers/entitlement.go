package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/oops-reader/oops-reader-backend/internal/entitlement"
	"github.com/oops-reader/oops-reader-backend/internal/identity"
	"github.com/oops-reader/oops-reader-backend/internal/transport/http/middleware"
)

type EntitlementHandler struct {
	identityService    *identity.Service
	entitlementService *entitlement.Service
}

func NewEntitlementHandler(identityService *identity.Service, entitlementService *entitlement.Service) *EntitlementHandler {
	return &EntitlementHandler{identityService: identityService, entitlementService: entitlementService}
}

func (h *EntitlementHandler) List(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authorization required"})
		return
	}
	account, err := h.identityService.GetUser(c.Request.Context(), userID)
	if err != nil {
		writeIdentityError(c, err)
		return
	}
	result, err := h.entitlementService.ListForUser(userID, account.AccountType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": result})
}
