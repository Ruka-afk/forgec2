package server

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleTaskHistory(c *gin.Context) {
	p := parsePagination(c, DefaultTaskPageSize, MaxTaskPageSize)
	filterType := c.Query("type")
	filterStatus := c.Query("status")
	filterAgent := c.Query("agent")
	filterQuery := c.Query("q")

	// Build query with filters
	silentTypes := []string{"screen_stream_start", "screen_stream_stop", "ls"}
	query := s.db.Model(&db.Task{}).
		Where("type NOT IN ?", silentTypes)
	// Multi-tenant isolation: operators only see tasks in their tenant.
	query = s.tenantScope(query, c)
	if filterType != "" {
		query = query.Where("type = ?", filterType)
	}
	if filterStatus != "" {
		query = query.Where("status = ?", filterStatus)
	}
	if filterAgent != "" {
		query = query.Where("agent_id = ?", filterAgent)
	}
	if filterQuery != "" {
		query = query.Where("command LIKE ? ESCAPE '\\'", "%"+escapeLike(filterQuery)+"%")
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "failed to count tasks")
		return
	}

	var tasks []db.Task
	if err := query.Preload("Agent").
		Order("created_at desc").Offset(p.Offset).Limit(p.PageSize).Find(&tasks).Error; err != nil {
		handleQueryError(c, err, "Failed to query tasks")
		return
	}

	// Collect distinct task types for filter dropdown (degraded gracefully on error)
	var taskTypes []string
	if err := s.db.Model(&db.Task{}).
		Where("type NOT IN ?", silentTypes).
		Distinct("type").Pluck("type", &taskTypes).Error; err != nil {
		slog.Warn("Failed to pluck task types", "err", err)
		taskTypes = []string{}
	}

	// Collect agents for filter dropdown (degraded gracefully on error)
	var agents []db.Implant
	if err := s.db.Select("id, hostname, ip").Order("hostname").Limit(500).Find(&agents).Error; err != nil {
		slog.Warn("Failed to query agent filter list", "err", err)
	}

	// Count failed tasks for "retry all" button (degraded gracefully on error)
	var failedCount int64
	if err := s.db.Model(&db.Task{}).Where("status = ?", "failed").Count(&failedCount).Error; err != nil {
		slog.Warn("Failed to count failed tasks", "err", err)
	}

	totalPages := int(total) / p.PageSize
	if int(total)%p.PageSize > 0 {
		totalPages++
	}

	stats := s.getNavStats(c)
	data := gin.H{
		"Title":          "ForgeC2 - Task History",
		"ActiveNav":      "tasks",
		"Tasks":          tasks,
		"Page":           p.Page,
		"PageSize":       p.PageSize,
		"Total":          total,
		"TotalPages":     totalPages,
		"FilterType":     filterType,
		"FilterStatus":   filterStatus,
		"FilterAgent":    filterAgent,
		"FilterQuery":    filterQuery,
		"HasFailedTasks": failedCount > 0,
		"TaskTypes":      taskTypes,
		"Agents":         agents,
	}
	for k, v := range stats {
		data[k] = v
	}

	s.renderPageOrJSON(c, data)
}

// handleExportTasks exports tasks as CSV for reporting
func (s *Server) handleExportTasks(c *gin.Context) {
	var tasks []db.Task
	if err := s.db.Preload("Agent").
		Where("type NOT IN ?", []string{"screen_stream_start", "screen_stream_stop", "ls"}).
		Order("created_at desc").Limit(ExportTaskLimit).Find(&tasks).Error; err != nil {
		handleQueryError(c, err, "Failed to export tasks")
		return
	}

	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)
	writer.Write([]string{"Time", "Agent", "Type", "Command", "Result", "Error", "Status"})

	for _, t := range tasks {
		agentName := ""
		if t.Agent.Hostname != "" {
			agentName = t.Agent.Hostname
		}
		writer.Write([]string{
			t.CreatedAt.Format("2006-01-02 15:04:05"),
			agentName,
			t.Type,
			csvSafe(t.Command),
			csvSafe(truncateString(t.Result, CSVResultTruncLen)),
			csvSafe(truncateString(t.Error, CSVErrorTruncLen)),
			t.Status,
		})
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		slog.Error("Failed to write CSV export", "error", err)
		respondError(c, http.StatusInternalServerError, "failed to export tasks")
		return
	}

	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", `attachment; filename="forgec2_tasks_`+time.Now().Format("2006-01-02")+`.csv"`)
	c.String(http.StatusOK, buf.String())
}

func (s *Server) apiBulkTaskStatus(c *gin.Context) {
	var req struct {
		TaskIDs []uint `json:"task_ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "task_ids required")
		return
	}
	if len(req.TaskIDs) > MaxTaskIDsPerRequest {
		respondError(c, http.StatusBadRequest, fmt.Sprintf("max %d task IDs per request", MaxTaskIDsPerRequest))
		return
	}
	var tasks []db.Task
	if err := s.db.Where("id IN ?", req.TaskIDs).Select("id, status, result, error, updated_at").Find(&tasks).Error; err != nil {
		handleQueryError(c, err, "Failed to bulk query task status")
		return
	}
	result := make(map[uint]db.Task, len(tasks))
	for i := range tasks {
		result[tasks[i].ID] = tasks[i]
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": result})
}
