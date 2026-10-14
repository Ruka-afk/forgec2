package server

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/plugin"
	"github.com/gin-gonic/gin"
)

const pluginInstallName = "reconprobe"

// pluginManifestJSON builds the JSON body the install endpoint expects.
func pluginManifestJSON(t *testing.T, entry string) []byte {
	t.Helper()
	manifest := plugin.Manifest{
		Name:        pluginInstallName,
		Version:     "1.0.0",
		Type:        "command",
		Description: "probe plugin",
		Author:      "tester",
		Category:    "recon",
		Entry:       entry,
		Interpreter: "bash",
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	return data
}

// newPluginTestServer builds a server with a real plugin manager whose root is a
// per-test temp directory, so install paths touch the filesystem for real.
func newPluginTestServer(t *testing.T) *Server {
	t.Helper()
	s := mustTenantServer(t)
	manager := plugin.NewManager(s.db)
	// LoadFromDisk is the only public way to repoint the manager's root; an
	// empty temp dir is a no-op scan.
	if err := manager.LoadFromDisk(t.TempDir()); err != nil {
		t.Fatalf("set plugin root: %v", err)
	}
	s.pluginManager = manager
	return s
}

// pluginDirFor resolves the directory the install handler will write into.
// Register may remap the record's directory once it succeeds, so this must be
// captured before the request rather than after.
func pluginDirFor(s *Server) string {
	return filepath.Join(filepath.Dir(s.pluginManager.PluginDir(pluginInstallName)), pluginInstallName)
}

// pluginInstallCtx posts a multipart install with the given manifest bytes and
// optional script.
func pluginInstallCtx(t *testing.T, s *Server, manifestBytes []byte, script string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, err := mw.CreateFormFile("manifest", "manifest.json")
	if err != nil {
		t.Fatalf("form file: %v", err)
	}
	if _, err := fw.Write(manifestBytes); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	if script != "" {
		sw, err := mw.CreateFormFile("script", "probe.sh")
		if err != nil {
			t.Fatalf("script file: %v", err)
		}
		if _, err := sw.Write([]byte(script)); err != nil {
			t.Fatalf("write script: %v", err)
		}
	}
	mw.Close()

	c, w := tenantScopedAdminContext(s, t, "plug-op", 1)
	c.Request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body.String()))
	c.Request.Header.Set("Content-Type", mw.FormDataContentType())
	return c, w
}

func installAuditCount(t *testing.T, s *Server) int64 {
	t.Helper()
	var n int64
	if err := s.db.Model(&db.AuditLog{}).Where("action = ?", "plugin_install").Count(&n).Error; err != nil {
		t.Fatalf("count audits: %v", err)
	}
	return n
}

// ── Install atomicity ───────────────────────────────────────────────────────

// TestPluginInstallRejectionLeavesNothingOnDisk is the core regression: the
// endpoint wrote manifest.yaml and the script before Register() ran, so a
// package rejected by the trust layer stayed on disk and could later be picked
// up by tryRegisterPluginFromDisk and activated.
func TestPluginInstallRejectionLeavesNothingOnDisk(t *testing.T) {
	s := newPluginTestServer(t)
	pluginDir := pluginDirFor(s)
	// require_signed with no trusted keys makes an unsigned package fatal.
	s.pluginManager.SetTrust(nil, true)

	c, w := pluginInstallCtx(t, s, pluginManifestJSON(t, "probe.sh"), "#!/bin/sh\necho hi\n")
	s.handlePluginInstall(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (%s)", w.Code, w.Body.String())
	}

	if entries, err := os.ReadDir(pluginDir); err == nil && len(entries) > 0 {
		t.Fatalf("a rejected plugin package left %d files behind in %s", len(entries), pluginDir)
	}

	var count int64
	if err := s.db.Model(&db.Plugin{}).Where("name = ?", pluginInstallName).Count(&count).Error; err != nil {
		t.Fatalf("count plugins: %v", err)
	}
	if count != 0 {
		t.Fatalf("%d plugin rows were created by a rejected install", count)
	}
	if n := installAuditCount(t, s); n != 0 {
		t.Fatalf("%d install audit rows were written by a rejected install", n)
	}
}

// TestPluginInstallRejectionKeepsExistingPlugin guards the rollback itself: a
// rejected reinstall must restore the package that was already there instead of
// destroying it.
func TestPluginInstallRejectionKeepsExistingPlugin(t *testing.T) {
	s := newPluginTestServer(t)
	pluginDir := pluginDirFor(s)
	if err := os.MkdirAll(pluginDir, 0750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	keepManifest := pluginManifestJSON(t, "probe.sh")
	keepScript := []byte("#!/bin/sh\necho original\n")
	if err := os.WriteFile(filepath.Join(pluginDir, "manifest.yaml"), keepManifest, 0600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pluginDir, "probe.sh"), keepScript, 0600); err != nil {
		t.Fatalf("write script: %v", err)
	}

	s.pluginManager.SetTrust(nil, true)
	c, w := pluginInstallCtx(t, s, pluginManifestJSON(t, "probe.sh"), "#!/bin/sh\necho attacker\n")
	s.handlePluginInstall(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (%s)", w.Code, w.Body.String())
	}

	gotManifest, err := os.ReadFile(filepath.Join(pluginDir, "manifest.yaml"))
	if err != nil {
		t.Fatalf("original manifest was not restored: %v", err)
	}
	if !bytes.Equal(gotManifest, keepManifest) {
		t.Fatal("the rejected reinstall left a different manifest on disk")
	}
	gotScript, err := os.ReadFile(filepath.Join(pluginDir, "probe.sh"))
	if err != nil {
		t.Fatalf("original script was not restored: %v", err)
	}
	if !bytes.Equal(gotScript, keepScript) {
		t.Fatalf("the rejected reinstall overwrote the stored script: %q", gotScript)
	}
}

// TestPluginInstallSuccessWritesAndEnables pins the happy path: the package is
// written, the record is enabled, and the install is audited once.
func TestPluginInstallSuccessWritesAndEnables(t *testing.T) {
	s := newPluginTestServer(t)
	pluginDir := pluginDirFor(s)

	c, w := pluginInstallCtx(t, s, pluginManifestJSON(t, "probe.sh"), "#!/bin/sh\necho hi\n")
	s.handlePluginInstall(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", w.Code, w.Body.String())
	}

	manifest, err := os.ReadFile(filepath.Join(pluginDir, "manifest.yaml"))
	if err != nil {
		t.Fatalf("manifest not written: %v", err)
	}
	if !strings.Contains(string(manifest), pluginInstallName) {
		t.Fatalf("manifest = %s", manifest)
	}
	script, err := os.ReadFile(filepath.Join(pluginDir, "probe.sh"))
	if err != nil {
		t.Fatalf("script not written: %v", err)
	}
	if !strings.Contains(string(script), "echo hi") {
		t.Fatalf("script = %q", script)
	}
	if n := installAuditCount(t, s); n != 1 {
		t.Fatalf("plugin_install audit rows = %d, want 1", n)
	}
}

// TestPluginInstallAcceptsDottedEntryNames pins the entry-point fix: the check
// used to reject any dot, so no ordinary script (probe.sh, main.py, plugin.js)
// could ever be installed.
func TestPluginInstallAcceptsDottedEntryNames(t *testing.T) {
	for _, entry := range []string{"probe.sh", "main.py", "plugin.js", "run", "a.b.sh"} {
		t.Run(entry, func(t *testing.T) {
			s := newPluginTestServer(t)
			pluginDir := pluginDirFor(s)
			c, w := pluginInstallCtx(t, s, pluginManifestJSON(t, entry), "#!/bin/sh\ntrue\n")
			s.handlePluginInstall(c)
			if w.Code != http.StatusOK {
				t.Fatalf("entry %q: status = %d (%s)", entry, w.Code, w.Body.String())
			}
			if _, err := os.Stat(filepath.Join(pluginDir, entry)); err != nil {
				t.Fatalf("entry %q was not written: %v", entry, err)
			}
		})
	}
}

// TestPluginInstallRejectsBadInput pins the request validation that must happen
// before anything is written to disk.
func TestPluginInstallRejectsBadInput(t *testing.T) {
	cases := []struct {
		name string
		body string
		want int
	}{
		{"missing manifest", "", http.StatusNotFound},
		{"manifest is not json", "not json at all", http.StatusBadRequest},
		{"path traversal in name", `{"name":"../../evil","entry":"x.sh"}`, http.StatusBadRequest},
		{"path separator in name", `{"name":"a/b","entry":"x.sh"}`, http.StatusBadRequest},
		{"empty name", `{"name":"","entry":"x.sh"}`, http.StatusBadRequest},
		{"dotdot in name", `{"name":"a..b","entry":"x.sh"}`, http.StatusBadRequest},
		{"path traversal in entry", `{"name":"okplugin","entry":"../../evil.sh"}`, http.StatusBadRequest},
		{"path separator in entry", `{"name":"okplugin","entry":"a/b.sh"}`, http.StatusBadRequest},
		{"dotdot in entry", `{"name":"okplugin","entry":"..evil.sh"}`, http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := newPluginTestServer(t)
			var c *gin.Context
			var w *httptest.ResponseRecorder
			if tc.body == "" {
				var body bytes.Buffer
				mw := multipart.NewWriter(&body)
				mw.Close()
				c, w = tenantScopedAdminContext(s, t, "plug-op", 1)
				c.Request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body.String()))
				c.Request.Header.Set("Content-Type", mw.FormDataContentType())
			} else {
				c, w = pluginInstallCtx(t, s, []byte(tc.body), "script")
			}
			s.handlePluginInstall(c)
			if w.Code != tc.want {
				t.Fatalf("status = %d, want %d (%s)", w.Code, tc.want, w.Body.String())
			}
		})
	}
}

// ── Staging rollback unit tests ─────────────────────────────────────────────

// TestPluginInstallStagingCleanup pins that cleanup rolls back only what this
// request wrote: created files are removed, overwritten files are restored to
// their previous bytes, and a pre-existing directory is never removed.
func TestPluginInstallStagingCleanup(t *testing.T) {
	root := t.TempDir()

	t.Run("created files are removed and the new dir goes with them", func(t *testing.T) {
		dir := filepath.Join(root, "fresh")
		if err := os.MkdirAll(dir, 0750); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		st := &pluginInstallStaging{pluginDir: dir, dirCreated: true}
		for _, name := range []string{"manifest.yaml", "probe.sh"} {
			if err := st.writeFile(filepath.Join(dir, name), []byte("staged"), 0600); err != nil {
				t.Fatalf("stage %s: %v", name, err)
			}
		}
		st.cleanup()

		for _, name := range []string{"manifest.yaml", "probe.sh"} {
			if err := os.Remove(filepath.Join(dir, name)); !os.IsNotExist(err) {
				t.Fatalf("%s survived cleanup", name)
			}
		}
		if _, err := os.Stat(dir); !os.IsNotExist(err) {
			t.Fatal("the directory this request created was not removed")
		}
	})

	t.Run("overwritten files are restored byte for byte", func(t *testing.T) {
		dir := filepath.Join(root, "existing")
		if err := os.MkdirAll(dir, 0750); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		original := []byte("the package that was already working")
		manifest := filepath.Join(dir, "manifest.yaml")
		if err := os.WriteFile(manifest, original, 0600); err != nil {
			t.Fatalf("seed: %v", err)
		}
		st := &pluginInstallStaging{pluginDir: dir, dirCreated: false}
		if err := st.writeFile(manifest, []byte("the rejected replacement"), 0600); err != nil {
			t.Fatalf("stage: %v", err)
		}
		st.cleanup()

		restored, err := os.ReadFile(manifest)
		if err != nil {
			t.Fatalf("the overwritten manifest was not restored: %v", err)
		}
		if !bytes.Equal(restored, original) {
			t.Fatalf("restored manifest = %q, want %q", restored, original)
		}
		if _, err := os.Stat(dir); err != nil {
			t.Fatal("a pre-existing plugin directory was removed")
		}
	})

	t.Run("unrelated files in the same dir survive", func(t *testing.T) {
		dir := filepath.Join(root, "mixed")
		if err := os.MkdirAll(dir, 0750); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		unrelated := filepath.Join(dir, "previous.sh")
		if err := os.WriteFile(unrelated, []byte("not mine"), 0600); err != nil {
			t.Fatalf("seed: %v", err)
		}
		st := &pluginInstallStaging{pluginDir: dir, dirCreated: false}
		if err := st.writeFile(filepath.Join(dir, "manifest.yaml"), []byte("staged"), 0600); err != nil {
			t.Fatalf("stage: %v", err)
		}
		st.cleanup()

		kept, err := os.ReadFile(unrelated)
		if err != nil {
			t.Fatalf("a file this request never wrote was removed: %v", err)
		}
		if string(kept) != "not mine" {
			t.Fatalf("unrelated file changed: %q", kept)
		}
	})

	t.Run("a created dir holding unrelated files is kept", func(t *testing.T) {
		dir := filepath.Join(root, "fresh-extra")
		if err := os.MkdirAll(dir, 0750); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		extra := filepath.Join(dir, "extra.txt")
		if err := os.WriteFile(extra, []byte("x"), 0600); err != nil {
			t.Fatalf("seed: %v", err)
		}
		st := &pluginInstallStaging{pluginDir: dir, dirCreated: true}
		if err := st.writeFile(filepath.Join(dir, "manifest.yaml"), []byte("staged"), 0600); err != nil {
			t.Fatalf("stage: %v", err)
		}
		st.cleanup()

		if _, err := os.Stat(dir); err != nil {
			t.Fatal("a non-empty directory was removed")
		}
		if _, err := os.Stat(extra); err != nil {
			t.Fatal("a file this request never wrote was removed")
		}
	})

	t.Run("cleanup is idempotent and tolerates missing files", func(t *testing.T) {
		dir := filepath.Join(root, "vanished")
		if err := os.MkdirAll(dir, 0750); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		st := &pluginInstallStaging{pluginDir: dir, dirCreated: true}
		st.entries = append(st.entries, stagedFile{path: filepath.Join(dir, "never-existed")})
		st.cleanup()
		st.cleanup() // must not panic or hang on the second pass
	})
}

// ── Plugin record resolution ────────────────────────────────────────────────

// TestResolvePluginRecord pins the id-or-name lookup and the honest miss.
func TestResolvePluginRecord(t *testing.T) {
	s := newPluginTestServer(t)
	seedPlugin(t, s, "reconscan", "reconscan")

	var seeded db.Plugin
	if err := s.db.Where("name = ?", "reconscan").First(&seeded).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}

	byID, err := s.resolvePluginRecord(strconv.Itoa(int(seeded.ID)))
	if err != nil {
		t.Fatalf("resolve by id: %v", err)
	}
	if byID.Name != "reconscan" {
		t.Fatalf("byID = %+v", byID)
	}
	byName, err := s.resolvePluginRecord("reconscan")
	if err != nil {
		t.Fatalf("resolve by name: %v", err)
	}
	if byName.ID != seeded.ID {
		t.Fatalf("byName = %+v", byName)
	}
	if _, err := s.resolvePluginRecord("does-not-exist"); err == nil {
		t.Fatal("an unknown plugin id resolved")
	}
}

func seedPlugin(t *testing.T, s *Server, name, description string) {
	t.Helper()
	plug := db.Plugin{Name: name, Version: "1.0.0", Description: description, Enabled: true}
	if err := s.db.Create(&plug).Error; err != nil {
		t.Fatalf("seed plugin: %v", err)
	}
}

// TestPluginInstallManifestIsYAMLParsable proves the JSON manifest the endpoint
// stores is readable back through the YAML loader.
func TestPluginInstallManifestIsYAMLParsable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "manifest.yaml")
	if err := os.WriteFile(path, pluginManifestJSON(t, "probe.sh"), 0600); err != nil {
		t.Fatalf("write: %v", err)
	}
	manifest, err := plugin.LoadManifest(path)
	if err != nil {
		t.Fatalf("the JSON manifest is not readable back as YAML: %v", err)
	}
	if manifest.Name != pluginInstallName || manifest.Entry != "probe.sh" {
		t.Fatalf("round-tripped manifest = %+v", manifest)
	}
}
