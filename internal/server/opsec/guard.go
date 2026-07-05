package opsec

import (
	"fmt"
	"strings"
	"sync"
)

type RiskLevel int

const (
	RiskLow     RiskLevel = 1
	RiskMedium  RiskLevel = 2
	RiskHigh    RiskLevel = 3
	RiskCritical RiskLevel = 4
)

type Action int

const (
	ActionBlock Action = 0
	ActionWarn  Action = 1
	ActionBypass Action = 2
)

type OpsecResult struct {
	Allowed     bool
	RuleName    string
	Message     string
	RiskLevel   RiskLevel
	ActionTaken Action
}

type OpsecContext struct {
	AgentID   string
	Username  string
	Hostname  string
	IP        string
	Domain    string
	TaskType  string
	IsDA      bool
	Processes []ProcessInfo
}

type ProcessInfo struct {
	Name     string
	PID      int
	Path     string
}

type Rule struct {
	Name        string
	Description string
	RiskLevel   RiskLevel
	DefaultAction Action
	Check       func(*OpsecContext) *OpsecResult
}

var (
	mu    sync.RWMutex
	rules []Rule
)

func init() {
	rules = defaultRules()
}

func defaultRules() []Rule {
	return []Rule{
		{
			Name:          "mimikatz_da_check",
			Description:   "Block mimikatz when running as Domain Admin",
			RiskLevel:     RiskCritical,
			DefaultAction: ActionBlock,
			Check: func(ctx *OpsecContext) *OpsecResult {
				if ctx.TaskType != "mimikatz" && ctx.TaskType != "dcsync" {
					return nil
				}
				if ctx.IsDA {
					return &OpsecResult{Allowed: false, RuleName: "mimikatz_da_check",
						Message: fmt.Sprintf("Blocked %s: running as Domain Admin (%s\\%s) is extremely dangerous",
							ctx.TaskType, ctx.Domain, ctx.Username)}
				}
				return nil
			},
		},
		{
			Name:          "lsass_edr_check",
			Description:   "Block LSASS operations when EDR processes are detected",
			RiskLevel:     RiskCritical,
			DefaultAction: ActionBlock,
			Check: func(ctx *OpsecContext) *OpsecResult {
				if ctx.TaskType != "mimikatz" && ctx.TaskType != "creds" {
					return nil
				}
				edrs := []string{"crowdstrike", "csfalcon", "sentinelone", "sentinelagent",
					"cylance", "carbonblack", "cb.exe", "defender", "msmpeng",
					"symantec", "sep", "trendmicro", "tmcc", "mcafee", "mfe",
					"paloaltonetworks", "traps", "sophos", "sav", "kaspersky",
					"avp", "bitdefender", "vsserv", "f-secure", "fsav",
					"eset", "ekrn", "fortinet", "fmon", "webroot", "wrsa"}
				for _, p := range ctx.Processes {
					lower := strings.ToLower(p.Name)
					for _, edr := range edrs {
						if strings.Contains(lower, edr) {
							return &OpsecResult{Allowed: false, RuleName: "lsass_edr_check",
								Message: fmt.Sprintf("Blocked %s: EDR process '%s' detected (PID: %d)",
									ctx.TaskType, p.Name, p.PID),
								RiskLevel: RiskCritical, ActionTaken: ActionBlock}
						}
					}
				}
				return nil
			},
		},
		{
			Name:          "inject_safe_check",
			Description:   "Warn when injecting into protected processes",
			RiskLevel:     RiskHigh,
			DefaultAction: ActionWarn,
			Check: func(ctx *OpsecContext) *OpsecResult {
				if ctx.TaskType != "inject" && ctx.TaskType != "shinject" {
					return nil
				}
				protected := []string{"lsass.exe", "winlogon.exe", "services.exe",
					"svchost.exe", "csrss.exe", "smss.exe", "system"}
				for _, p := range ctx.Processes {
					lower := strings.ToLower(p.Name)
					for _, prot := range protected {
						if strings.Contains(lower, prot) {
							return &OpsecResult{Allowed: true, RuleName: "inject_safe_check",
								Message: fmt.Sprintf("Warning: injecting into '%s' may trigger alerts", p.Name),
								RiskLevel: RiskHigh, ActionTaken: ActionWarn}
						}
					}
				}
				return nil
			},
		},
		{
			Name:          "net_ad_check",
			Description:   "Warn when running recon commands as high-privilege user",
			RiskLevel:     RiskMedium,
			DefaultAction: ActionWarn,
			Check: func(ctx *OpsecContext) *OpsecResult {
				reconTypes := map[string]bool{"ldap_users": true, "ldap_groups": true,
					"ldap_computers": true, "ldap_spn": true, "ldap_query": true,
					"net": true, "netstat": true, "users": true, "portscan": true}
				if !reconTypes[ctx.TaskType] {
					return nil
				}
				if ctx.IsDA {
					return &OpsecResult{Allowed: true, RuleName: "net_ad_check",
						Message: fmt.Sprintf("Running '%s' as high-privilege user; LDAP queries may be audited",
							ctx.TaskType),
						RiskLevel: RiskMedium, ActionTaken: ActionWarn}
				}
				return nil
			},
		},
	}
}

func SetRules(newRules []Rule) {
	mu.Lock()
	defer mu.Unlock()
	rules = newRules
}

func CheckTask(ctx *OpsecContext) []*OpsecResult {
	mu.RLock()
	defer mu.RUnlock()

	var results []*OpsecResult
	for _, rule := range rules {
		result := rule.Check(ctx)
		if result == nil {
			continue
		}
		result.RuleName = rule.Name
		result.RiskLevel = rule.RiskLevel
		if result.ActionTaken == 0 {
			result.ActionTaken = rule.DefaultAction
		}
		if result.ActionTaken == ActionBlock {
			result.Allowed = false
		} else {
			result.Allowed = true
		}
		results = append(results, result)
	}
	return results
}
