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
