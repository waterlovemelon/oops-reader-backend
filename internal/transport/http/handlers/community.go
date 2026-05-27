package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/oops-reader/oops-reader-backend/internal/community"
	"github.com/oops-reader/oops-reader-backend/internal/transport/http/middleware"
)

type CommunityHandler struct {
	service *community.Service
}

func NewCommunityHandler(service *community.Service) *CommunityHandler {
	return &CommunityHandler{service: service}
}

type createThreadRequest struct {
	BoardID        string `json:"board_id" binding:"required"`
	Title          string `json:"title" binding:"required"`
	Content        string `json:"content" binding:"required"`
	OptionalBookID string `json:"optional_book_id"`
}

type addCommentRequest struct {
	ParentCommentID string `json:"parent_comment_id"`
	Content         string `json:"content" binding:"required"`
}

type reactRequest struct {
	TargetType   string `json:"target_type" binding:"required"`
	TargetID     string `json:"target_id" binding:"required"`
	ReactionType string `json:"reaction_type" binding:"required"`
}

func (h *CommunityHandler) ListBoards(c *gin.Context) {
	boards := h.service.ListBoards()
	data := make([]gin.H, 0, len(boards))
	for _, board := range boards {
		data = append(data, boardJSON(board))
	}
	c.JSON(http.StatusOK, gin.H{"data": data})
}

func (h *CommunityHandler) ListThreads(c *gin.Context) {
	page, pageSize := pagination(c)
	threads, err := h.service.ListThreads(c.Query("board_id"), page, pageSize)
	if err != nil {
		writeCommunityError(c, err)
		return
	}
	data := make([]gin.H, 0, len(threads))
	for _, thread := range threads {
		data = append(data, threadJSON(thread))
	}
	c.JSON(http.StatusOK, gin.H{"data": data, "pagination": gin.H{"page": page, "page_size": pageSize}})
}

func (h *CommunityHandler) CreateThread(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authorization required"})
		return
	}
	var req createThreadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "details": err.Error()})
		return
	}
	thread, err := h.service.CreateThread(strconv.FormatUint(userID, 10), req.BoardID, req.Title, req.Content, req.OptionalBookID)
	if err != nil {
		writeCommunityError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": threadJSON(thread)})
}

func (h *CommunityHandler) GetThread(c *gin.Context) {
	thread, err := h.service.GetThread(c.Param("id"))
	if err != nil {
		writeCommunityError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": threadJSON(thread)})
}

func (h *CommunityHandler) AddComment(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authorization required"})
		return
	}
	var req addCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "details": err.Error()})
		return
	}
	comment, err := h.service.AddComment(strconv.FormatUint(userID, 10), c.Param("id"), req.ParentCommentID, req.Content)
	if err != nil {
		writeCommunityError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": commentJSON(comment)})
}

func (h *CommunityHandler) React(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authorization required"})
		return
	}
	var req reactRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "details": err.Error()})
		return
	}
	reaction, err := h.service.React(strconv.FormatUint(userID, 10), req.TargetType, req.TargetID, req.ReactionType)
	if err != nil {
		writeCommunityError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"id": reaction.ID, "target_type": reaction.TargetType, "target_id": reaction.TargetID, "reaction_type": reaction.ReactionType,
	}})
}

func boardJSON(board community.Board) gin.H {
	return gin.H{"id": board.ID, "name": board.Name, "description": board.Description, "status": board.Status}
}

func threadJSON(thread community.Thread) gin.H {
	comments := make([]gin.H, 0, len(thread.Comments))
	for _, comment := range thread.Comments {
		comments = append(comments, commentJSON(comment))
	}
	return gin.H{
		"id": thread.ID, "board_id": thread.BoardID, "user_id": thread.UserID, "title": thread.Title, "content": thread.Content,
		"optional_book_id": thread.OptionalBookID, "status": thread.Status, "comment_count": thread.CommentCount,
		"reaction_counts": thread.ReactionCounts, "comments": comments, "created_at": thread.CreatedAt, "updated_at": thread.UpdatedAt,
	}
}

func commentJSON(comment community.Comment) gin.H {
	return gin.H{
		"id": comment.ID, "thread_id": comment.ThreadID, "user_id": comment.UserID, "parent_comment_id": comment.ParentCommentID,
		"content": comment.Content, "status": comment.Status, "reaction_counts": comment.ReactionCounts, "created_at": comment.CreatedAt,
	}
}

func writeCommunityError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, community.ErrInvalidInput):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, community.ErrBoardNotFound), errors.Is(err, community.ErrThreadNotFound), errors.Is(err, community.ErrCommentNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}
