package server

import (
	"fmt"
	"html/template"
	"os"
	"strings"
)

type pageScriptEntry struct {
	Bundle        string
	Vendors       []string // third-party libs always loaded (not in app bundles)
	Scripts       []string // page scripts for dev mode
}

// pageScriptMap maps content template names to JS bundles (prod) and individual scripts (dev).
var pageScriptMap = map[string]pageScriptEntry{
	"agents_content":          {Bundle: "agents.bundle.js", Vendors: []string{"virtual-list.js"}, Scripts: []string{"virtual-list.js", "agents.js"}},
	"agent_detail_content":    {Bundle: "agents.bundle.js", Scripts: []string{"agent-detail.js"}},
	"shell_content":           {Bundle: "agents.bundle.js", Scripts: []string{"shell.js"}},
	"files_content":           {Bundle: "agents.bundle.js", Scripts: []string{"files.js"}},
	"dashboard_content":       {Bundle: "dashboard.bundle.js", Vendors: []string{"chart.min.js", "charts.js"}, Scripts: []string{"chart.min.js", "charts.js", "dashboard.js"}},
	"topology_content":        {Bundle: "dashboard.bundle.js", Vendors: []string{"vis-network.min.js"}, Scripts: []string{"vis-network.min.js", "topology.js"}},
	"settings_content":        {Bundle: "settings.bundle.js", Scripts: []string{"settings.js"}},
	"credentials_content":     {Bundle: "ops.bundle.js", Vendors: []string{"virtual-list.js"}, Scripts: []string{"virtual-list.js", "credentials.js"}},
	"tasks_content":           {Bundle: "ops.bundle.js", Vendors: []string{"virtual-list.js"}, Scripts: []string{"virtual-list.js", "tasks.js"}},
	"audit_content":           {Bundle: "ops.bundle.js", Vendors: []string{"virtual-list.js"}, Scripts: []string{"virtual-list.js", "audit.js"}},
	"plugins_content":         {Bundle: "plugins.bundle.js", Scripts: []string{"plugins.js"}},
	"bof_content":             {Bundle: "plugins.bundle.js", Scripts: []string{"bof.js"}},
	"generate_content":        {Bundle: "generate.bundle.js", Scripts: []string{"generate.js"}},
	"listeners_content":       {Bundle: "listeners.bundle.js", Scripts: []string{"listeners.js"}},
	"listener_detail_content": {Bundle: "listeners.bundle.js", Scripts: []string{"listeners.js"}},
	"ai_content":              {Bundle: "comms.bundle.js", Scripts: []string{"ai.js"}},
	"chat_content":            {Bundle: "comms.bundle.js", Scripts: []string{"chat.js"}},
	"toolkit_content":         {Bundle: "toolkit.bundle.js", Scripts: []string{"toolkit.js"}},
	"lateral_content":         {Bundle: "toolkit.bundle.js", Scripts: []string{"lateral.js"}},
	"loot_content":            {Bundle: "report.bundle.js", Scripts: []string{"loot.js"}},
	"privesc_content":         {Bundle: "toolkit.bundle.js", Scripts: []string{"privesc.js"}},
	"report_content":          {Bundle: "report.bundle.js", Scripts: []string{"report.js"}},
	"scanner_content":         {Bundle: "toolkit.bundle.js", Scripts: []string{"scanner.js"}},
	"token_content":           {Bundle: "admin.bundle.js", Scripts: []string{"token.js"}},
	"templates_content":       {Bundle: "admin.bundle.js", Scripts: []string{"templates.js"}},
	"timeline_content":        {Bundle: "report.bundle.js", Scripts: []string{"timeline.js"}},
	"traffic_content":         {Bundle: "comms.bundle.js", Scripts: []string{"traffic.js"}},
	"automation_content":      {Bundle: "report.bundle.js", Scripts: []string{"automation.js"}},
	"infrastructure_content":  {Bundle: "report.bundle.js", Scripts: []string{"infrastructure.js"}},
	"users_content":           {Bundle: "admin.bundle.js", Scripts: []string{"users.js"}},
	"screen_content":          {Bundle: "admin.bundle.js", Scripts: []string{"screen.js"}},
	"pivoting_content":        {Bundle: "admin.bundle.js", Scripts: []string{"pivoting.js"}},
	"bof_repo_content":        {Bundle: "admin.bundle.js", Scripts: []string{"bof_repo.js"}},
	"search_content":          {Bundle: "search.bundle.js", Scripts: []string{"search.js"}},
	"builds_content":          {Scripts: []string{}},
	"tokens_global_content":   {Scripts: []string{"token.js"}},
}

func appendPageScripts(tmplName string, content template.HTML, useBundles bool) template.HTML {
	entry, ok := pageScriptMap[tmplName]
	if !ok {
		return content
	}

	var b strings.Builder
	b.WriteString(string(content))

	if useBundles && entry.Bundle != "" {
		for _, vendor := range entry.Vendors {
			b.WriteString(fmt.Sprintf(`<script src="/static/js/%s"></script>`, vendor))
		}
		b.WriteString(fmt.Sprintf(`<script src="/static/js/%s"></script>`, entry.Bundle))
		return template.HTML(b.String())
	}

	for _, script := range entry.Scripts {
		b.WriteString(fmt.Sprintf(`<script src="/static/js/%s"></script>`, script))
	}
	return template.HTML(b.String())
}

func isDevMode() bool {
	return os.Getenv("FORGEC2_DEV_MODE") == "1"
}