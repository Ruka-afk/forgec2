package server

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
)

type apiKeyResponse struct {
	ID        uint   `json:"id"`
	Name      string `json:"name"`
	Prefix    string `json:"prefix"`
	LastUsed  string `json:"last_used,omitempty"`
	ExpiresAt string `json:"expires_at,omitempty"`
	Active    bool   `json:"active"`
	CreatedAt string `json:"created_at"`
}

func generateAPIKey() (plaintext string, hash string, prefix string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", "", "", fmt.Errorf("generate random key: %w", err)
	}
	plaintext = "fc2_" + hex.EncodeToString(b)
	h := sha256.Sum256([]byte(plaintext))
	hash = hex.EncodeToString(h[:])
	prefix = plaintext[:12]
	return plaintext, hash, prefix, nil
}

func hashAPIKey(key string) string {
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:])
}

func maskAPIKey(plaintext string) string {
	if len(plaintext) <= 12 {
		return plaintext
	}
	return plaintext[:8] + "..." + plaintext[len(plaintext)-4:]
}

func (s *Server) handleCreateAPIKey(c *gin.Context) {
	var req struct {
		Name      string `json:"name"`
		ExpiresAt string `json:"expires_at,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid request")
		return
	}
	if req.Name == "" {
		respondError(c, http.StatusBadRequest, "name is required")
		return
	}

	userID, _ := c.Get("user_id")

	plaintext, hash, prefix, err := generateAPIKey()
	if err != nil {
		slog.Error("Failed to generate API key", "error", err)
		respondError(c, http.StatusInternalServerError, "failed to generate key")
		return
	}

	apiKey := db.ApiKey{
		UserID:    userID.(uint),
		Name:      req.Name,
		KeyHash:   hash,
		Prefix:    prefix,
		Active:    true,
		CreatedAt: time.Now(),
	}

	if req.ExpiresAt != "" {
		t, err := time.Parse(time.RFC3339, req.ExpiresAt)
		if err != nil {
			respondError(c, http.StatusBadRequest, "invalid expires_at format (use RFC3339)")
			return
		}
		apiKey.ExpiresAt = t
	}

	if err := s.db.Create(&apiKey).Error; err != nil {
		slog.Error("Failed to create API key", "error", err)
		respondError(c, http.StatusInternalServerError, "failed to create key")
		return
	}

	s.LogAuditRecord(c, "api_key_create", "user", fmt.Sprintf("%d", userID.(uint)), fmt.Sprintf("API key created: %s", req.Name), true, nil)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"id":         apiKey.ID,
			"name":       apiKey.Name,
			"key":        plaintext,
			"prefix":     prefix,
			"created_at": apiKey.CreatedAt.Format(time.RFC3339),
			"expires_at": apiKey.ExpiresAt.Format(time.RFC3339),
		},
		"message": "Store this key securely - it will not be shown again",
	})
}

func (s *Server) handleListAPIKeys(c *gin.Context) {
	userID, _ := c.Get("user_id")

	var keys []db.ApiKey
	if err := s.db.Where("user_id = ?", userID).Order("created_at DESC").Find(&keys).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "failed to list keys")
		return
	}

	resp := make([]apiKeyResponse, len(keys))
	for i, k := range keys {
		resp[i] = apiKeyResponse{
			ID:        k.ID,
			Name:      k.Name,
			Prefix:    k.Prefix,
			Active:    k.Active,
			CreatedAt: k.CreatedAt.Format(time.RFC3339),
		}
		if !k.LastUsed.IsZero() {
			resp[i].LastUsed = k.LastUsed.Format(time.RFC3339)
		}
		if !k.ExpiresAt.IsZero() {
			resp[i].ExpiresAt = k.ExpiresAt.Format(time.RFC3339)
		}
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": resp})
}

func (s *Server) handleRevokeAPIKey(c *gin.Context) {
	id := c.Param("id")
	userID, _ := c.Get("user_id")

	var key db.ApiKey
	if err := s.db.Where("id = ? AND user_id = ?", id, userID).First(&key).Error; err != nil {
		respondError(c, http.StatusNotFound, "API key not found")
		return
	}

	if err := s.db.Model(&key).Update("active", false).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "failed to revoke key")
		return
	}

	s.LogAuditRecord(c, "api_key_revoke", "user", fmt.Sprintf("%d", userID.(uint)), fmt.Sprintf("API key revoked: %s (id=%s)", key.Name, id), true, nil)

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "API key revoked"})
}

func (s *Server) handleRotateAPIKey(c *gin.Context) {
	id := c.Param("id")
	userID, _ := c.Get("user_id")

	var oldKey db.ApiKey
	if err := s.db.Where("id = ? AND user_id = ?", id, userID).First(&oldKey).Error; err != nil {
		respondError(c, http.StatusNotFound, "API key not found")
		return
	}

	if err := s.db.Model(&oldKey).Update("active", false).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "failed to deactivate old key")
		return
	}

	plaintext, hash, prefix, err := generateAPIKey()
	if err != nil {
		slog.Error("Failed to generate rotated API key", "error", err)
		respondError(c, http.StatusInternalServerError, "failed to generate key")
		return
	}

	newKey := db.ApiKey{
		UserID:    oldKey.UserID,
		Name:      oldKey.Name,
		KeyHash:   hash,
		Prefix:    prefix,
		ExpiresAt: oldKey.ExpiresAt,
		Active:    true,
		CreatedAt: time.Now(),
	}

	if err := s.db.Create(&newKey).Error; err != nil {
		slog.Error("Failed to create rotated API key", "error", err)
		respondError(c, http.StatusInternalServerError, "failed to create rotated key")
		return
	}

	s.LogAuditRecord(c, "api_key_rotate", "user", fmt.Sprintf("%d", userID.(uint)), fmt.Sprintf("API key rotated: %s (old_id=%s, new_id=%d)", oldKey.Name, id, newKey.ID), true, nil)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"id":         newKey.ID,
			"name":       newKey.Name,
			"key":        plaintext,
			"prefix":     prefix,
			"created_at": newKey.CreatedAt.Format(time.RFC3339),
			"expires_at": newKey.ExpiresAt.Format(time.RFC3339),
		},
		"message": "Store this key securely - it will not be shown again. Old key has been revoked.",
	})
}

// authenticateAPIKey validates an API key from the X-API-Key header.
func (s *Server) authenticateAPIKey(c *gin.Context) bool {
	key := c.GetHeader("X-API-Key")
	if key == "" {
		return false
	}

	hash := hashAPIKey(key)
	var apiKey db.ApiKey
	if err := s.db.Where("key_hash = ? AND active = ?", hash, true).First(&apiKey).Error; err != nil {
		return false
	}

	if !apiKey.ExpiresAt.IsZero() && time.Now().After(apiKey.ExpiresAt) {
		return false
	}

	s.db.Model(&apiKey).Update("last_used", time.Now())

	var user db.User
	if err := s.db.Where("id = ? AND is_active = ?", apiKey.UserID, true).First(&user).Error; err != nil {
		return false
	}

	c.Set("user_id", user.ID)
	c.Set("user", user.Username)
	c.Set("user_role", user.Role)
	return true
}
