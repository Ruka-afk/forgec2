package server

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/testutil"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// The beacon check-in is the hot path of the whole server: every implant walks
// it every few seconds, so its cost per request is what decides how many
// implants one core carries. These benchmarks measure that round trip through
// the real handler — frame decode, per-agent key derivation, replay check,
// task claim and response encryption — over an in-memory SQLite database.
//
// Measured: the HTTP handler exactly as the router calls it, including
// request/response crypto. Not measured: socket and kernel time (httptest
// recorder, no real connection), the operator UI's polling load, and SQLite's
// fsync cost (":memory:" instead of a WAL file). docs/CAPACITY.md records the
// numbers and the reproduction commands.

// beaconBenchServer registers one agent through the real registration path and
// returns the server, its router, the agent and the database.
//
// The dedup cache is left nil: every benchmark iteration posts a frame whose
// fingerprint matches the previous one, and dedup would turn every iteration
// after the first into a replay rejection instead of the measured path. The
// replay check itself is a mutex plus one map lookup, so omitting it does not
// flatter the numbers; re-enabling it is a one-line change if that ever needs
// measuring.
func beaconBenchServer(b *testing.B) (*Server, *gin.Engine, *tcpTestAgent, *gorm.DB) {
	b.Helper()
	gin.SetMode(gin.TestMode)
	database := testutil.SetupTestDB(b)
	s, r := initBeaconTestServer(b, database)
	agent := v3TestAgent(b, s, "11111111-2222-4333-8444-555555555555")

	w := postJSON(r, "/beacon", agent.registerFrame())
	if w.Code != http.StatusOK {
		b.Fatalf("register: status=%d body=%s", w.Code, w.Body.String())
	}
	var regResp struct {
		ECDHPub string `json:"ecdh_pub"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &regResp); err != nil {
		b.Fatalf("register response: %v", err)
	}
	if err := agent.establishFromServerKey(regResp.ECDHPub); err != nil {
		b.Fatalf("establish session: %v", err)
	}
	s.beaconDedupMu.Lock()
	s.beaconDedupCache = nil
	s.beaconDedupMu.Unlock()
	return s, r, agent, database
}

// benchBeaconFrame posts one encrypted check-in and fails the benchmark on a
// non-200, so a change that silently turns the hot path into an error reply
// cannot look like a speedup.
func benchBeaconFrame(b *testing.B, r *gin.Engine, agent *tcpTestAgent, inner []byte) {
	b.Helper()
	w := postJSON(r, "/beacon", agent.encryptedFrame(inner))
	if w.Code != http.StatusOK {
		b.Fatalf("beacon: status=%d body=%s", w.Code, w.Body.String())
	}
}

// BenchmarkBeaconCheckin measures the steady-state check-in: an idle implant
// with no tasks waiting and nothing to report.
func BenchmarkBeaconCheckin(b *testing.B) {
	_, r, agent, _ := beaconBenchServer(b)
	inner, err := json.Marshal(map[string]interface{}{"uuid": agent.uuid, "pv": 2})
	if err != nil {
		b.Fatalf("marshal: %v", err)
	}
	benchBeaconFrame(b, r, agent, inner) // warm the session/routing paths

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchBeaconFrame(b, r, agent, inner)
	}
}

// BenchmarkBeaconTaskDelivery measures the check-in that hands the implant its
// next command: pending-task selection, claim and a response frame carrying
// the task.
func BenchmarkBeaconTaskDelivery(b *testing.B) {
	_, r, agent, database := beaconBenchServer(b)
	inner, err := json.Marshal(map[string]interface{}{"uuid": agent.uuid, "pv": 2})
	if err != nil {
		b.Fatalf("marshal: %v", err)
	}
	var previous *db.Task

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		// One claimable task per iteration; the previous one is removed so the
		// table does not grow under the benchmark.
		task := &db.Task{AgentID: agent.uuid, Type: "shell", Command: "whoami", Status: "pending", Priority: 1}
		if err := database.Create(task).Error; err != nil {
			b.Fatalf("create task: %v", err)
		}
		if previous != nil {
			if err := database.Delete(previous).Error; err != nil {
				b.Fatalf("delete task: %v", err)
			}
		}
		previous = task
		b.StartTimer()

		benchBeaconFrame(b, r, agent, inner)
	}
}

// BenchmarkBeaconCheckinWithResult measures the check-in an implant performs
// after finishing a command: the same round trip plus one shell result
// (status transition, output storage and the audit/event rows it drives).
func BenchmarkBeaconCheckinWithResult(b *testing.B) {
	_, r, agent, database := beaconBenchServer(b)
	var previous *db.Task

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		// A task delivered but not yet reported, like step 4 of the E2E flow:
		// the implant answers with the result and acknowledges the task.
		task := &db.Task{AgentID: agent.uuid, Type: "shell", Command: "whoami", Status: "sent", Priority: 1}
		if err := database.Create(task).Error; err != nil {
			b.Fatalf("create task: %v", err)
		}
		if previous != nil {
			if err := database.Delete(previous).Error; err != nil {
				b.Fatalf("delete task: %v", err)
			}
		}
		previous = task
		resultsJSON, err := json.Marshal([]taskResult{{
			TaskID: task.ID,
			Type:   "shell",
			Output: "nt authority\\system\r\n",
		}})
		if err != nil {
			b.Fatalf("marshal results: %v", err)
		}
		inner, err := json.Marshal(map[string]interface{}{
			"uuid":    agent.uuid,
			"pv":      2,
			"results": json.RawMessage(resultsJSON),
			"acks":    []uint{task.ID},
		})
		if err != nil {
			b.Fatalf("marshal frame: %v", err)
		}
		b.StartTimer()

		benchBeaconFrame(b, r, agent, inner)
	}
}
