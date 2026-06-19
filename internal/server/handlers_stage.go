package server

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
)

// handleServeStage serves the XOR-encoded stage file for Artifact Kit stagers.
// The xorKey parameter is the full hex-encoded XOR key, used as filename.
func (s *Server) handleServeStage(c *gin.Context) {
	xorKey := c.Param("xorKey")
	if xorKey == "" {
		c.String(http.StatusBadRequest, "missing xor key")
		return
	}

	// Sanitize: only allow hex characters
	for _, ch := range xorKey {
		if !((ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f') || (ch >= 'A' && ch <= 'F')) {
			c.String(http.StatusBadRequest, "invalid xor key")
			return
		}
	}

	agentsDir := s.cfg.Server.DataDir
	if agentsDir == "" {
		agentsDir = "data"
	}

	stagePath := filepath.Join(agentsDir, "agents", "stage_"+xorKey+".enc")
	stagePath = filepath.Clean(stagePath)

	// Verify the file is within the agents directory to prevent traversal
	allowedDir := filepath.Clean(filepath.Join(agentsDir, "agents"))
	if !isPathWithin(allowedDir, stagePath) {
		c.String(http.StatusForbidden, "forbidden")
		return
	}

	if _, err := os.Stat(stagePath); os.IsNotExist(err) {
		c.String(http.StatusNotFound, "stage not found")
		return
	}

	c.File(stagePath)
}

func isPathWithin(base, target string) bool {
	base = filepath.Clean(base)
	target = filepath.Clean(target)
	return len(target) >= len(base) && target[:len(base)] == base &&
		(len(target) == len(base) || target[len(base)] == filepath.Separator)
}
