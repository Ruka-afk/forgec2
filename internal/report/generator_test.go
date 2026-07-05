package report

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func sampleData() *ReportData {
	return &ReportData{
		Title:          "Penetration Test Report",
		Subtitle:       "Internal Network Assessment",
		GeneratedAt:    time.Now(),
		ExecutedBy:     "Red Team",
		TotalAgents:    5,
		OnlineAgents:   3,
		TotalTasks:     100,
		CompletedTasks: 85,
		FailedTasks:    15,
		SuccessRate:    85.0,
		DateRange: DateRange{
			Start: time.Now().Add(-24 * time.Hour),
			End:   time.Now(),
		},
		Agents: []AgentSummary{
			{ID: "agent-1", Hostname: "WS-001", IP: "10.0.1.10", OS: "Windows 11", Username: "admin", Status: "Online", TaskCount: 30},
			{ID: "agent-2", Hostname: "SRV-001", IP: "10.0.1.20", OS: "Windows Server 2022", Username: "Administrator", Status: "Online", TaskCount: 45},
		},
		Credentials: []CredentialSummary{
			{ID: 1, AgentID: "agent-1", Source: "Mimikatz", Username: "CORP\\jdoe", Password: "P@ssw0rd!"},
		},
		Vulnerabilities: []VulnSummary{
			{AgentID: "agent-1", Type: "Missing Patch", Severity: "High", Description: "MS-2024-001", CVE: "CVE-2024-XXXX"},
		},
		IOC: []IOCEntry{
			{Type: "IP", Value: "192.168.1.100", Description: "C2 callback", FirstSeen: time.Now()},
		},
	}
}

func TestNewGenerator(t *testing.T) {
	g := NewGenerator("/tmp")
	if g == nil {
		t.Fatal("NewGenerator returned nil")
	}
	if g.dataDir != "/tmp" {
		t.Fatalf("dataDir = %q, want %q", g.dataDir, "/tmp")
	}
}

func TestGenerateHTML(t *testing.T) {
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "report.html")

	g := NewGenerator(tmpDir)
	err := g.GenerateHTML(sampleData(), outputPath)
	if err != nil {
		t.Fatalf("GenerateHTML() error = %v", err)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}

	content := string(data)
	if !strings.Contains(content, "Penetration Test Report") {
		t.Fatal("HTML should contain title")
	}
	if !strings.Contains(content, "WS-001") {
		t.Fatal("HTML should contain agent hostname")
	}
	if !strings.Contains(content, "CORP\\jdoe") {
		t.Fatal("HTML should contain credential username")
	}
	if !strings.Contains(content, "Completed Tasks") {
		t.Fatal("HTML should contain Completed Tasks")
	}
	if !strings.Contains(content, "Failed Tasks") {
		t.Fatal("HTML should contain Failed Tasks")
	}
}

func TestGenerateJSON(t *testing.T) {
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "report.json")

	g := NewGenerator(tmpDir)
	err := g.GenerateJSON(sampleData(), outputPath)
	if err != nil {
		t.Fatalf("GenerateJSON() error = %v", err)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}

	content := string(data)
	if !strings.Contains(content, "\"Title\": \"Penetration Test Report\"") {
		t.Fatal("JSON should contain title")
	}
	if !strings.Contains(content, "\"Hostname\": \"WS-001\"") {
		t.Fatal("JSON should contain agent hostname")
	}
}

func TestGenerateMarkdown(t *testing.T) {
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "report.md")

	g := NewGenerator(tmpDir)
	err := g.GenerateMarkdown(sampleData(), outputPath)
	if err != nil {
		t.Fatalf("GenerateMarkdown() error = %v", err)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}

	content := string(data)
	if !strings.Contains(content, "# Penetration Test Report") {
		t.Fatal("MD should contain title")
	}
	if !strings.Contains(content, "| WS-001 |") {
		t.Fatal("MD should contain agent row")
	}
	if !strings.Contains(content, "CORP\\jdoe") {
		t.Fatal("MD should contain credential row")
	}
}

func TestGenerateMarkdownEmptySections(t *testing.T) {
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "report.md")

	g := NewGenerator(tmpDir)
	data := &ReportData{
		Title:       "Empty Report",
		GeneratedAt: time.Now(),
	}

	err := g.GenerateMarkdown(data, outputPath)
	if err != nil {
		t.Fatal(err)
	}

	content, _ := os.ReadFile(outputPath)
	if !strings.Contains(string(content), "Empty Report") {
		t.Fatal("basic markdown should still render")
	}
}

func TestGetOutputPath(t *testing.T) {
	tmpDir := t.TempDir()
	g := NewGenerator(tmpDir)

	path := g.GetOutputPath("html")
	if !strings.HasSuffix(path, ".html") {
		t.Fatalf("expected .html suffix, got %s", path)
	}
	if !strings.Contains(path, "forgec2_report_") {
		t.Fatalf("expected forgec2_report_ prefix, got %s", path)
	}
	if !strings.HasPrefix(path, filepath.Join(tmpDir, "reports")) {
		t.Fatalf("expected reports/ subdirectory, got %s", path)
	}
}
