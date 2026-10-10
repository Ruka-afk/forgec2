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
