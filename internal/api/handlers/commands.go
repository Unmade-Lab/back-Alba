package handlers

import (
	"net/http"

	"github.com/Unmade-Lab/back-Alba/internal/actions"
	"github.com/Unmade-Lab/back-Alba/internal/convctx"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type CommandHandler struct {
	committer  *actions.Committer
	ctxManager *convctx.Manager
	logger     *zap.Logger
}

func NewCommandHandler(committer *actions.Committer, ctxManager *convctx.Manager, logger *zap.Logger) *CommandHandler {
	return &CommandHandler{committer: committer, ctxManager: ctxManager, logger: logger}
}

type CommitRequest struct {
	SessionID  string                 `json:"session_id"`
	ActionName string                 `json:"action_name"`
	Payload    map[string]interface{} `json:"payload"`
}

// CommitDraft POST /api/v1/commands/commit
func (h *CommandHandler) CommitDraft(c *gin.Context) {
	var req CommitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("invalid commit request", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	if req.ActionName == "" || req.SessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "session_id and action_name are required"})
		return
	}

	ctx := c.Request.Context()

	// 1. Commit the draft to the database
	if err := h.committer.Commit(ctx, req.ActionName, req.Payload); err != nil {
		h.logger.Error("failed to commit draft", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to commit changes"})
		return
	}

	// 2. Clear the draft state from the conversation
	convCtx, err := h.ctxManager.Load(ctx, req.SessionID)
	if err == nil && convCtx.Draft != nil {
		convCtx.Draft = nil
		h.ctxManager.Save(ctx, convCtx)
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Draft committed successfully",
	})
}
