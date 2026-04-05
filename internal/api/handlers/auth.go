package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

type AuthHandler struct {
	db     *pgxpool.Pool
	logger *zap.Logger
}

func NewAuthHandler(db *pgxpool.Pool, logger *zap.Logger) *AuthHandler {
	return &AuthHandler{db: db, logger: logger}
}

type ActivateRequest struct {
	Token    string `json:"token" binding:"required"`
	Password string `json:"password" binding:"required,min=6"`
}

// ActivateInvite POST /api/v1/auth/activate
// Receives an invitation token and a password, verifies the token, creates the User, and deletes the token.
func (h *AuthHandler) ActivateInvite(c *gin.Context) {
	var req ActivateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: token and password (min 6 chars) are required"})
		return
	}

	ctx := c.Request.Context()

	// 1. Begin transaction
	tx, err := h.db.Begin(ctx)
	if err != nil {
		h.logger.Error("failed to start transaction", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}
	defer tx.Rollback(ctx)

	// 2. Find the invitation
	var email, name, role string
	err = tx.QueryRow(ctx, 
		`SELECT email, name, role FROM invitations WHERE token = $1 AND expires_at > NOW()`, 
		req.Token,
	).Scan(&email, &name, &role)

	if err != nil {
		if err == pgx.ErrNoRows {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid or expired invitation token"})
			return
		}
		h.logger.Error("failed to query invitation", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	// 3. Create the user
	// Hash password (using sha256 for simplicity in this example, should use bcrypt in production)
	hash := sha256.Sum256([]byte(req.Password))
	passwordHash := hex.EncodeToString(hash[:])

	_, err = tx.Exec(ctx, 
		`INSERT INTO users (email, name, password_hash, role) VALUES ($1, $2, $3, $4)`,
		email, name, passwordHash, role,
	)
	if err != nil {
		h.logger.Error("failed to insert user", zap.Error(err))
		// E.g., if email already exists
		c.JSON(http.StatusConflict, gin.H{"error": "User with this email already exists"})
		return
	}

	// 4. Delete the used invitation
	_, err = tx.Exec(ctx, `DELETE FROM invitations WHERE token = $1`, req.Token)
	if err != nil {
		h.logger.Error("failed to delete invitation", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	// 5. Commit
	if err := tx.Commit(ctx); err != nil {
		h.logger.Error("failed to commit transaction", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	h.logger.Info("User activated successfully via invitation", zap.String("email", email))

	c.JSON(http.StatusOK, gin.H{
		"message": "Account created and activated successfully",
		"email":   email,
		"role":    role,
	})
}
