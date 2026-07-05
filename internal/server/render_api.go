package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// renderPageOrJSON returns page data as JSON for the Next.js frontend.
func (s *Server) renderPageOrJSON(c *gin.Context, data gin.H) {
	s.addUserToData(c, data)
	c.JSON(http.StatusOK, data)
}
