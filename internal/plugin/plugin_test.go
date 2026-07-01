package plugin

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestManifestValidation(t *testing.T) {
	tests := []struct {
		name      string
		manifest  Manifest
		wantError string
	}{
		{
			name:      "missing name",
			manifest:  Manifest{Version: "1.0.0", Type: "command", Entry: "run.py", Interpreter: "python"},
			wantError: "name is required",
		},
		{
			name:      "missing version",
			manifest:  Manifest{Name: "test", Type: "command", Entry: "run.py", Interpreter: "python"},
			wantError: "version is required",
		},
		{
			name:      "invalid type",
			manifest:  Manifest{Name: "test", Version: "1.0.0", Type: "invalid", Entry: "run.py", Interpreter: "python"},
			wantError: "invalid plugin type",
		},
		{
			name:      "missing entry",
			manifest:  Manifest{Name: "test", Version: "1.0.0", Type: "command", Interpreter: "python"},
			wantError: "entry is required",
		},
		{
			name:      "invalid interpreter",
			manifest:  Manifest{Name: "test", Version: "1.0.0", Type: "command", Entry: "run.py", Interpreter: "ruby"},
			wantError: "invalid interpreter",
		},
		{
			name:      "negative timeout",
			manifest:  Manifest{Name: "test", Version: "1.0.0", Type: "command", Entry: "run.py", Interpreter: "python", Timeout: -1},
			wantError: "timeout cannot be negative",
		},
		{
			name:     "valid command plugin",
			manifest: Manifest{Name: "test", Version: "1.0.0", Type: "command", Entry: "run.py", Interpreter: "python"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.manifest.Validate()
			if tt.wantError != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantError)
				}
				if !strings.Contains(err.Error(), tt.wantError) {
					t.Fatalf("expected error containing %q, got %q", tt.wantError, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestManifestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	manifest := &Manifest{
		Name:        "test",
		Version:     "1.0.0",
		Type:        "command",
		Description: "test plugin",
		Author:      "tester",
		Category:    "utility",
		Entry:       "run.py",
		Interpreter: "python",
		Timeout:     30,
		Params: []ManifestParam{
			{Name: "target", Type: "string", Required: true},
		},
		Config: map[string]interface{}{"key": "value"},
	}

	if err := manifest.Save(dir); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := LoadManifest(filepath.Join(dir, "manifest.yaml"))
	if err != nil {
		t.Fatalf("LoadManifest failed: %v", err)
	}

	if loaded.Name != manifest.Name {
		t.Fatalf("name mismatch: got %q want %q", loaded.Name, manifest.Name)
	}
	if loaded.Timeout != manifest.Timeout {
		t.Fatalf("timeout mismatch: got %d want %d", loaded.Timeout, manifest.Timeout)
	}
	if len(loaded.Params) != 1 || loaded.Params[0].Name != "target" {
		t.Fatalf("params mismatch: got %+v", loaded.Params)
	}
	if loaded.Config["key"] != "value" {
		t.Fatalf("config mismatch: got %+v", loaded.Config)
	}
}

func TestResultParsing(t *testing.T) {
	tests := []struct {
		name    string
		stdout  string
		wantErr bool
		check   func(t *testing.T, r *Result)
	}{
		{
			name:   "full result",
			stdout: `{"success":true,"data":{"open_ports":[80]},"output":"ok"}`,
			check: func(t *testing.T, r *Result) {
				if !r.Success {
					t.Fatal("expected success")
				}
				ports := r.Data["open_ports"].([]interface{})
				if len(ports) != 1 {
					t.Fatalf("expected 1 open port, got %d", len(ports))
				}
			},
		},
		{
			name:    "empty output",
			stdout:  "",
			wantErr: true,
		},
		{
			name:    "invalid json",
			stdout:  "not json",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := parseResult([]byte(tt.stdout))
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.check != nil {
				tt.check(t, r)
			}
		})
	}
}

func TestReportParsing(t *testing.T) {
	stdout := `{"title":"Test Report","format":"json","content":"eyJrZXkiOiJ2YWx1ZSJ9"}`
	r, err := parseReport([]byte(stdout))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Title != "Test Report" {
		t.Fatalf("title mismatch: got %q", r.Title)
	}
	if string(r.Content) != `{"key":"value"}` {
		t.Fatalf("content mismatch: got %q", string(r.Content))
	}
}
