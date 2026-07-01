package server

import (
	"fmt"
	"net/http"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
)

const ServerVersion = "2.1.0"

func (s *Server) handleHealth(c *gin.Context) {
	uptime := time.Since(s.startTime)
	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"version": ServerVersion,
		"uptime":  uptime.String(),
	})
}

func (s *Server) handleBuildLogs(c *gin.Context) {
	pageStr := c.DefaultQuery("page", "1")
	pageSizeStr := c.DefaultQuery("pageSize", "20")
	filterStatus := c.Query("status")
	filterPlatform := c.Query("platform")

	pageNum, _ := parseInt(pageStr)
	if pageNum < 1 {
		pageNum = 1
	}
	pageSize, _ := parseInt(pageSizeStr)
	if pageSize < 1 {
		pageSize = DefaultPageSize
	}
	if pageSize > MaxPageSize {
		pageSize = MaxPageSize
	}

	query := s.db.Model(&db.BuildLog{})
	if filterStatus != "" {
		query = query.Where("status = ?", filterStatus)
	}
	if filterPlatform != "" {
		query = query.Where("platform = ?", filterPlatform)
	}

	var total int64
	query.Count(&total)

	var logs []db.BuildLog
	query.Order("created_at desc").Offset((pageNum - 1) * pageSize).Limit(pageSize).Find(&logs)

	var successCount, failedCount int64
	s.db.Model(&db.BuildLog{}).Where("status = ?", "success").Count(&successCount)
	s.db.Model(&db.BuildLog{}).Where("status = ?", "failed").Count(&failedCount)

	totalPages := (int(total) + pageSize - 1) / pageSize
	if totalPages < 1 {
		totalPages = 1
	}
	prevPage := pageNum - 1
	nextPage := pageNum + 1
	stats := s.getNavStats()
	data := gin.H{
		"Title":          "ForgeC2 - Build Logs",
		"ActiveNav":      "builds",
		"Logs":           logs,
		"Page":           pageNum,
		"PrevPage":       prevPage,
		"NextPage":       nextPage,
		"PageSize":       pageSize,
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

	s.renderPage(c, "builds_content", data)
}

// logBuild creates a build log entry
func (s *Server) logBuild(platform, format, c2URL string, listenerID uint, filename, status, errStr, outputPath string) {
	user := "system"
	s.db.Create(&db.BuildLog{
		Platform:   platform,
		Format:     format,
		C2URL:      c2URL,
		ListenerID: listenerID,
		Filename:   filename,
		User:       user,
		Status:     status,
		Error:      errStr,
		OutputPath: outputPath,
	})
}

// parseInt is a simple helper to convert string to int, returns 0 on error
func parseInt(s string) (int, error) {
	var r int
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("not a number: %s", s)
		}
		r = r*10 + int(c-'0')
	}
	return r, nil
}
