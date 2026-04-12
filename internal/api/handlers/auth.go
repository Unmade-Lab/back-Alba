package handlers

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

// Define interface for chat service to avoid circular dependency if any
type OnboardingService interface {
	InitializeOnboarding(ctx context.Context, userID uuid.UUID, workspaceID uuid.UUID) error
}

const (
	accessTokenTTL  = 15 * time.Minute
	refreshTokenTTL = 30 * 24 * time.Hour // 30 days
	bcryptCost      = 12
)

type AuthHandler struct {
	db          *pgxpool.Pool
	logger      *zap.Logger
	jwtSecret   string
	chatService OnboardingService
}

func NewAuthHandler(db *pgxpool.Pool, jwtSecret string, logger *zap.Logger, chatService OnboardingService) *AuthHandler {
	return &AuthHandler{db: db, jwtSecret: jwtSecret, logger: logger, chatService: chatService}
}

// ─────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────

func hashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func checkPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// generateRefreshToken creates a random opaque token and its SHA-256 hash.
func generateRefreshToken() (token, tokenHash string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return
	}
	token = hex.EncodeToString(b)
	sum := sha256.Sum256([]byte(token))
	tokenHash = hex.EncodeToString(sum[:])
	return
}

func (h *AuthHandler) signAccessToken(userID, email, role, workspaceID string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id":      userID,
		"email":        email,
		"role":         role,
		"workspace_id": workspaceID,
		"exp":          time.Now().Add(accessTokenTTL).Unix(),
	})
	return token.SignedString([]byte(h.jwtSecret))
}

// issueTokenPair creates an access token + a refresh token stored in DB.
func (h *AuthHandler) issueTokenPair(ctx context.Context, userID, email, role, workspaceID string) (accessToken, refreshToken string, err error) {
	accessToken, err = h.signAccessToken(userID, email, role, workspaceID)
	if err != nil {
		return
	}

	refreshToken, tokenHash, err := generateRefreshToken()
	if err != nil {
		return "", "", err
	}

	_, err = h.db.Exec(ctx,
		`INSERT INTO refresh_tokens (user_id, workspace_id, token_hash, expires_at)
		 VALUES ($1, $2, $3, $4)`,
		userID, workspaceID, tokenHash, time.Now().Add(refreshTokenTTL),
	)
	if err != nil {
		return "", "", err
	}
	return accessToken, refreshToken, nil
}

// ─────────────────────────────────────────────
// POST /api/v1/auth/register
// ─────────────────────────────────────────────

type RegisterWorkspaceRequest struct {
	WorkspaceName string `json:"workspace_name" binding:"required"`
	UserName      string `json:"user_name" binding:"required"`
	Email         string `json:"email" binding:"required,email"`
	Password      string `json:"password" binding:"required,min=6"`
}

func (h *AuthHandler) RegisterWorkspace(c *gin.Context) {
	var req RegisterWorkspaceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request parameters"})
		return
	}

	ctx := c.Request.Context()

	tx, err := h.db.Begin(ctx)
	if err != nil {
		h.logger.Error("failed to start transaction", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var exists bool
	err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)", req.Email).Scan(&exists)
	if err != nil {
		h.logger.Error("failed to check email existence", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}
	if exists {
		c.JSON(http.StatusConflict, gin.H{"error": "User with this email already exists"})
		return
	}

	// Create workspace
	var workspaceID string
	err = tx.QueryRow(ctx, "INSERT INTO workspaces (name) VALUES ($1) RETURNING id", req.WorkspaceName).Scan(&workspaceID)
	if err != nil {
		h.logger.Error("failed to insert workspace", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create workspace"})
		return
	}

	// Hash password with bcrypt
	passwordHash, err := hashPassword(req.Password)
	if err != nil {
		h.logger.Error("failed to hash password", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	var userID string
	err = tx.QueryRow(ctx,
		`INSERT INTO users (email, name, password_hash, role, workspace_id) VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		req.Email, req.UserName, passwordHash, "admin", workspaceID,
	).Scan(&userID)
	if err != nil {
		h.logger.Error("failed to insert user", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}

	if err := tx.Commit(ctx); err != nil {
		h.logger.Error("failed to commit transaction", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	// Proactively initialize onboarding chat
	go func() {
		uID, _ := uuid.Parse(userID)
		wID, _ := uuid.Parse(workspaceID)
		if err := h.chatService.InitializeOnboarding(context.Background(), uID, wID); err != nil {
			h.logger.Error("failed to initialize onboarding chat", zap.Error(err))
		}
	}()

	accessToken, refreshToken, err := h.issueTokenPair(ctx, userID, req.Email, "admin", workspaceID)
	if err != nil {
		h.logger.Error("failed to issue token pair", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate tokens"})
		return
	}

	h.logger.Info("Workspace and admin user created", zap.String("workspace", workspaceID), zap.String("email", req.Email))

	c.JSON(http.StatusCreated, gin.H{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"user": map[string]string{
			"id":           userID,
			"name":         req.UserName,
			"email":        req.Email,
			"role":         "admin",
			"workspace_id": workspaceID,
		},
		"workspace": map[string]string{
			"id":   workspaceID,
			"name": req.WorkspaceName,
		},
	})
}

// ─────────────────────────────────────────────
// POST /api/v1/auth/login
// ─────────────────────────────────────────────

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: valid email and password are required"})
		return
	}

	ctx := c.Request.Context()

	var userID, name, passwordHash, role, workspaceID string
	err := h.db.QueryRow(ctx, "SELECT id, name, password_hash, role, workspace_id FROM users WHERE email = $1", req.Email).
		Scan(&userID, &name, &passwordHash, &role, &workspaceID)
	if err != nil {
		if err == pgx.ErrNoRows {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
			return
		}
		h.logger.Error("failed to query user for login", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	if !checkPassword(passwordHash, req.Password) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
		return
	}

	accessToken, refreshToken, err := h.issueTokenPair(ctx, userID, req.Email, role, workspaceID)
	if err != nil {
		h.logger.Error("failed to issue token pair", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate tokens"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"user": map[string]string{
			"id":           userID,
			"name":         name,
			"email":        req.Email,
			"role":         role,
			"workspace_id": workspaceID,
		},
	})
}

// ─────────────────────────────────────────────
// POST /api/v1/auth/refresh
// ─────────────────────────────────────────────

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

func (h *AuthHandler) Refresh(c *gin.Context) {
	var req RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "refresh_token is required"})
		return
	}

	ctx := c.Request.Context()

	// Hash the incoming token to look it up
	sum := sha256.Sum256([]byte(req.RefreshToken))
	tokenHash := hex.EncodeToString(sum[:])

	var tokenID, userID, workspaceID string
	var expiresAt time.Time
	err := h.db.QueryRow(ctx,
		`SELECT id, user_id, workspace_id, expires_at
		 FROM refresh_tokens
		 WHERE token_hash = $1`,
		tokenHash,
	).Scan(&tokenID, &userID, &workspaceID, &expiresAt)

	if err != nil {
		if err == pgx.ErrNoRows {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired refresh token"})
			return
		}
		h.logger.Error("failed to query refresh token", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	if time.Now().After(expiresAt) {
		// Clean up expired token
		_, _ = h.db.Exec(ctx, "DELETE FROM refresh_tokens WHERE id = $1", tokenID)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired refresh token"})
		return
	}

	// Load user details
	var email, name, role string
	err = h.db.QueryRow(ctx, "SELECT email, name, role FROM users WHERE id = $1", userID).
		Scan(&email, &name, &role)
	if err != nil {
		h.logger.Error("failed to load user for refresh", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	// Rotate: delete old token, issue new pair
	_, _ = h.db.Exec(ctx, "DELETE FROM refresh_tokens WHERE id = $1", tokenID)

	accessToken, newRefreshToken, err := h.issueTokenPair(ctx, userID, email, role, workspaceID)
	if err != nil {
		h.logger.Error("failed to issue token pair on refresh", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate tokens"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"access_token":  accessToken,
		"refresh_token": newRefreshToken,
	})
}

// ─────────────────────────────────────────────
// POST /api/v1/auth/logout
// ─────────────────────────────────────────────

type LogoutRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

func (h *AuthHandler) Logout(c *gin.Context) {
	var req LogoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "refresh_token is required"})
		return
	}

	ctx := c.Request.Context()

	sum := sha256.Sum256([]byte(req.RefreshToken))
	tokenHash := hex.EncodeToString(sum[:])

	_, err := h.db.Exec(ctx, "DELETE FROM refresh_tokens WHERE token_hash = $1", tokenHash)
	if err != nil {
		h.logger.Error("failed to delete refresh token on logout", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Logged out successfully"})
}

// ─────────────────────────────────────────────
// POST /api/v1/auth/activate
// ─────────────────────────────────────────────

type ActivateRequest struct {
	Token    string `json:"token" binding:"required"`
	Password string `json:"password" binding:"required,min=6"`
}

func (h *AuthHandler) ActivateInvite(c *gin.Context) {
	var req ActivateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: token and password (min 6 chars) are required"})
		return
	}

	ctx := c.Request.Context()

	tx, err := h.db.Begin(ctx)
	if err != nil {
		h.logger.Error("failed to start transaction", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var email, name, role, workspaceID string
	err = tx.QueryRow(ctx,
		`SELECT email, name, role, workspace_id FROM invitations WHERE token = $1 AND expires_at > NOW()`,
		req.Token,
	).Scan(&email, &name, &role, &workspaceID)
	if err != nil {
		if err == pgx.ErrNoRows {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid or expired invitation token"})
			return
		}
		h.logger.Error("failed to query invitation", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	// Hash password with bcrypt
	passwordHash, err := hashPassword(req.Password)
	if err != nil {
		h.logger.Error("failed to hash password", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	var userID string
	err = tx.QueryRow(ctx,
		`INSERT INTO users (email, name, password_hash, role, workspace_id) VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		email, name, passwordHash, role, workspaceID,
	).Scan(&userID)
	if err != nil {
		h.logger.Error("failed to insert user", zap.Error(err))
		c.JSON(http.StatusConflict, gin.H{"error": "User with this email already exists"})
		return
	}

	_, err = tx.Exec(ctx, `DELETE FROM invitations WHERE token = $1`, req.Token)
	if err != nil {
		h.logger.Error("failed to delete invitation", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	if err := tx.Commit(ctx); err != nil {
		h.logger.Error("failed to commit transaction", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	h.logger.Info("User activated via invitation", zap.String("email", email))

	c.JSON(http.StatusOK, gin.H{
		"message":      "Account created and activated successfully",
		"email":        email,
		"role":         role,
		"workspace_id": workspaceID,
		"user_id":      userID,
	})
}

// ─────────────────────────────────────────────
// GET /api/v1/workspace/status
// ─────────────────────────────────────────────

func (h *AuthHandler) WorkspaceStatus(c *gin.Context) {
	widStr := c.GetString("workspace_id")
	if widStr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "workspace_id not found in context"})
		return
	}

	wid, err := uuid.Parse(widStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid workspace_id"})
		return
	}

	var name string
	var onboarded bool
	err = h.db.QueryRow(c.Request.Context(), 
		"SELECT name, onboarding_completed FROM workspaces WHERE id = $1", 
		wid).Scan(&name, &onboarded)

	if err != nil {
		h.logger.Error("failed to query workspace status", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":                   widStr,
		"name":                 name,
		"onboarding_completed": onboarded,
	})
}
