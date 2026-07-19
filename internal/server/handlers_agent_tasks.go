package server

import (
	"encoding/csv"
	"strconv"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleTaskHistory(c *gin.Context) {
	pageStr := c.DefaultQuery("page", "1")
	pageSizeStr := c.DefaultQuery("pageSize", "50")
	filterType := c.Query("type")
	filterStatus := c.Query("status")
	filterAgent := c.Query("agent")
	filterQuery := c.Query("q")

	pageNum, _ := strconv.Atoi(pageStr)
	if pageNum < 1 {
		pageNum = 1
	}
	pageSize, _ := strconv.Atoi(pageSizeStr)
	if pageSize < 1 {
		pageSize = DefaultTaskPageSize
	}
	if pageSize > MaxTaskPageSize {
		pageSize = MaxTaskPageSize
	}

	// Build query with filters
	silentTypes := []string{"screen_stream_start", "screen_stream_stop", "ls"}
	query := s.db.Model(&db.Task{}).
		Where("type NOT IN ?", silentTypes)
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
	query.Count(&total)

	var tasks []db.Task
	query.Preload("Agent").
		Order("created_at desc").Offset((pageNum - 1) * pageSize).Limit(pageSize).Find(&tasks)

	// Collect distinct task types for filter dropdown
	var taskTypes []string
	s.db.Model(&db.Task{}).
		Where("type NOT IN ?", silentTypes).
		Distinct("type").Pluck("type", &taskTypes)

	// Collect agents for filter dropdown
	var agents []db.Implant
	s.db.Select("id, hostname, ip").Order("hostname").Find(&agents)

	// Count failed tasks for "retry all" button
	var failedCount int64
	s.db.Model(&db.Task{}).Where("status = ?", "failed").Count(&failedCount)

	totalPages := int(total) / pageSize
	if int(total)%pageSize > 0 {
		totalPages++
	}

	stats := s.getNavStats()
	data := gin.H{
		"Title":          "ForgeC2 - Task History",
		"ActiveNav":      "tasks",
		"Tasks":          tasks,
		"Page":           pageNum,
		"PageSize":       pageSize,
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
	s.db.Preload("Agent").
		Where("type NOT IN ?", []string{"screen_stream_start", "screen_stream_stop", "ls"}).
		Order("created_at desc").Limit(ExportTaskLimit).Find(&tasks) // cap to avoid huge exports

	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", `attachment; filename="forgec2_tasks_`+time.Now().Format("2006-01-02")+`.csv"`)

	writer := csv.NewWriter(c.Writer)
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
			t.Command,
			truncateString(t.Result, CSVResultTruncLen),
			truncateString(t.Error, CSVErrorTruncLen),
			t.Status,
		})
	}
	writer.Flush()
}
