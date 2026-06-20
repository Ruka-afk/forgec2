package report

import (
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"time"
)

// ReportData contains all data for report generation
type ReportData struct {
	Title       string
	Subtitle    string
	GeneratedAt time.Time
	DateRange   DateRange
	ExecutedBy  string

	// Statistics
	TotalAgents    int
	OnlineAgents   int
	TotalTasks     int
	CompletedTasks int
	FailedTasks    int
	SuccessRate    float64

	// Data sections
	Agents          []AgentSummary
	Tasks           []TaskSummary
	Credentials     []CredentialSummary
	Screenshots     []ScreenshotSummary
	Vulnerabilities []VulnSummary
	IOC             []IOCEntry

	// Customization
	Logo        string // Base64 encoded
	ColorScheme string
	Header      string
	Footer      string
}

// DateRange represents the report time range
type DateRange struct {
	Start time.Time
	End   time.Time
}

// AgentSummary contains agent statistics
type AgentSummary struct {
	ID        string
	Hostname  string
	IP        string
	OS        string
	Username  string
	Status    string
	FirstSeen time.Time
	LastSeen  time.Time
	TaskCount int
}

// TaskSummary contains task statistics
type TaskSummary struct {
	ID        uint
	AgentID   string
	Type      string
	Command   string
	Status    string
	Result    string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// CredentialSummary contains credential information
type CredentialSummary struct {
	ID        uint
	AgentID   string
	Source    string
	Username  string
	Password  string
	Hash      string
	CreatedAt time.Time
}

// ScreenshotSummary contains screenshot information
type ScreenshotSummary struct {
	AgentID   string
	Filename  string
	Timestamp time.Time
}

// VulnSummary contains vulnerability information
type VulnSummary struct {
	AgentID     string
	Type        string
	Severity    string
	Description string
	CVE         string
}

// IOCEntry contains indicator of compromise
type IOCEntry struct {
	Type        string // IP, Domain, Hash, File
	Value       string
	Description string
	FirstSeen   time.Time
}

// Generator generates reports in various formats
type Generator struct {
	dataDir string
}

// NewGenerator creates a new report generator
func NewGenerator(dataDir string) *Generator {
	return &Generator{
		dataDir: dataDir,
	}
}

// GenerateHTML generates an HTML report
func (g *Generator) GenerateHTML(data *ReportData, outputPath string) error {
	tmpl := `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>{{.Title}}</title>
    <style>
        body { font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif; margin: 40px; color: #333; }
        .header { text-align: center; margin-bottom: 40px; border-bottom: 3px solid #4F46E5; padding-bottom: 20px; }
        .header h1 { color: #4F46E5; margin: 0; }
        .header .subtitle { color: #666; font-size: 14px; margin-top: 5px; }
        .section { margin-bottom: 30px; }
        .section h2 { color: #4F46E5; border-left: 4px solid #4F46E5; padding-left: 10px; }
        .stats-grid { display: grid; grid-template-columns: repeat(3, 1fr); gap: 20px; margin-bottom: 30px; }
        .stat-card { background: #f8f9fa; padding: 20px; border-radius: 8px; text-align: center; }
        .stat-card .value { font-size: 36px; font-weight: bold; color: #4F46E5; }
        .stat-card .label { font-size: 14px; color: #666; margin-top: 5px; }
        table { width: 100%; border-collapse: collapse; margin-bottom: 20px; }
        th { background: #4F46E5; color: white; padding: 12px; text-align: left; }
        td { padding: 10px; border-bottom: 1px solid #ddd; }
        tr:hover { background: #f5f5f5; }
        .success { color: #10B981; font-weight: bold; }
        .failed { color: #EF4444; font-weight: bold; }
        .footer { text-align: center; margin-top: 40px; padding-top: 20px; border-top: 1px solid #ddd; color: #999; font-size: 12px; }
    </style>
</head>
<body>
    <div class="header">
        {{if .Logo}}<img src="data:image/png;base64,{{.Logo}}" alt="Logo" style="max-height: 80px;">{{end}}
        <h1>{{.Title}}</h1>
        <div class="subtitle">{{.Subtitle}}</div>
        <div class="subtitle">生成时间: {{.GeneratedAt.Format "2006-01-02 15:04:05"}}</div>
        <div class="subtitle">执行者: {{.ExecutedBy}}</div>
        <div class="subtitle">日期范围: {{.DateRange.Start.Format "2006-01-02"}} - {{.DateRange.End.Format "2006-01-02"}}</div>
    </div>

    <div class="section">
        <h2>执行概览</h2>
        <div class="stats-grid">
            <div class="stat-card">
                <div class="value">{{.TotalAgents}}</div>
                <div class="label">总 Agent 数</div>
            </div>
            <div class="stat-card">
                <div class="value">{{.OnlineAgents}}</div>
                <div class="label">在线 Agent</div>
            </div>
            <div class="stat-card">
                <div class="value">{{printf "%.1f" .SuccessRate}}%</div>
                <div class="label">任务成功率</div>
            </div>
        </div>
        <div class="stats-grid">
            <div class="stat-card">
                <div class="value">{{.TotalTasks}}</div>
                <div class="label">总任务数</div>
            </div>
            <div class="stat-card">
                <div class="value success">{{.CompletedTasks}}</div>
                <div class="label">成功任务</div>
            </div>
            <div class="stat-card">
                <div class="value failed">{{.FailedTasks}}</div>
                <div class="label">失败任务</div>
            </div>
        </div>
    </div>

    {{if .Agents}}
    <div class="section">
        <h2>Agent 列表</h2>
        <table>
            <thead>
                <tr>
                    <th>主机名</th>
                    <th>IP 地址</th>
                    <th>操作系统</th>
                    <th>用户</th>
                    <th>状态</th>
                    <th>任务数</th>
                </tr>
            </thead>
            <tbody>
                {{range .Agents}}
                <tr>
                    <td>{{.Hostname}}</td>
                    <td>{{.IP}}</td>
                    <td>{{.OS}}</td>
                    <td>{{.Username}}</td>
                    <td>{{.Status}}</td>
                    <td>{{.TaskCount}}</td>
                </tr>
                {{end}}
            </tbody>
        </table>
    </div>
    {{end}}

    {{if .Credentials}}
    <div class="section">
        <h2>凭据收集</h2>
        <table>
            <thead>
                <tr>
                    <th>来源</th>
                    <th>用户名</th>
                    <th>密码/Hash</th>
                    <th>收集时间</th>
                </tr>
            </thead>
            <tbody>
                {{range .Credentials}}
                <tr>
                    <td>{{.Source}}</td>
                    <td>{{.Username}}</td>
                    <td>{{if .Password}}{{.Password}}{{else}}{{.Hash}}{{end}}</td>
                    <td>{{.CreatedAt.Format "2006-01-02 15:04"}}</td>
                </tr>
                {{end}}
            </tbody>
        </table>
    </div>
    {{end}}

    {{if .Vulnerabilities}}
    <div class="section">
        <h2>发现的漏洞</h2>
        <table>
            <thead>
                <tr>
                    <th>Agent</th>
                    <th>类型</th>
                    <th>严重程度</th>
                    <th>描述</th>
                    <th>CVE</th>
                </tr>
            </thead>
            <tbody>
                {{range .Vulnerabilities}}
                <tr>
                    <td>{{.AgentID}}</td>
                    <td>{{.Type}}</td>
                    <td>{{.Severity}}</td>
                    <td>{{.Description}}</td>
                    <td>{{.CVE}}</td>
                </tr>
                {{end}}
            </tbody>
        </table>
    </div>
    {{end}}

    {{if .IOC}}
    <div class="section">
        <h2>IOC (威胁指标)</h2>
        <table>
            <thead>
                <tr>
                    <th>类型</th>
                    <th>值</th>
                    <th>描述</th>
                    <th>首次发现</th>
                </tr>
            </thead>
            <tbody>
                {{range .IOC}}
                <tr>
                    <td>{{.Type}}</td>
                    <td>{{.Value}}</td>
                    <td>{{.Description}}</td>
                    <td>{{.FirstSeen.Format "2006-01-02"}}</td>
                </tr>
                {{end}}
            </tbody>
        </table>
    </div>
    {{end}}

    <div class="footer">
        {{.Footer}}<br>
        ForgeC2 Professional Report - Confidential
    </div>
</body>
</html>
`

	t, err := template.New("report").Parse(tmpl)
	if err != nil {
		return err
	}

	f, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer f.Close()

	return t.Execute(f, data)
}

// GenerateJSON generates a JSON report
func (g *Generator) GenerateJSON(data *ReportData, outputPath string) error {
	f, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer f.Close()

	encoder := json.NewEncoder(f)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

// GenerateMarkdown generates a Markdown report
func (g *Generator) GenerateMarkdown(data *ReportData, outputPath string) error {
	md := fmt.Sprintf(`# %s

%s

**生成时间**: %s  
**执行者**: %s  
**日期范围**: %s - %s

---

## 执行概览

| 指标 | 数值 |
|------|------|
| 总 Agent 数 | %d |
| 在线 Agent | %d |
| 总任务数 | %d |
| 成功任务 | %d |
| 失败任务 | %d |
| 成功率 | %.1f%% |

`,
		data.Title,
		data.Subtitle,
		data.GeneratedAt.Format("2006-01-02 15:04:05"),
		data.ExecutedBy,
		data.DateRange.Start.Format("2006-01-02"),
		data.DateRange.End.Format("2006-01-02"),
		data.TotalAgents,
		data.OnlineAgents,
		data.TotalTasks,
		data.CompletedTasks,
		data.FailedTasks,
		data.SuccessRate,
	)

	if len(data.Agents) > 0 {
		md += "\n## Agent 列表\n\n"
		md += "| 主机名 | IP | OS | 用户 | 状态 | 任务数 |\n"
		md += "|--------|----|----|----|----|--------|\n"
		for _, a := range data.Agents {
			md += fmt.Sprintf("| %s | %s | %s | %s | %s | %d |\n",
				a.Hostname, a.IP, a.OS, a.Username, a.Status, a.TaskCount)
		}
	}

	if len(data.Credentials) > 0 {
		md += "\n## 凭据收集\n\n"
		md += "| 来源 | 用户名 | 密码/Hash | 时间 |\n"
		md += "|------|--------|-----------|------|\n"
		for _, c := range data.Credentials {
			secret := c.Password
			if secret == "" {
				secret = c.Hash
			}
			md += fmt.Sprintf("| %s | %s | %s | %s |\n",
				c.Source, c.Username, secret, c.CreatedAt.Format("2006-01-02 15:04"))
		}
	}

	if len(data.Vulnerabilities) > 0 {
		md += "\n## 发现的漏洞\n\n"
		md += "| Agent | 类型 | 严重程度 | 描述 | CVE |\n"
		md += "|-------|------|----------|------|-----|\n"
		for _, v := range data.Vulnerabilities {
			md += fmt.Sprintf("| %s | %s | %s | %s | %s |\n",
				v.AgentID, v.Type, v.Severity, v.Description, v.CVE)
		}
	}

	if len(data.IOC) > 0 {
		md += "\n## IOC (威胁指标)\n\n"
		md += "| 类型 | 值 | 描述 | 首次发现 |\n"
		md += "|------|----|----|----------|\n"
		for _, i := range data.IOC {
			md += fmt.Sprintf("| %s | %s | %s | %s |\n",
				i.Type, i.Value, i.Description, i.FirstSeen.Format("2006-01-02"))
		}
	}

	md += fmt.Sprintf("\n---\n\n%s\n\n*ForgeC2 Professional Report - Confidential*\n", data.Footer)

	return os.WriteFile(outputPath, []byte(md), 0644)
}

// GetOutputPath generates a unique output path
func (g *Generator) GetOutputPath(format string) string {
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("forgec2_report_%s.%s", timestamp, format)
	return filepath.Join(g.dataDir, "reports", filename)
}
