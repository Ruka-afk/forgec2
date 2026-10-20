package server

import (
	"encoding/json"
	"net/http"
	"net/url"
	"testing"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
)

func TestHandleFileSearchQueuesStructuredTask(t *testing.T) {
	gin.SetMode(gin.TestMode)
	database := newContractDB(t)
	srv := newTestFileServer(t, database)
	agent := seedImplant(t, database)

	form := url.Values{}
	form.Set("path", `C:\Users\Public`)
	form.Set("pattern", "*.jpg,*.png")
	form.Set("categories", "image,video")
	form.Set("min_size", "1024")
	form.Set("max_size", "1048576")
	form.Set("modified_after", "2026-01-01")
	form.Set("modified_before", "2026-12-31")
	form.Set("include_hidden", "1")
	form.Set("max_depth", "99")
	form.Set("max_results", "99999")
	form.Set("timeout_seconds", "999")

	w, c := newFormContext(http.MethodPost, "/", &form)
	c.Params = gin.Params{{Key: "id", Value: agent.ID}}
	srv.handleFileSearch(c)
	assertStatus(t, w, http.StatusOK)
	body := assertSuccessJSON(t, w)
	if body["task_id"] == nil {
		t.Fatalf("missing task_id: %s", w.Body.String())
	}

	var task db.Task
	if err := database.Where("agent_id = ? AND type = ?", agent.ID, "file_search").First(&task).Error; err != nil {
		t.Fatalf("load task: %v", err)
	}
	if task.Path != `C:\Users\Public` || task.Command != "*.jpg,*.png" {
		t.Fatalf("task path/command = %q / %q", task.Path, task.Command)
	}
	var opts map[string]any
	if err := json.Unmarshal([]byte(task.Data), &opts); err != nil {
		t.Fatalf("decode opts: %v; data=%s", err, task.Data)
	}
	if got := int(opts["max_depth"].(float64)); got != fileSearchHardMaxDepth {
		t.Errorf("max_depth = %d, want %d", got, fileSearchHardMaxDepth)
	}
	if got := int(opts["max_results"].(float64)); got != fileSearchHardMaxResults {
		t.Errorf("max_results = %d, want %d", got, fileSearchHardMaxResults)
	}
	if got := int(opts["timeout_seconds"].(float64)); got != fileSearchHardTimeout {
		t.Errorf("timeout_seconds = %d, want %d", got, fileSearchHardTimeout)
	}
	if opts["include_hidden"] != true {
		t.Errorf("include_hidden = %v", opts["include_hidden"])
	}
	if opts["modified_after"] != "2026-01-01" || opts["modified_before"] != "2026-12-31" {
		t.Errorf("date filters = %v / %v", opts["modified_after"], opts["modified_before"])
	}
	if got := int64(opts["min_size"].(float64)); got != 1024 {
		t.Errorf("min_size = %d", got)
	}
	cats, _ := opts["categories"].([]any)
	if len(cats) != 2 || cats[0] != "image" || cats[1] != "video" {
		t.Errorf("categories = %#v", opts["categories"])
	}
}

func TestHandleFileSearchDefaultsAndValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	database := newContractDB(t)
	srv := newTestFileServer(t, database)
	agent := seedImplant(t, database)

	t.Run("missing path", func(t *testing.T) {
		w, c := newFormContext(http.MethodPost, "/", nil)
		c.Params = gin.Params{{Key: "id", Value: agent.ID}}
		srv.handleFileSearch(c)
		assertStatus(t, w, http.StatusBadRequest)
	})

	t.Run("unknown agent", func(t *testing.T) {
		form := url.Values{"path": {`C:\Users`}}
		w, c := newFormContext(http.MethodPost, "/", &form)
		c.Params = gin.Params{{Key: "id", Value: "missing"}}
		srv.handleFileSearch(c)
		assertStatus(t, w, http.StatusNotFound)
	})

	t.Run("defaults", func(t *testing.T) {
		form := url.Values{"path": {`C:\Users\Public`}}
		w, c := newFormContext(http.MethodPost, "/", &form)
		c.Params = gin.Params{{Key: "id", Value: agent.ID}}
		srv.handleFileSearch(c)
		assertStatus(t, w, http.StatusOK)

		var task db.Task
		if err := database.Where("agent_id = ? AND type = ?", agent.ID, "file_search").Order("id desc").First(&task).Error; err != nil {
			t.Fatalf("load task: %v", err)
		}
		if task.Command != "*" {
			t.Errorf("default command = %q, want *", task.Command)
		}
		var opts map[string]any
		if err := json.Unmarshal([]byte(task.Data), &opts); err != nil {
			t.Fatalf("decode opts: %v", err)
		}
		if got := int(opts["max_depth"].(float64)); got != fileSearchDefaultMaxDepth {
			t.Errorf("max_depth = %d", got)
		}
		if got := int(opts["max_results"].(float64)); got != fileSearchDefaultMaxResults {
			t.Errorf("max_results = %d", got)
		}
		if _, ok := opts["categories"]; ok {
			t.Errorf("categories should be absent for all: %#v", opts["categories"])
		}
	})

	t.Run("unknown category", func(t *testing.T) {
		form := url.Values{"path": {`C:\Users`}, "categories": {"image,nope"}}
		w, c := newFormContext(http.MethodPost, "/", &form)
		c.Params = gin.Params{{Key: "id", Value: agent.ID}}
		srv.handleFileSearch(c)
		assertStatus(t, w, http.StatusBadRequest)
	})

	t.Run("inverted size", func(t *testing.T) {
		form := url.Values{"path": {`C:\Users`}, "min_size": {"100"}, "max_size": {"1"}}
		w, c := newFormContext(http.MethodPost, "/", &form)
		c.Params = gin.Params{{Key: "id", Value: agent.ID}}
		srv.handleFileSearch(c)
		assertStatus(t, w, http.StatusBadRequest)
	})

	t.Run("bad date", func(t *testing.T) {
		form := url.Values{"path": {`C:\Users`}, "modified_after": {"yesterday"}}
		w, c := newFormContext(http.MethodPost, "/", &form)
		c.Params = gin.Params{{Key: "id", Value: agent.ID}}
		srv.handleFileSearch(c)
		assertStatus(t, w, http.StatusBadRequest)
	})
}
