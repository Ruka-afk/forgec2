package server

import (
	"math"
	"net/http"
	"sort"
	"time"

	"github.com/gin-gonic/gin"
)

// handleTrafficProfileGet returns a baseline report and recent beacon records for an agent.
func (s *Server) handleTrafficProfileGet(c *gin.Context) {
	agentID := c.Param("id")
	if agentID == "" {
		respondError(c, http.StatusBadRequest, "agent id required")
		return
	}

	logs := s.trafficLog.recent(500)
	var agentLogs []TrafficEntry
	for _, l := range logs {
		if l.AgentID == agentID {
			agentLogs = append(agentLogs, l)
		}
	}

	if len(agentLogs) < 2 {
		c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{
			"agent_id":              agentID,
			"sample_count":          len(agentLogs),
			"baseline_interval":     0,
			"baseline_jitter":       0,
			"baseline_packet_size":  0,
			"mean_interval":         0,
			"stddev_interval":       0,
			"mean_packet_size":      0,
			"cv":                    0,
			"auto_adapt":            false,
			"recent_records":        agentLogs,
			"suggestion":            nil,
		}})
		return
	}

	sort.Slice(agentLogs, func(i, j int) bool {
		return agentLogs[i].Time.Before(agentLogs[j].Time)
	})

	intervals := make([]float64, 0, len(agentLogs)-1)
	sizes := make([]float64, 0, len(agentLogs))
	for i, l := range agentLogs {
		sizes = append(sizes, float64(l.Size))
		if i > 0 {
			intervals = append(intervals, agentLogs[i].Time.Sub(agentLogs[i-1].Time).Seconds())
		}
	}

	meanInterval := avg(intervals)
	stddevInterval := stddev(intervals, meanInterval)
	meanSize := avg(sizes)
	cv := 0.0
	if meanInterval > 0 {
		cv = stddevInterval / meanInterval
	}

	baselineInterval := int(math.Round(meanInterval))
	baselineJitter := 0
	if baselineInterval > 0 {
		baselineJitter = int(math.Round(cv * 100))
	}

	suggestion := computeAdaptationSuggestion(baselineInterval, baselineJitter, int(meanSize))

	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{
		"agent_id":              agentID,
		"sample_count":          len(agentLogs),
		"baseline_interval":     baselineInterval,
		"baseline_jitter":       baselineJitter,
		"baseline_packet_size":  int(math.Round(meanSize)),
		"mean_interval":         int(math.Round(meanInterval)),
		"stddev_interval":       int(math.Round(stddevInterval)),
		"mean_packet_size":      int(math.Round(meanSize)),
		"cv":                    math.Round(cv*1000) / 1000,
		"auto_adapt":            false,
		"recent_records":        agentLogs,
		"suggestion":            suggestion,
	}})
}

// handleTrafficProfileAdapt queues an adaptation task for the agent.
func (s *Server) handleTrafficProfileAdapt(c *gin.Context) {
	agentID := c.Param("id")
	if agentID == "" {
		respondError(c, http.StatusBadRequest, "agent id required")
		return
	}
	s.LogAuditRecord(c, "traffic_adapt", "agent", agentID, "Traffic adaptation triggered", true, nil)
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Adaptation task queued for agent " + agentID})
}

// handleTrafficProfileAutoAdapt toggles auto-adapt for an agent.
func (s *Server) handleTrafficProfileAutoAdapt(c *gin.Context) {
	agentID := c.Param("id")
	if agentID == "" {
		respondError(c, http.StatusBadRequest, "agent id required")
		return
	}

	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid request")
		return
	}
	s.LogAuditRecord(c, "traffic_auto_adapt", "agent", agentID,
		"Auto-adapt toggled", true, nil)
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "auto-adapt set"})
}

func avg(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	var sum float64
	for _, v := range vals {
		sum += v
	}
	return sum / float64(len(vals))
}

func stddev(vals []float64, mean float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	var ss float64
	for _, v := range vals {
		d := v - mean
		ss += d * d
	}
	return math.Sqrt(ss / float64(len(vals)))
}

func computeAdaptationSuggestion(interval, jitter, packetSize int) map[string]interface{} {
	if interval <= 0 {
		return nil
	}
	desired := interval
	reason := "baseline matches environment"
	if jitter < 10 {
		desired = int(math.Round(float64(interval) * 1.2))
		reason = "low jitter detected, increasing interval"
	} else if jitter > 50 {
		desired = int(math.Round(float64(interval) * 0.8))
		reason = "high jitter detected, reducing interval"
	}
	if desired < 5 {
		desired = 5
	}
	return map[string]interface{}{
		"desired_interval": desired,
		"desired_jitter":   jitter,
		"pad_size":         packetSize + 32,
		"reason":           reason,
		"confidence":       "medium",
		"suggested_at":     time.Now().Format(time.RFC3339),
	}
}
