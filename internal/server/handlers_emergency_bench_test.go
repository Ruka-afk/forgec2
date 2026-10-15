package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/server/middleware"
	"github.com/gin-gonic/gin"
)

// benchKillSwitchArm measures arming the kill switch over an n-implant fleet —
// the largest fan-out path in the emergency handlers: one uninstall task per
// registered implant. The fleet and the operator are seeded once; every
// iteration resets the task table and the in-memory pending counters outside
// the timed region so each measurement is a full fan-out, not a warmed-up
// no-op.
func benchKillSwitchArm(b *testing.B, n int) {
	s, database := batchTestServer(b)

	implants := make([]db.Implant, 0, n)
	for i := 0; i < n; i++ {
		implants = append(implants, db.Implant{
			ID:       fmt.Sprintf("bench-%d", i),
			Hostname: "bench",
			Status:   "online",
			TenantID: 1,
		})
	}
	if err := database.CreateInBatches(implants, 200).Error; err != nil {
		b.Fatalf("seed implants: %v", err)
	}
	hash, err := middleware.HashPassword("bench-pw")
	if err != nil {
		b.Fatalf("hash password: %v", err)
	}
	user := db.User{Username: "bench-op", PasswordHash: hash}
	if err := database.Create(&user).Error; err != nil {
		b.Fatalf("seed user: %v", err)
	}

	const body = `{"action":"arm","password":"bench-pw"}`
	arm := func() *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest(http.MethodPost, "/api/killswitch", strings.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Set("user_id", user.ID)
		s.handleKillSwitch(c)
		return w
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		if err := database.Exec("DELETE FROM tasks").Error; err != nil {
			b.Fatalf("reset tasks: %v", err)
		}
		s.agentPendingTasksMu.Lock()
		s.agentPendingTasks = make(map[string]int)
		s.agentPendingTasksMu.Unlock()
		s.setKillSwitch(false, "", "bench")
		b.StartTimer()

		w := arm()
		if w.Code != http.StatusOK {
			b.Fatalf("kill-switch arm failed: %d %s", w.Code, w.Body.String())
		}
		// Keep the benchmark honest: every iteration must fan out to the
		// whole fleet, not silently skip agents at a per-agent cap.
		var resp struct {
			Dispatched int `json:"tasks_dispatched"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			b.Fatalf("arm response parse: %v (body=%s)", err, w.Body.String())
		}
		if resp.Dispatched != n {
			b.Fatalf("dispatched %d of %d agents", resp.Dispatched, n)
		}
	}
}

func BenchmarkKillSwitchArmFleet(b *testing.B) {
	for _, n := range []int{100, 1000} {
		b.Run(fmt.Sprintf("fleet=%d", n), func(b *testing.B) { benchKillSwitchArm(b, n) })
	}
}
