package server

import (
	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
)

// attackTechnique is the MITRE ATT&CK technique mapping used by the coverage page.
type attackTechnique struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Tactic    string   `json:"tactic"`
	TaskTypes []string `json:"task_types"`
}

// attackTacticMap maps each MITRE tactic to representative techniques and the
// ForgeC2 task types that satisfy them.
var attackTacticMap = []struct {
	Tactic     string
	Techniques []attackTechnique
}{
	{
		Tactic: "Execution",
		Techniques: []attackTechnique{
			{ID: "T1059", Name: "Command and Scripting Interpreter", Tactic: "Execution", TaskTypes: []string{"shell", "exec"}},
			{ID: "T1053", Name: "Scheduled Task/Job", Tactic: "Execution", TaskTypes: []string{"schtasks_add"}},
			{ID: "T1569", Name: "System Services", Tactic: "Execution", TaskTypes: []string{"run", "service_start"}},
		},
	},
	{
		Tactic: "Persistence",
		Techniques: []attackTechnique{
			{ID: "T1547", Name: "Boot or Logon Autostart", Tactic: "Persistence", TaskTypes: []string{"persist", "reg_add"}},
			{ID: "T1053", Name: "Scheduled Task/Job", Tactic: "Persistence", TaskTypes: []string{"schtasks_add"}},
			{ID: "T1543", Name: "Create or Modify System Process", Tactic: "Persistence", TaskTypes: []string{"service_create"}},
		},
	},
	{
		Tactic: "Privilege Escalation",
		Techniques: []attackTechnique{
			{ID: "T1134", Name: "Access Token Manipulation", Tactic: "Privilege Escalation", TaskTypes: []string{"elevate", "steal_token"}},
			{ID: "T1547", Name: "Boot or Logon Autostart", Tactic: "Privilege Escalation", TaskTypes: []string{"persist"}},
			{ID: "T1068", Name: "Exploitation for Privilege Escalation", Tactic: "Privilege Escalation", TaskTypes: []string{"exploit"}},
		},
	},
	{
		Tactic: "Defense Evasion",
		Techniques: []attackTechnique{
			{ID: "T1562", Name: "Impair Defenses", Tactic: "Defense Evasion", TaskTypes: []string{"disable_av", "killproc"}},
			{ID: "T1070", Name: "Indicator Removal", Tactic: "Defense Evasion", TaskTypes: []string{"clear_logs"}},
			{ID: "T1055", Name: "Process Injection", Tactic: "Defense Evasion", TaskTypes: []string{"inject"}},
			{ID: "T1547", Name: "Masquerading", Tactic: "Defense Evasion", TaskTypes: []string{"spoof"}},
		},
	},
	{
		Tactic: "Credential Access",
		Techniques: []attackTechnique{
			{ID: "T1003", Name: "OS Credential Dumping", Tactic: "Credential Access", TaskTypes: []string{"lsass", "mimikatz", "wdigest", "lsa_dump"}},
			{ID: "T1056", Name: "Input Capture", Tactic: "Credential Access", TaskTypes: []string{"keylogger_start", "clipboard_start"}},
			{ID: "T1110", Name: "Brute Force", Tactic: "Credential Access", TaskTypes: []string{"bruteforce"}},
		},
	},
	{
		Tactic: "Discovery",
		Techniques: []attackTechnique{
			{ID: "T1057", Name: "Process Discovery", Tactic: "Discovery", TaskTypes: []string{"ps"}},
			{ID: "T1083", Name: "File and Directory Discovery", Tactic: "Discovery", TaskTypes: []string{"ls", "find"}},
			{ID: "T1016", Name: "System Network Configuration Discovery", Tactic: "Discovery", TaskTypes: []string{"ipconfig", "arp", "net"}},
			{ID: "T1033", Name: "System Owner/User Discovery", Tactic: "Discovery", TaskTypes: []string{"whoami"}},
		},
	},
	{
		Tactic: "Collection",
		Techniques: []attackTechnique{
			{ID: "T1113", Name: "Screen Capture", Tactic: "Collection", TaskTypes: []string{"screenshot", "screen_stream_start"}},
			{ID: "T1056", Name: "Input Capture", Tactic: "Collection", TaskTypes: []string{"keylogger_start", "clipboard_start"}},
		},
	},
	{
		Tactic: "Lateral Movement",
		Techniques: []attackTechnique{
			{ID: "T1021", Name: "Remote Services", Tactic: "Lateral Movement", TaskTypes: []string{"wmi", "psexec", "winrm", "ssh"}},
			{ID: "T1570", Name: "Lateral Tool Transfer", Tactic: "Lateral Movement", TaskTypes: []string{"pivot_upload"}},
		},
	},
	{
		Tactic: "Command and Control",
		Techniques: []attackTechnique{
			{ID: "T1071", Name: "Application Layer Protocol", Tactic: "Command and Control", TaskTypes: []string{"beacon", "sleep", "profile_rotate"}},
			{ID: "T1573", Name: "Encrypted Channel", Tactic: "Command and Control", TaskTypes: []string{"beacon"}},
			{ID: "T1095", Name: "Non-Application Layer Protocol", Tactic: "Command and Control", TaskTypes: []string{"beacon"}},
		},
	},
}

// handleAttackCoverage returns MITRE ATT&CK technique coverage for an agent.
// GET /attack/coverage?agent_id=
func (s *Server) handleAttackCoverage(c *gin.Context) {
	agentID := c.Query("agent_id")

	used := map[string]bool{}
	if agentID != "" {
		var types []string
		s.db.Table("tasks").Select("DISTINCT type").Where("agent_id = ?", agentID).Pluck("type", &types)
		for _, t := range types {
			used[t] = true
		}
	} else {
		var types []string
		s.db.Table("tasks").Select("DISTINCT type").Pluck("type", &types)
		for _, t := range types {
			used[t] = true
		}
	}

	usedTaskTypes := make([]string, 0, len(used))
	for k := range used {
		usedTaskTypes = append(usedTaskTypes, k)
	}

	var tactics []map[string]interface{}
	total := 0
	totalCovered := 0
	for _, grp := range attackTacticMap {
		techs := make([]map[string]interface{}, 0, len(grp.Techniques))
		covered := 0
		for _, tech := range grp.Techniques {
			isCovered := false
			for _, tt := range tech.TaskTypes {
				if used[tt] {
					isCovered = true
					break
				}
			}
			if isCovered {
				covered++
				totalCovered++
			}
			total++
			techs = append(techs, map[string]interface{}{
				"id":         tech.ID,
				"name":       tech.Name,
				"tactic":     tech.Tactic,
				"task_types": tech.TaskTypes,
				"covered":    isCovered,
			})
		}
		tactics = append(tactics, map[string]interface{}{
			"tactic":     grp.Tactic,
			"techniques": techs,
			"covered":    covered,
			"total":      len(grp.Techniques),
		})
	}

	respond(c, gin.H{
		"tactics":         tactics,
		"total_covered":   totalCovered,
		"total":           total,
		"used_task_types": usedTaskTypes,
	})
}

// handleMitrePhases returns kill-chain phase coverage across all campaigns.
// GET /mitre/phases
func (s *Server) handleMitrePhases(c *gin.Context) {
	var campaigns []db.Campaign
	s.db.Preload("Agents").Limit(500).Find(&campaigns)

	phaseTasks := map[string]int{}
	phaseCampaigns := map[string]map[string]bool{}

	// Collect all agent IDs across campaigns
	allAgentIDs := make([]string, 0)
	for _, camp := range campaigns {
		for _, a := range camp.Agents {
			allAgentIDs = append(allAgentIDs, a.ID)
		}
	}

	// Batch-load all tasks for these agents
	type agentTask struct {
		AgentID string
		Type    string
	}
	var allTasks []agentTask
	if len(allAgentIDs) > 0 {
		s.db.Table("tasks").Select("agent_id, type").Where("agent_id IN ?", allAgentIDs).Find(&allTasks)
	}

	// Index tasks by agent
	tasksByAgent := map[string][]string{}
	for _, t := range allTasks {
		tasksByAgent[t.AgentID] = append(tasksByAgent[t.AgentID], t.Type)
	}

	for _, camp := range campaigns {
		campPhaseSet := map[string]bool{}
		for _, a := range camp.Agents {
			for _, tt := range tasksByAgent[a.ID] {
				p := taskPhase(tt)
				phaseTasks[p]++
				campPhaseSet[p] = true
			}
		}
		for p := range campPhaseSet {
			if phaseCampaigns[p] == nil {
				phaseCampaigns[p] = map[string]bool{}
			}
			phaseCampaigns[p][camp.ID] = true
		}
	}

	phases := make([]map[string]interface{}, 0, len(PHASE_ORDER))
	for _, p := range PHASE_ORDER {
		covered := 0
		if set, ok := phaseCampaigns[p]; ok {
			covered = len(set)
		}
		phases = append(phases, map[string]interface{}{
			"phase":             p,
			"total_tasks":       phaseTasks[p],
			"campaigns_covered": covered,
			"total_campaigns":   len(campaigns),
		})
	}

	respond(c, gin.H{"success": true, "data": phases})
}
