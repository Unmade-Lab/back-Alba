package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

type AuthHandler struct {
	db        *pgxpool.Pool
	logger    *zap.Logger
	jwtSecret string
}

func NewAuthHandler(db *pgxpool.Pool, jwtSecret string, logger *zap.Logger) *AuthHandler {
	return &AuthHandler{db: db, jwtSecret: jwtSecret, logger: logger}
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

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// Login POST /api/v1/auth/login
// Verifies credentials and issues a JWT token
func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: valid email and password are required"})
		return
	}

	ctx := c.Request.Context()

	var userID, name, passwordHash, role string
	err := h.db.QueryRow(ctx, "SELECT id, name, password_hash, role FROM users WHERE email = $1", req.Email).
		Scan(&userID, &name, &passwordHash, &role)

	if err != nil {
		if err == pgx.ErrNoRows {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
			return
		}
		h.logger.Error("failed to query user for login", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	// Verify password hash
	hash := sha256.Sum256([]byte(req.Password))
	if hex.EncodeToString(hash[:]) != passwordHash {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
		return
	}

	// Generate JWT
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": userID,
		"email":   req.Email,
		"role":    role,
		"exp":     time.Now().Add(24 * time.Hour).Unix(),
	})

	tokenString, err := token.SignedString([]byte(h.jwtSecret))
	if err != nil {
		h.logger.Error("failed to sign token", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token": tokenString,
		"user": map[string]string{
			"id":    userID,
			"name":  name,
			"email": req.Email,
			"role":  role,
		},
	})
}

