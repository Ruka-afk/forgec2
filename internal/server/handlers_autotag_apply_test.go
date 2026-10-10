package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/forgec2/forgec2/internal/config"
	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/testutil"
	"github.com/gin-gonic/gin"
)

// TestAutoTagApplyBatchesAssignments proves the apply handler inserts all
// matching assignments in one pass (it used to issue one INSERT per pair) and
// reports the true applied count.
func TestAutoTagApplyBatchesAssignments(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := &Server{db: testutil.SetupTestDB(t), cfg: &config.Config{}}

	tag := db.AgentTag{ID: "tag-win", Name: "windows-tag", Color: "#fff"}
	if err := s.db.Create(&tag).Error; err != nil {
		t.Fatalf("seed tag: %v", err)
	}
	rule := db.AutoTagRule{
		ID:        "rule-win",
		Name:      "all-windows",
		Enabled:   true,
		Condition: `[{"field":"os","op":"contains","value":"Windows"}]`,
		TagID:     tag.ID,
	}
	if err := s.db.Create(&rule).Error; err != nil {
		t.Fatalf("seed rule: %v", err)
	}
	for _, id := range []string{"at-a", "at-b", "at-c"} {
		if err := s.db.Create(&db.Implant{ID: id, Hostname: id, OS: "Windows 11"}).Error; err != nil {
			t.Fatalf("seed agent: %v", err)
		}
	}
	// A Linux agent the rule must not match.
	if err := s.db.Create(&db.Implant{ID: "at-linux", Hostname: "at-linux", OS: "Linux"}).Error; err != nil {
		t.Fatalf("seed linux agent: %v", err)
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/autotag/apply", nil)
	s.handleAutoTagApply(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Applied int  `json:"applied"`
		Capped  bool `json:"capped"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Applied != 3 || resp.Capped {
		t.Fatalf("applied=%d capped=%v, want 3/false", resp.Applied, resp.Capped)
	}

	var count int64
	if err := s.db.Model(&db.AgentTagAssignment{}).Count(&count).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 3 {
		t.Fatalf("assignment rows=%d, want 3", count)
	}

	// Second apply must be a no-op (existing set is honored): the batch
	// path must not duplicate or error.
	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Request = httptest.NewRequest(http.MethodPost, "/api/autotag/apply", nil)
	s.handleAutoTagApply(c2)
	if w2.Code != http.StatusOK {
		t.Fatalf("second apply status=%d", w2.Code)
	}
	if err := s.db.Model(&db.AgentTagAssignment{}).Count(&count).Error; err != nil {
		t.Fatalf("count after: %v", err)
	}
	if count != 3 {
		t.Fatalf("assignment rows after second apply=%d, want 3 (no duplicates)", count)
	}
}
