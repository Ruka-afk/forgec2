package server

import (
	"net/http"
	"sync"
	"time"

	"github.com/forgec2/forgec2/internal/malleable"
	"github.com/gin-gonic/gin"
)

type ProfileManager struct {
	mu             sync.RWMutex
	activeProfiles map[string]string // agentID -> profileName
	profiles       map[string]*malleable.Profile
	stats          map[string]*profileStats // profileName -> stats
	defaultProfile string
}

type profileStats struct {
	activeAgents  int
	lastSwitched  time.Time
	switchCount   int
}

func NewProfileManager() *ProfileManager {
	pm := &ProfileManager{
		activeProfiles: make(map[string]string),
		profiles:       make(map[string]*malleable.Profile),
		stats:          make(map[string]*profileStats),
		defaultProfile: "default",
	}
	presets := malleable.PredefinedProfiles()
	for name, profile := range presets {
		pm.profiles[name] = profile
		pm.stats[name] = &profileStats{}
	}
	return pm
}

func (pm *ProfileManager) GetProfile(name string) *malleable.Profile {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	if p, ok := pm.profiles[name]; ok {
		return p
	}
	if p, ok := pm.profiles[pm.defaultProfile]; ok {
		return p
	}
	return nil
}

func (pm *ProfileManager) GetActiveProfile(agentID string) string {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	if name, ok := pm.activeProfiles[agentID]; ok {
		return name
	}
	return pm.defaultProfile
}

func (pm *ProfileManager) SetActiveProfile(agentID, profileName string) bool {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	if _, ok := pm.profiles[profileName]; !ok {
		return false
	}
	prev := pm.activeProfiles[agentID]
	pm.activeProfiles[agentID] = profileName
	if prev != "" && prev != profileName {
		if s, ok := pm.stats[prev]; ok {
			s.activeAgents--
		}
	}
	if s, ok := pm.stats[profileName]; ok {
		s.activeAgents++
		s.lastSwitched = time.Now()
		s.switchCount++
	}
	return true
}

func (pm *ProfileManager) RemoveAgent(agentID string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	if name, ok := pm.activeProfiles[agentID]; ok {
		if s, ok := pm.stats[name]; ok {
			s.activeAgents--
		}
		delete(pm.activeProfiles, agentID)
	}
}

func (pm *ProfileManager) ListProfiles() []gin.H {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	out := make([]gin.H, 0, len(pm.profiles))
	for name, profile := range pm.profiles {
		stats := pm.stats[name]
		out = append(out, gin.H{
			"name":         name,
			"description":  profile.Description,
			"active_agents": stats.activeAgents,
			"switch_count":  stats.switchCount,
			"last_switched": stats.lastSwitched,
		})
	}
	return out
}

func (pm *ProfileManager) ListActiveAgents() map[string]string {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	out := make(map[string]string, len(pm.activeProfiles))
	for k, v := range pm.activeProfiles {
		out[k] = v
	}
	return out
}

func (s *Server) apiListMalleableProfiles(c *gin.Context) {
	if s.profileManager == nil {
		respondSuccess(c, nil)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": s.profileManager.ListProfiles()})
}

func (s *Server) apiSetActiveProfile(c *gin.Context) {
	var req struct {
		AgentID string `json:"agent_id" binding:"required"`
		Profile string `json:"profile" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}
	if s.profileManager == nil {
		s.profileManager = NewProfileManager()
	}
	if !s.profileManager.SetActiveProfile(req.AgentID, req.Profile) {
		respondError(c, http.StatusBadRequest, "unknown profile: "+req.Profile)
		return
	}
	respondSuccess(c, gin.H{
		"agent_id": req.AgentID,
		"profile":  req.Profile,
	})
}

func (s *Server) apiGetActiveProfiles(c *gin.Context) {
	if s.profileManager == nil {
		respondSuccess(c, gin.H{})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": s.profileManager.ListActiveAgents()})
}

func (s *Server) apiBatchSetActiveProfile(c *gin.Context) {
	var req struct {
		AgentIDs []string `json:"agent_ids" binding:"required"`
		Profile  string   `json:"profile" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}
	if s.profileManager == nil {
		s.profileManager = NewProfileManager()
	}
	if _, ok := s.profileManager.profiles[req.Profile]; !ok {
		respondError(c, http.StatusBadRequest, "unknown profile: "+req.Profile)
		return
	}
	updated := make([]string, 0, len(req.AgentIDs))
	for _, id := range req.AgentIDs {
		if s.profileManager.SetActiveProfile(id, req.Profile) {
			updated = append(updated, id)
		}
	}
	respondSuccess(c, gin.H{
		"updated": updated,
		"profile": req.Profile,
	})
}
