package server

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleHealth(c *gin.Context) {
	uptime := time.Since(s.startTime)
	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"version": ServerVersion,
		"uptime":  uptime.String(),
	})
}

func (s *Server) handleReadyCheck(c *gin.Context) {
	checks := gin.H{}
	ready := true

	// Check database connectivity
	if sqlDB, err := s.db.DB(); err != nil {
		checks["database"] = "error: " + sanitizeError(err, "Server operation")
		ready = false
	} else if err := sqlDB.Ping(); err != nil {
		checks["database"] = "error: " + sanitizeError(err, "Server operation")
		ready = false
	} else {
		checks["database"] = "connected"
	}

	// Check listener count
	var listenerCount int64
	if err := s.db.Model(&db.Listener{}).Count(&listenerCount).Error; err != nil {
		checks["listeners"] = "error: " + sanitizeError(err, "Server operation")
		ready = false
	} else {
		checks["listeners"] = listenerCount
	}

	status := "ok"
	code := http.StatusOK
	if !ready {
		status = "not ready"
		code = http.StatusServiceUnavailable
	}

	c.JSON(code, gin.H{
		"status": status,
		"checks": checks,
	})
}

func (s *Server) handleBuildLogs(c *gin.Context) {
	p := parsePagination(c, DefaultPageSize, MaxPageSize)
	filterStatus := c.Query("status")
	filterPlatform := c.Query("platform")

	query := s.db.Model(&db.BuildLog{})
	if filterStatus != "" {
		query = query.Where("status = ?", filterStatus)
	}
	if filterPlatform != "" {
		query = query.Where("platform = ?", filterPlatform)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		slog.Error("Failed to count build logs", "err", err)
	}

	var logs []db.BuildLog
	if err := query.Order("created_at desc").Offset(p.Offset).Limit(p.PageSize).Find(&logs).Error; err != nil {
		slog.Error("Failed to query build logs", "err", err)
	}

	var successCount, failedCount int64
	if err := s.db.Model(&db.BuildLog{}).Where("status = ?", "success").Count(&successCount).Error; err != nil {
		slog.Error("Failed to count successful builds", "err", err)
	}
	if err := s.db.Model(&db.BuildLog{}).Where("status = ?", "failed").Count(&failedCount).Error; err != nil {
		slog.Error("Failed to count failed builds", "err", err)
	}

	totalPages := (int(total) + p.PageSize - 1) / p.PageSize
	if totalPages < 1 {
		totalPages = 1
	}
	prevPage := p.Page - 1
	nextPage := p.Page + 1
	stats := s.getNavStats()
	data := gin.H{
		"Title":          "ForgeC2 - Build Logs",
		"ActiveNav":      "builds",
		"Logs":           logs,
		"Page":           p.Page,
		"PrevPage":       prevPage,
		"NextPage":       nextPage,
		"PageSize":       p.PageSize,
		"TotalPages":     totalPages,
		"Total":          int(total),
		"SuccessCount":   successCount,
		"FailedCount":    failedCount,
		"FilterStatus":   filterStatus,
		"FilterPlatform": filterPlatform,
	}
	for k, v := range stats {
		data[k] = v
	}

	s.renderPageOrJSON(c, data)
}

// logBuild creates a build log entry
func (s *Server) logBuild(platform, format, c2URL string, listenerID uint, filename, status, errStr, outputPath string) {
	user := "system"
	if err := s.db.Create(&db.BuildLog{
		Platform:   platform,
		Format:     format,
		C2URL:      c2URL,
		ListenerID: listenerID,
		Filename:   filename,
		User:       user,
		Status:     status,
		Error:      errStr,
		OutputPath: outputPath,
	}).Error; err != nil {
		slog.Error("Failed to create build log", "error", err)
	}
}
