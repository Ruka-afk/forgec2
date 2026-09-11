package server

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/testutil"
	"github.com/gin-gonic/gin"
)

func TestHandleWallpaper(t *testing.T) {
	database := testutil.SetupTestDB(t)
	s := newTestFileServer(t, database)
	agent := seedImplant(t, database)

	t.Run("missing image rejected", func(t *testing.T) {
		w, c := newFormContext(http.MethodPost, "/agents/"+agent.ID+"/wallpaper", &url.Values{})
		c.Params = gin.Params{{Key: "id", Value: agent.ID}}
		s.handleWallpaper(c)
		assertStatus(t, w, http.StatusBadRequest)
	})

	t.Run("valid request queues task", func(t *testing.T) {
		form := &url.Values{"image": {"https://example.com/bg.jpg"}, "style": {"fit"}}
		w, c := newFormContext(http.MethodPost, "/agents/"+agent.ID+"/wallpaper", form)
		c.Params = gin.Params{{Key: "id", Value: agent.ID}}
		s.handleWallpaper(c)
		m := assertSuccessJSON(t, w)
		id, ok := m["task_id"].(float64)
		if !ok || id == 0 {
			t.Fatalf("expected task_id, got %s", w.Body.String())
		}
		var task db.Task
		if err := database.First(&task, uint(id)).Error; err != nil {
			t.Fatalf("task row missing: %v", err)
		}
		if task.Type != "wallpaper" || task.Command != "https://example.com/bg.jpg" || task.Data != "fit" {
			t.Errorf("bad task row: %+v", task)
		}
	})
}
