package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/oops-reader/oops-reader-backend/internal/community"
	"github.com/oops-reader/oops-reader-backend/internal/transport/http/middleware"
)

type CommunityHandler struct {
	service   *community.Service
	storage   community.AttachmentStorage
}

func NewCommunityHandler(service *community.Service, storage community.AttachmentStorage) *CommunityHandler {
	return &CommunityHandler{service: service, storage: storage}
}

type createThreadRequest struct {
	BoardID        string   `json:"board_id" binding:"required"`
	Title          string   `json:"title" binding:"required"`
	Content        string   `json:"content" binding:"required"`
	OptionalBookID string   `json:"optional_book_id"`
	AttachmentIDs  []string `json:"attachment_ids"`
}

type addCommentRequest struct {
	ParentCommentID string   `json:"parent_comment_id"`
	Content         string   `json:"content" binding:"required"`
	AttachmentIDs   []string `json:"attachment_ids"`
}

type reactRequest struct {
	TargetType   string `json:"target_type" binding:"required"`
	TargetID     string `json:"target_id" binding:"required"`
	ReactionType string `json:"reaction_type" binding:"required"`
}

func (h *CommunityHandler) ListBoards(c *gin.Context) {
	boards, err := h.service.ListBoards(c.Request.Context())
	if err != nil {
		writeCommunityError(c, err)
		return
	}
	data := make([]gin.H, 0, len(boards))
	for _, board := range boards {
		data = append(data, boardJSON(board))
	}
	c.JSON(http.StatusOK, gin.H{"data": data})
}

func (h *CommunityHandler) ListThreads(c *gin.Context) {
	page, pageSize := pagination(c)
	threads, err := h.service.ListThreads(c.Request.Context(), c.Query("board_id"), page, pageSize)
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
	thread, err := h.service.CreateThread(c.Request.Context(),
		strconv.FormatUint(userID, 10), req.BoardID, req.Title, req.Content, req.OptionalBookID, req.AttachmentIDs)
	if err != nil {
		writeCommunityError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": threadJSON(thread)})
}

func (h *CommunityHandler) GetThread(c *gin.Context) {
	thread, err := h.service.GetThread(c.Request.Context(), c.Param("id"))
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
	comment, err := h.service.AddComment(c.Request.Context(),
		strconv.FormatUint(userID, 10), c.Param("id"), req.ParentCommentID, req.Content, req.AttachmentIDs)
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
	reaction, err := h.service.React(c.Request.Context(),
		strconv.FormatUint(userID, 10), req.TargetType, req.TargetID, req.ReactionType)
	if err != nil {
		writeCommunityError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"id": reaction.ID, "target_type": reaction.TargetType, "target_id": reaction.TargetID, "reaction_type": reaction.ReactionType,
	}})
}

// UploadAttachment handles multipart image upload for community attachments.
func (h *CommunityHandler) UploadAttachment(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authorization required"})
		return
	}

	const maxUploadBytes = 5 * 1024 * 1024 // 5 MB
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxUploadBytes)

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required or too large, max 5 MB", "details": err.Error()})
		return
	}
	defer file.Close()

	// Secondary check on reported size (FormFile may not trigger MaxBytesReader for small files).
	if header.Size > maxUploadBytes {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file too large, max 5 MB"})
		return
	}

	stored, err := h.storage.Save(c.Request.Context(), community.SaveAttachmentInput{
		OwnerUserID: strconv.FormatUint(userID, 10),
		Filename:    header.Filename,
		Reader:      file,
	})
	if err != nil {
		writeCommunityError(c, err)
		return
	}

	// Create attachment record in DB.
	att, err := h.service.CreateAttachment(c.Request.Context(), community.CreateAttachmentInput{
		OwnerUserID:     strconv.FormatUint(userID, 10),
		FileType:        community.FileTypeImage,
		StorageProvider: "local",
		StorageKey:      stored.StorageKey,
		PublicURL:       stored.PublicURL,
		MIMEType:        stored.MIMEType,
		FileSize:        stored.FileSize,
		Width:           stored.Width,
		Height:          stored.Height,
		ChecksumSHA256:  stored.ChecksumSHA256,
	})
	if err != nil {
		writeCommunityError(c, err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": attachmentJSON(att)})
}

// ── JSON serializers ─────────────────────────────────────────────────────────

func boardJSON(board community.Board) gin.H {
	return gin.H{"id": board.ID, "name": board.Name, "description": board.Description, "status": board.Status}
}

func threadJSON(thread community.Thread) gin.H {
	comments := make([]gin.H, 0, len(thread.Comments))
	for _, comment := range thread.Comments {
		comments = append(comments, commentJSON(comment))
	}
	attachments := make([]gin.H, 0, len(thread.Attachments))
	for _, att := range thread.Attachments {
		attachments = append(attachments, attachmentBriefJSON(att))
	}
	return gin.H{
		"id": thread.ID, "board_id": thread.BoardID, "user_id": thread.UserID, "title": thread.Title, "content": thread.Content,
		"optional_book_id": thread.OptionalBookID, "status": thread.Status, "comment_count": thread.CommentCount,
		"reaction_counts": thread.ReactionCounts, "attachments": attachments, "comments": comments,
		"created_at": thread.CreatedAt, "updated_at": thread.UpdatedAt,
	}
}

func commentJSON(comment community.Comment) gin.H {
	attachments := make([]gin.H, 0, len(comment.Attachments))
	for _, att := range comment.Attachments {
		attachments = append(attachments, attachmentBriefJSON(att))
	}
	return gin.H{
		"id": comment.ID, "thread_id": comment.ThreadID, "user_id": comment.UserID, "parent_comment_id": comment.ParentCommentID,
		"content": comment.Content, "status": comment.Status, "reaction_counts": comment.ReactionCounts,
		"attachments": attachments, "created_at": comment.CreatedAt,
	}
}

func attachmentJSON(att community.Attachment) gin.H {
	return gin.H{
		"id": att.ID, "file_type": att.FileType, "mime_type": att.MIMEType,
		"file_size": att.FileSize, "width": att.Width, "height": att.Height,
		"status": att.Status,
	}
}

func attachmentBriefJSON(att community.Attachment) gin.H {
	return gin.H{
		"id": att.ID, "file_type": att.FileType, "url": att.PublicURL,
		"mime_type": att.MIMEType, "width": att.Width, "height": att.Height,
	}
}

func writeCommunityError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, community.ErrInvalidInput):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, community.ErrBoardNotFound),
		errors.Is(err, community.ErrThreadNotFound),
		errors.Is(err, community.ErrCommentNotFound),
		errors.Is(err, community.ErrAttachmentNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, community.ErrAttachmentNotOwned),
		errors.Is(err, community.ErrAttachmentAlreadyBound):
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
	case errors.Is(err, community.ErrAttachmentLimitExceeded):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("internal error: %v", err)})
	}
}
