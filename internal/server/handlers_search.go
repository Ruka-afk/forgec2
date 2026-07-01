package server

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
)

type SearchResult struct {
	Type     string `json:"type"`
	ID       string `json:"id"`
	Title    string `json:"title"`
	Subtitle string `json:"subtitle"`
	URL      string `json:"url"`
	Icon     string `json:"icon"`
}

func (s *Server) handleSearchPage(c *gin.Context) {
	q := strings.TrimSpace(c.Query("q"))
	stats := s.getNavStats()
	data := gin.H{
		"Title":       "ForgeC2 - Search",
		"ActiveNav":   "search",
		"SearchQuery": q,
	}
	for k, v := range stats {
		data[k] = v
	}
	s.renderPage(c, "search_content", data)
}

func (s *Server) handleAPISearch(c *gin.Context) {
	q := strings.TrimSpace(c.Query("q"))
	if q == "" {
		c.JSON(http.StatusOK, gin.H{"success": true, "results": []SearchResult{}, "query": ""})
		return
	}

	like := "%" + q + "%"
	const perType = 8
	var results []SearchResult

	var agents []db.Implant
	s.db.Select("id", "hostname", "username", "ip", "os", "status").
		Where("hostname LIKE ? OR username LIKE ? OR ip LIKE ? OR notes LIKE ? OR id LIKE ?",
			like, like, like, like, like).
		Limit(perType).Find(&agents)
	for _, a := range agents {
		results = append(results, SearchResult{
			Type:     "agent",
			ID:       a.ID,
			Title:    a.Hostname,
			Subtitle: fmt.Sprintf("%s@%s · %s · %s", a.Username, a.IP, a.OS, a.Status),
			URL:      "/agents/" + a.ID,
			Icon:     "fa-bug",
		})
	}

	var listeners []db.Listener
	s.db.Select("id", "name", "host", "port", "scheme").
		Where("name LIKE ? OR host LIKE ? OR notes LIKE ?", like, like, like).
		Limit(perType).Find(&listeners)
	for _, l := range listeners {
		results = append(results, SearchResult{
			Type:     "listener",
			ID:       fmt.Sprintf("%d", l.ID),
			Title:    l.Name,
			Subtitle: fmt.Sprintf("%s://%s:%d", l.Scheme, l.Host, l.Port),
			URL:      fmt.Sprintf("/listeners/%d", l.ID),
			Icon:     "fa-satellite-dish",
		})
	}

	var creds []db.CredentialEntry
	s.db.Select("id", "domain", "username", "source", "type").
		Where("domain LIKE ? OR username LIKE ? OR notes LIKE ?", like, like, like).
		Limit(perType).Find(&creds)
	for _, cr := range creds {
		results = append(results, SearchResult{
			Type:     "credential",
			ID:       fmt.Sprintf("%d", cr.ID),
			Title:    cr.Domain + "\\" + cr.Username,
			Subtitle: cr.Source + " · " + cr.Type,
			URL:      "/credentials",
			Icon:     "fa-key",
		})
	}

	var bofs []db.BOFFile
	s.db.Select("id", "name", "description").
		Where("name LIKE ? OR description LIKE ?", like, like).
		Limit(perType).Find(&bofs)
	for _, b := range bofs {
		results = append(results, SearchResult{
			Type:     "bof",
			ID:       fmt.Sprintf("%d", b.ID),
			Title:    b.Name,
			Subtitle: b.Description,
			URL:      "/bof",
			Icon:     "fa-cube",
		})
	}

	var users []db.User
	s.db.Select("id", "username", "role").
		Where("username LIKE ?", like).
		Limit(perType).Find(&users)
	for _, u := range users {
		results = append(results, SearchResult{
			Type:     "user",
			ID:       fmt.Sprintf("%d", u.ID),
			Title:    u.Username,
			Subtitle: u.Role,
			URL:      "/users",
			Icon:     "fa-user-shield",
		})
	}

	var tasks []db.Task
	s.db.Select("id", "agent_id", "type", "command", "status").
		Where("command LIKE ? OR type LIKE ? OR agent_id LIKE ?", like, like, like).
		Order("created_at DESC").
		Limit(perType).Find(&tasks)
	for _, t := range tasks {
		cmd := t.Command
		if len(cmd) > 60 {
			cmd = cmd[:60] + "…"
		}
		results = append(results, SearchResult{
			Type:     "task",
			ID:       fmt.Sprintf("%d", t.ID),
			Title:    t.Type,
			Subtitle: fmt.Sprintf("%s · %s · %s", t.AgentID, t.Status, cmd),
			URL:      "/agents/" + t.AgentID,
			Icon:     "fa-terminal",
		})
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "results": results, "query": q})
}