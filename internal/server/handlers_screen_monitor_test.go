package server

import (
	"encoding/json"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
)

func TestScreenMonitorLifecycleIsValidatedAndIdempotent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	database := newContractDB(t)
	srv := newTestFileServer(t, database)
	agent := seedImplant(t, database)

	form := url.Values{"interval": {"3"}, "quality": {"high"}}
	w, c := newFormContext(http.MethodPost, "/", &form)
	c.Params = gin.Params{{Key: "id", Value: agent.ID}}
	srv.handleStartScreenMonitor(c)
	assertStatus(t, w, http.StatusOK)
	response := assertSuccessJSON(t, w)
	if response["active"] != true || response["task_id"] == nil {
		t.Fatalf("unexpected start response: %s", w.Body.String())
	}

	var startTasks []db.Task
	if err := database.Where("agent_id = ? AND type = ?", agent.ID, "screen_stream_start").Find(&startTasks).Error; err != nil {
		t.Fatalf("load start task: %v", err)
	}
	if len(startTasks) != 1 || startTasks[0].Command != "3,high" {
		t.Fatalf("expected one 3,high task, got %#v", startTasks)
	}

	// Starting the same agent again must not consume another monitor slot or
	// queue a duplicate stream goroutine on the agent.
	w, c = newFormContext(http.MethodPost, "/", &form)
	c.Params = gin.Params{{Key: "id", Value: agent.ID}}
	srv.handleStartScreenMonitor(c)
	assertStatus(t, w, http.StatusOK)
	response = assertSuccessJSON(t, w)
	if response["already_active"] != true {
		t.Fatalf("expected already_active response: %s", w.Body.String())
	}
	var startCount int64
	database.Model(&db.Task{}).Where("agent_id = ? AND type = ?", agent.ID, "screen_stream_start").Count(&startCount)
	if startCount != 1 {
		t.Fatalf("duplicate start queued %d tasks", startCount)
	}

	w, c = newFormContext(http.MethodPost, "/", nil)
	c.Params = gin.Params{{Key: "id", Value: agent.ID}}
	srv.handleStopScreenMonitor(c)
	assertStatus(t, w, http.StatusOK)
	if srv.IsScreenMonitoring(agent.ID) {
		t.Fatal("agent remained registered after stop")
	}
	var preservedStartCount int64
	database.Model(&db.Task{}).Where("agent_id = ? AND type = ?", agent.ID, "screen_stream_start").Count(&preservedStartCount)
	if preservedStartCount != 1 {
		t.Fatalf("stop deleted screen history; start count=%d", preservedStartCount)
	}

	// Repeated stop is a successful no-op and must not queue more stop tasks.
	w, c = newFormContext(http.MethodPost, "/", nil)
	c.Params = gin.Params{{Key: "id", Value: agent.ID}}
	srv.handleStopScreenMonitor(c)
	assertStatus(t, w, http.StatusOK)
	var stopCount int64
	database.Model(&db.Task{}).Where("agent_id = ? AND type = ?", agent.ID, "screen_stream_stop").Count(&stopCount)
	if stopCount != 1 {
		t.Fatalf("idempotent stop queued %d tasks", stopCount)
	}
}

func TestScreenMonitorRejectsInvalidIntervalWithoutRegistering(t *testing.T) {
	gin.SetMode(gin.TestMode)
	database := newContractDB(t)
	srv := newTestFileServer(t, database)
	agent := seedImplant(t, database)

	form := url.Values{"interval": {"3000"}, "quality": {"medium"}}
	w, c := newFormContext(http.MethodPost, "/", &form)
	c.Params = gin.Params{{Key: "id", Value: agent.ID}}
	srv.handleStartScreenMonitor(c)
	assertStatus(t, w, http.StatusBadRequest)
	if srv.IsScreenMonitoring(agent.ID) {
		t.Fatal("invalid request registered a monitor")
	}
}

func TestLatestScreenFrameProvidesHTTPFallback(t *testing.T) {
	gin.SetMode(gin.TestMode)
	database := newContractDB(t)
	srv := newTestFileServer(t, database)
	agent := seedImplant(t, database)
	srv.screenMonitorImplants[agent.ID] = time.Now()
	srv.BroadcastScreenshot(agent.ID, "/9j/test-frame")

	w, c := newFormContext(http.MethodGet, "/", nil)
	c.Params = gin.Params{{Key: "id", Value: agent.ID}}
	srv.handleGetAgentScreenshot(c)
	assertStatus(t, w, http.StatusOK)

	var response struct {
		Data struct {
			Image     string `json:"image"`
			CaptureID string `json:"capture_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Data.Image != "data:image/jpeg;base64,/9j/test-frame" || response.Data.CaptureID == "" {
		t.Fatalf("unexpected fallback response: %s", w.Body.String())
	}
}
