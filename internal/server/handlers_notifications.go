package server

import (
	"log/slog"
	"net/http"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
)

// handleListNotifications returns paginated notifications with optional filters.
// GET /notifications?page=1&page_size=50&type=&severity=&read=
func (s *Server) handleListNotifications(c *gin.Context) {
	pg := parsePagination(c, 50, 200)
	typ := c.Query("type")
	severity := c.Query("severity")
	read := c.Query("read")

	q := s.db.Model(&db.Notification{})
	if typ != "" {
		q = q.Where("type = ?", typ)
	}
	if severity != "" {
		q = q.Where("severity = ?", severity)
	}
	if read == "true" {
		q = q.Where("read = ?", true)
	} else if read == "false" {
		q = q.Where("read = ?", false)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		slog.Error("Failed to count notifications", "err", err)
	}

	var items []db.Notification
	if err := q.Order("created_at desc").Offset(pg.Offset).Limit(pg.PageSize).Find(&items).Error; err != nil {
		slog.Error("Failed to query notifications", "err", err)
	}

	respond(c, gin.H{"notifications": items, "total": total})
}

// handleMarkNotificationRead marks a single notification as read.
// PUT /notifications/:id/read
func (s *Server) handleMarkNotificationRead(c *gin.Context) {
	id := c.Param("id")
	res := s.db.Model(&db.Notification{}).Where("id = ?", id).Update("read", true)
	if res.Error != nil {
		respondError(c, http.StatusInternalServerError, sanitizeError(res.Error, "Notification operation"))
		return
	}
	if res.RowsAffected == 0 {
		respondError(c, http.StatusNotFound, "notification not found")
		return
	}
	respond(c, gin.H{"success": true})
}

// handleMarkAllNotificationsRead marks all unread notifications as read.
// PUT /notifications/read-all
func (s *Server) handleMarkAllNotificationsRead(c *gin.Context) {
	if err := s.db.Model(&db.Notification{}).Where("read = ?", false).Update("read", true).Error; err != nil {
		respondError(c, http.StatusInternalServerError, sanitizeError(err, "Notification operation"))
		return
	}
	respond(c, gin.H{"success": true})
}

// handleDeleteNotification deletes a single notification.
// DELETE /notifications/:id
func (s *Server) handleDeleteNotification(c *gin.Context) {
	id := c.Param("id")
	res := s.db.Delete(&db.Notification{}, "id = ?", id)
	if res.Error != nil {
		respondError(c, http.StatusInternalServerError, sanitizeError(res.Error, "Notification operation"))
		return
	}
	if res.RowsAffected == 0 {
		respondError(c, http.StatusNotFound, "notification not found")
		return
	}
	respond(c, gin.H{"success": true})
}

// handleClearAllNotifications deletes all notifications (admin only).
// DELETE /notifications
func (s *Server) handleClearAllNotifications(c *gin.Context) {
	if !s.requireAdmin(c) {
		return
	}
	if err := s.db.Where("1 = 1").Delete(&db.Notification{}).Error; err != nil {
		respondError(c, http.StatusInternalServerError, sanitizeError(err, "Notification operation"))
		return
	}
	respond(c, gin.H{"success": true})
}
