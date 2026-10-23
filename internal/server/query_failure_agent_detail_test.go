package server

import (
	"testing"

	"github.com/gin-gonic/gin"
)

// TestGetTokens_QueryFailureIsNotAnEmptyList proves a failed token read
// answers 500. The empty-200 form made the agent-detail UI report "no
// tokens" on a transient DB failure — a false bill of health for a
// privilege-escalation surface.
func TestGetTokens_QueryFailureIsNotAnEmptyList(t *testing.T) {
	assertQueryFailureIs500(t, "agent tokens", "/api/agents/a1/tokens", func(s *Server, c *gin.Context) {
		c.Params = gin.Params{{Key: "id", Value: "a1"}}
		s.handleGetTokens(c)
	})
}

// TestCloudResults_QueryFailureIsNotAnEmptyList proves a failed cloud-cred
// read answers 500: an empty 200 would claim the agent harvested nothing.
func TestCloudResults_QueryFailureIsNotAnEmptyList(t *testing.T) {
	assertQueryFailureIs500(t, "cloud results", "/api/cloud/results/a1", func(s *Server, c *gin.Context) {
		c.Params = gin.Params{{Key: "agentId", Value: "a1"}}
		s.handleCloudResults(c)
	})
}

// TestBloodHoundList_QueryFailureIsNotAnEmptyList proves a failed
// BloodHound list read answers 500 instead of "nothing collected yet".
func TestBloodHoundList_QueryFailureIsNotAnEmptyList(t *testing.T) {
	assertQueryFailureIs500(t, "bloodhound list", "/api/bloodhound/list", func(s *Server, c *gin.Context) {
		s.handleBloodHoundList(c)
	})
}

// TestBloodHoundStatus_QueryFailureIsNotAnEmptyList proves the summary
// endpoint refuses to report a zero count when the count query failed.
func TestBloodHoundStatus_QueryFailureIsNotAnEmptyList(t *testing.T) {
	assertQueryFailureIs500(t, "bloodhound status", "/api/bloodhound/status", func(s *Server, c *gin.Context) {
		s.handleBloodHoundStatus(c)
	})
}

// TestPhishingLists_QueryFailureIsNotAnEmptyList covers all three phishing
// read endpoints: an empty 200 would read as "no templates", "no campaigns"
// and "no captures" respectively.
func TestPhishingLists_QueryFailureIsNotAnEmptyList(t *testing.T) {
	assertQueryFailureIs500(t, "phishing templates", "/api/phishing/templates", func(s *Server, c *gin.Context) {
		s.handleAPIPhishingTemplates(c)
	})
	assertQueryFailureIs500(t, "phishing campaigns", "/api/phishing/campaigns", func(s *Server, c *gin.Context) {
		s.handleAPIPhishingCampaigns(c)
	})
	assertQueryFailureIs500(t, "phishing captures", "/api/phishing/captures", func(s *Server, c *gin.Context) {
		s.handleAPIPhishingCaptures(c)
	})
}

// TestListNotifications_QueryFailureIsNotAnEmptyList proves a failed
// notifications read answers 500: an empty 200 with total 0 reads as "no
// notifications", which is the one state an operator must never be told
// falsely (missed alerts).
func TestListNotifications_QueryFailureIsNotAnEmptyList(t *testing.T) {
	assertQueryFailureIs500(t, "notifications", "/notifications", func(s *Server, c *gin.Context) {
		s.handleListNotifications(c)
	})
}

// TestDashboardTaskStatus_QueryFailureIsNotAnEmptyList proves the task
// status aggregate refuses to render zeros when its query failed.
func TestDashboardTaskStatus_QueryFailureIsNotAnEmptyList(t *testing.T) {
	assertQueryFailureIs500(t, "dashboard task status", "/api/dashboard/task-status", func(s *Server, c *gin.Context) {
		c.Set(tenantIDContextKey, tenantResolution{tid: 1, ok: true})
		s.handleDashboardTaskStatus(c)
	})
}

// TestDashboardAttackPath_QueryFailureIsNotAnEmptyList proves the attack
// path graph refuses to draw "no agents" when its query failed.
func TestDashboardAttackPath_QueryFailureIsNotAnEmptyList(t *testing.T) {
	assertQueryFailureIs500(t, "dashboard attack path", "/api/dashboard/attack-path", func(s *Server, c *gin.Context) {
		c.Set(tenantIDContextKey, tenantResolution{tid: 1, ok: true})
		s.handleDashboardAttackPath(c)
	})
}

// TestCollabAgents_QueryFailureIsNotAnEmptyList proves a failed collab
// agent or lock read answers 500: an empty 200 reads as "no agents" and
// silently drops active operator locks.
func TestCollabAgents_QueryFailureIsNotAnEmptyList(t *testing.T) {
	assertQueryFailureIs500(t, "collab agents", "/api/collab/agents", func(s *Server, c *gin.Context) {
		c.Set(tenantIDContextKey, tenantResolution{tid: 1, ok: true})
		s.handleCollabAgents(c)
	})
}

// TestListWebhooks_QueryFailureIsNotAnEmptyList proves a failed webhook
// read answers 500: an empty 200 reads as "alerting is disabled".
func TestListWebhooks_QueryFailureIsNotAnEmptyList(t *testing.T) {
	assertQueryFailureIs500(t, "webhooks", "/api/webhooks", func(s *Server, c *gin.Context) {
		c.Set(tenantIDContextKey, tenantResolution{tid: 1, ok: true})
		s.handleListWebhooks(c)
	})
}

// TestCircuitBreakerEvents_QueryFailureIsNotAnEmptyList proves a failed
// breaker-event read answers 500: an empty 200 reads as "never tripped".
func TestCircuitBreakerEvents_QueryFailureIsNotAnEmptyList(t *testing.T) {
	assertQueryFailureIs500(t, "circuit breaker events", "/api/circuit-breaker/events", func(s *Server, c *gin.Context) {
		s.handleAPICircuitBreakerEvents(c)
	})
}

// TestListTemplatesJSON_QueryFailureIsNotAnEmptyList proves a failed
// template read answers 500: an empty 200 reads as "no templates exist".
func TestListTemplatesJSON_QueryFailureIsNotAnEmptyList(t *testing.T) {
	assertQueryFailureIs500(t, "templates json", "/api/templates", func(s *Server, c *gin.Context) {
		s.handleListTemplatesJSON(c)
	})
}

// TestChatAndMeshReads_QueryFailureIsNotAnEmptyList covers the chat
// history/channels and mesh route/topology endpoints: an empty 200 on each
// would read as "quiet channel", "no channels", "no peers".
func TestChatAndMeshReads_QueryFailureIsNotAnEmptyList(t *testing.T) {
	assertQueryFailureIs500(t, "chat history", "/chat/history", func(s *Server, c *gin.Context) {
		s.handleAPIChatHistory(c)
	})
	assertQueryFailureIs500(t, "chat channels", "/chat/channels", func(s *Server, c *gin.Context) {
		s.handleAPIChatChannels(c)
	})
	assertQueryFailureIs500(t, "mesh topology", "/api/mesh/topology", func(s *Server, c *gin.Context) {
		s.handleAPIMeshTopology(c)
	})
}

// TestTimeline_QueryFailureIsNotAnEmptyList proves the merged timeline
// refuses to answer with half its feeds missing: a silent drop shows a
// partial history as if complete.
func TestTimeline_QueryFailureIsNotAnEmptyList(t *testing.T) {
	assertQueryFailureIs500(t, "timeline", "/api/timeline", func(s *Server, c *gin.Context) {
		s.handleAPITimelineData(c)
	})
}

// TestChainTopology_QueryFailureIsNotAnEmptyList proves the three chain
// views refuse to draw an empty topology on a failed relation load.
func TestChainTopology_QueryFailureIsNotAnEmptyList(t *testing.T) {
	assertQueryFailureIs500(t, "chain graph", "/api/chain/graph", func(s *Server, c *gin.Context) {
		s.handleAPIChainGraph(c)
	})
	assertQueryFailureIs500(t, "chain list", "/api/chain/list", func(s *Server, c *gin.Context) {
		s.handleAPIChainList(c)
	})
	assertQueryFailureIs500(t, "agent chain get", "/api/agents/a1/chain", func(s *Server, c *gin.Context) {
		s.handleAgentChainGet(c)
	})
}
