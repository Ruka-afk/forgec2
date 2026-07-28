package server

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/forgec2/forgec2/internal/config"
	"github.com/gin-gonic/gin"
)

// respond sends a JSON 200 response with the given payload.
func respond(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, data)
}

// respondSuccess wraps data in the standard success envelope: {success: true, data: ...}.
// Use this for all list/detail endpoints to ensure a consistent API response format.
func respondSuccess(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}

// respondSuccessWithTotal wraps data with pagination metadata.
func respondSuccessWithTotal(c *gin.Context, data interface{}, total int64, page, pageSize int) {
	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"data":     data,
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
	})
}

// respondError sends a JSON error response with the given HTTP status and message.
func respondError(c *gin.Context, status int, msg string) {
	c.JSON(status, gin.H{"success": false, "error": msg})
}

// respondErrorSafe sends a JSON error response with the error message sanitized
// through sanitizeError, so raw err.Error() is never exposed to clients.
func respondErrorSafe(c *gin.Context, status int, err error, context string) {
	respondError(c, status, sanitizeError(err, context))
}

// sanitizeError maps known internal errors to safe user-facing messages.
// Unknown errors return a generic message — never expose err.Error() to clients.
func sanitizeError(err error, context string) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "duplicate") || strings.Contains(msg, "UNIQUE"):
		return "A record with the same value already exists"
	case strings.Contains(msg, "no such table") || strings.Contains(msg, "no such column"):
		return "Database schema error — restart the server to apply migrations"
	case strings.Contains(msg, "foreign key") || strings.Contains(msg, "FOREIGN KEY"):
		return "Cannot complete: related records still reference this item"
	case strings.Contains(msg, "record not found") || strings.Contains(msg, "ErrNoRows"):
		return "Record not found"
	case strings.Contains(msg, "permission denied") || strings.Contains(msg, "access denied"):
		return "Permission denied"
	case strings.Contains(msg, "connection refused"):
		return "External service is unreachable"
	case strings.Contains(msg, "timeout") || strings.Contains(msg, "deadline exceeded"):
		return "Operation timed out"
	case strings.Contains(msg, "no space left"):
		return "Disk space is full"
	default:
		if context != "" {
			return context + " failed"
		}
		return "An internal error occurred"
	}
}

var sanitizeRe = regexp.MustCompile(`[^\w.\-]`)

// registerBoth registers a handler under both /api/<path> and /<path>.
func registerBoth(rg *gin.RouterGroup, method, path string, handler gin.HandlerFunc) {
	apiPath := "/api" + path
	switch method {
	case http.MethodGet:
		rg.GET(apiPath, handler)
		rg.GET(path, handler)
	case http.MethodPost:
		rg.POST(apiPath, handler)
		rg.POST(path, handler)
	case http.MethodPut:
		rg.PUT(apiPath, handler)
		rg.PUT(path, handler)
	case http.MethodDelete:
		rg.DELETE(apiPath, handler)
		rg.DELETE(path, handler)
	}
}

// --- #5: File upload helper ---

// readFileUpload reads a multipart file upload, validates size, and returns the
// bytes and filename. Sends an error response and returns nil + false on failure.
func readFileUpload(c *gin.Context, fieldName string) ([]byte, string, bool) {
	file, err := c.FormFile(fieldName)
	if err != nil {
		respondError(c, http.StatusBadRequest, fieldName+" file required")
		return nil, "", false
	}
	if file.Size > MaxUploadSize {
		respondError(c, http.StatusBadRequest, fmt.Sprintf("file too large (max %d bytes)", MaxUploadSize))
		return nil, "", false
	}
	f, err := file.Open()
	if err != nil {
		respondError(c, http.StatusInternalServerError, "failed to open uploaded file")
		return nil, "", false
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "failed to read uploaded file")
		return nil, "", false
	}
	return data, file.Filename, true
}

// --- #6: DB lookup helper ---

// requireAdmin checks that the current user is an admin. Returns false and sends
// a 403 response if not.
func (s *Server) requireAdmin(c *gin.Context) bool {
	role, _ := c.Get("user_role")
	if role != "admin" {
		respondError(c, http.StatusForbidden, "admin role required")
		return false
	}
	return true
}

// requireOperator checks that the current user has operator or admin role.
// Returns false and sends a 403 response for viewers.
func (s *Server) requireOperator(c *gin.Context) bool {
	role, _ := c.Get("user_role")
	if role == "viewer" || role == "" {
		respondError(c, http.StatusForbidden, "operator role required")
		return false
	}
	return true
}

// findOrFail fetches a record by primary key. Returns true if found, or sends a
// 404 response and returns false.
func (s *Server) findOrFail(c *gin.Context, dest interface{}, id, entityName string) bool {
	if err := s.db.First(dest, "id = ?", id).Error; err != nil {
		respondError(c, http.StatusNotFound, entityName+" not found")
		return false
	}
	return true
}

// findOrFailPreload fetches a record with preloaded associations by primary key.
func (s *Server) findOrFailPreload(c *gin.Context, dest interface{}, id, entityName string, preloads ...string) bool {
	q := s.db
	for _, p := range preloads {
		q = q.Preload(p)
	}
	if err := q.First(dest, "id = ?", id).Error; err != nil {
		respondError(c, http.StatusNotFound, entityName+" not found")
		return false
	}
	return true
}

// --- #7: Pagination helper ---

// paginationParams holds parsed pagination query parameters.
type paginationParams struct {
	Page     int
	PageSize int
	Offset   int
}

// parsePagination reads page/page_size query parameters with sane defaults.
// Accepts both "page_size" and "pageSize" query params for backward compatibility.
func parsePagination(c *gin.Context, defaultPageSize, maxPageSize int) paginationParams {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	pageSizeStr := c.DefaultQuery("page_size", "")
	if pageSizeStr == "" {
		pageSizeStr = c.DefaultQuery("pageSize", strconv.Itoa(defaultPageSize))
	}
	pageSize, _ := strconv.Atoi(pageSizeStr)
	if pageSize < 1 || pageSize > maxPageSize {
		pageSize = defaultPageSize
	}
	return paginationParams{
		Page:     page,
		PageSize: pageSize,
		Offset:   (page - 1) * pageSize,
	}
}

// --- #8: List helper ---

// listAll queries all records ordered as specified and responds with data + total.
func (s *Server) listAll(c *gin.Context, dest interface{}, order string) {
	if err := s.db.Order(order).Find(dest).Error; err != nil {
		slog.Error("ListAll query failed", "error", err)
		respondError(c, http.StatusInternalServerError, "failed to query data")
		return
	}
	respond(c, gin.H{"success": true, "data": dest, "total": reflect.ValueOf(dest).Elem().Len()})
}

// csvSanitize prevents CSV injection by stripping dangerous leading characters.
func csvSanitize(s string) string {
	s = strings.ReplaceAll(s, "\"", "\"\"")
	if len(s) > 0 && (s[0] == '=' || s[0] == '+' || s[0] == '-' || s[0] == '@' || s[0] == '\t' || s[0] == '\r' || s[0] == '\n') {
		s = "'" + s
	}
	return s
}

// serveFileSafe serves a file after validating it falls within the base directory.
// If downloadName is non-empty, it triggers a download (Content-Disposition).
func serveFileSafe(c *gin.Context, absPath, baseDir, downloadName string) {
	if err := validateFilePath(absPath, baseDir); err != nil {
		respondError(c, http.StatusNotFound, "file not found")
		return
	}
	if downloadName != "" {
		c.FileAttachment(absPath, downloadName)
	} else {
		c.File(absPath)
	}
}

// allowedOrigin checks whether an HTTP request's Origin is permitted based on
// the server's AllowedOrigins config. When empty, only localhost origins pass.
func allowedOrigin(cfg *config.Config, r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	originHost := u.Hostname()
	if len(cfg.Server.AllowedOrigins) == 0 {
		return originHost == "localhost" || originHost == "127.0.0.1" || originHost == "::1"
	}
	for _, allowed := range cfg.Server.AllowedOrigins {
		if originHost == allowed {
			return true
		}
	}
	return false
}

// sanitizeFilename strips characters that could enable Content-Disposition header injection
// and prevents path traversal by using filepath.Base and removing leading dots.
func sanitizeFilename(name string) string {
	name = filepath.Base(name)
	name = strings.TrimLeft(name, ".")
	name = sanitizeRe.ReplaceAllString(name, "_")
	if name == "" || name == "." {
		return "download"
	}
	return name
}

// deriveTOTPKey derives a 32-byte AES key from the JWT secret using SHA-256.
func deriveTOTPKey(jwtSecret string) []byte {
	h := sha256.Sum256([]byte("forgec2-totp-encrypt:" + jwtSecret))
	return h[:]
}

// encryptSecret encrypts a plaintext string using AES-256-GCM.
// Output format: base64(nonce(12) + ciphertext).
func encryptSecret(plaintext, jwtSecret string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	key := deriveTOTPKey(jwtSecret)
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// decryptSecret decrypts a base64-encoded AES-256-GCM ciphertext.
func decryptSecret(encoded, jwtSecret string) (string, error) {
	if encoded == "" {
		return "", nil
	}
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	key := deriveTOTPKey(jwtSecret)
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

// validateFilePath ensures a file path is contained within the expected base directory.
func validateFilePath(filePath, baseDir string) error {
	cleaned := filepath.Clean(filePath)
	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return fmt.Errorf("resolve base dir: %w", err)
	}
	absFile, err := filepath.Abs(cleaned)
	if err != nil {
		return fmt.Errorf("resolve file path: %w", err)
	}
	if !strings.HasPrefix(absFile, absBase+string(filepath.Separator)) && absFile != absBase {
		return fmt.Errorf("path traversal detected: %s", filePath)
	}
	return nil
}

// validateWebhookURL checks that a webhook URL does not target private/internal IPs.
func validateWebhookURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("URL has no hostname")
	}

	ip := net.ParseIP(host)
	if ip != nil {
		if ip.IsLoopback() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
			return fmt.Errorf("webhook URL targets blocked IP: %s", host)
		}
		if ip.IsPrivate() {
			return fmt.Errorf("webhook URL targets private IP: %s", host)
		}
	}

	// DNS resolution check: resolve the hostname and verify no resolved IP is private
	if ip == nil {
		ips, err := net.LookupIP(host)
		if err == nil {
			for _, resolved := range ips {
				if resolved.IsLoopback() || resolved.IsUnspecified() || resolved.IsLinkLocalUnicast() || resolved.IsLinkLocalMulticast() {
					return fmt.Errorf("webhook URL hostname %s resolves to blocked IP: %s", host, resolved.String())
				}
				if resolved.IsPrivate() {
					return fmt.Errorf("webhook URL hostname %s resolves to private IP: %s", host, resolved.String())
				}
			}
		}
	}

	// Block known cloud metadata hostnames
	blockedHosts := []string{
		"169.254.169.254", "metadata.google.internal",
		"169.254.169.254.nip.io", "localhost",
	}
	for _, h := range blockedHosts {
		if strings.EqualFold(host, h) {
			return fmt.Errorf("webhook URL targets blocked hostname: %s", host)
		}
	}

	// Only allow http/https schemes
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("webhook URL must use http or https scheme, got %s", u.Scheme)
	}

	return nil
}

// escapeLike escapes SQL LIKE wildcard characters in user input
// so they are treated as literal text rather than pattern characters.
// Usage: .Where("field LIKE ? ESCAPE '\\'", "%"+escapeLike(input)+"%")
func escapeLike(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	s = strings.ReplaceAll(s, `[`, `\[`)
	s = strings.ReplaceAll(s, `]`, `\]`)
	return s
}

// marshalJSONSafe wraps json.Marshal with error logging.
// Returns the JSON bytes and a boolean indicating success.
func marshalJSONSafe(v interface{}) ([]byte, bool) {
	b, err := json.Marshal(v)
	if err != nil {
		slog.Error("json.Marshal failed", "error", err, "type", fmt.Sprintf("%T", v))
		return nil, false
	}
	return b, true
}
