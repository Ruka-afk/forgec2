package server

import (
	"net/http"
	"strings"
	"unicode"

	"github.com/gin-gonic/gin"
)

// renderPageOrJSON returns page data as JSON for the Next.js frontend.
func (s *Server) renderPageOrJSON(c *gin.Context, data gin.H) {
	s.addUserToData(c, data)
	addSnakeCaseKeys(data)
	c.JSON(http.StatusOK, data)
}

// toSnakeCase converts a PascalCase or camelCase string to snake_case.
func toSnakeCase(s string) string {
	var result []rune
	for i, r := range s {
		if i > 0 {
			prev := rune(s[i-1])
			if unicode.IsUpper(r) {
				if unicode.IsLower(prev) {
					result = append(result, '_')
				} else if i+1 < len(s) && unicode.IsLower(rune(s[i+1])) {
					result = append(result, '_')
				}
			}
		}
		result = append(result, unicode.ToLower(r))
	}
	return strings.ToLower(string(result))
}

// addSnakeCaseKeys adds snake_case aliases for every PascalCase key in the map.
// This allows the frontend to access data using snake_case keys while preserving
// PascalCase keys for HTML template compatibility.
func addSnakeCaseKeys(data gin.H) {
	for k, v := range data {
		snake := toSnakeCase(k)
		if snake != k {
			if _, exists := data[snake]; !exists {
				data[snake] = v
			}
		}
	}
}
