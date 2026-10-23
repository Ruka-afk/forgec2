package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
)

// The settings save handlers mutate s.cfg (scalars, strings and maps) while
// the config reloader swaps the same struct under s.configMu and the beacon
// paths read it under RLock. Until this batch, the save handlers took no
// lock at all: a save racing a beacon-path reader (or the settings page)
// was a data race on live config state — including the Malleable header
// maps handed to the template by reference. These tests pin the fix.

func TestSaveMalleableProfileRacesBeaconReader(t *testing.T) {
	s := mustTenantServer(t)
	s.configPath = filepath.Join(t.TempDir(), "config.yaml")

	var wg sync.WaitGroup
	stop := make(chan struct{})

	// Beacon-path reader: takes configMu.RLock and copies s.cfg.Malleable
	// (profile.go) on every wrapped response.
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				c, _ := gin.CreateTestContext(httptest.NewRecorder())
				c.Request, _ = http.NewRequest(http.MethodGet, "/beacon", nil)
				s.applyMalleableProfile(c, []byte("body"))
			}
		}()
	}

	for i := 0; i < 60; i++ {
		c, _ := settingsFormCtx(s, t, "race-op", "admin", url.Values{
			"enabled":      {"true"},
			"headers_text": {"X-Test: one\nX-More: two"},
		}, nil)
		s.handleSaveMalleableProfile(c)
	}
	close(stop)
	wg.Wait()
}

func TestSaveServerConfigRacesSettingsPageReader(t *testing.T) {
	s := mustTenantServer(t)
	s.configPath = filepath.Join(t.TempDir(), "config.yaml")

	var wg sync.WaitGroup
	stop := make(chan struct{})
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				c, _ := gin.CreateTestContext(httptest.NewRecorder())
				c.Request, _ = http.NewRequest(http.MethodGet, "/settings", nil)
				c.Set("user", "admin")
				c.Set("user_role", "admin")
				s.handleSettingsPage(c)
			}
		}()
	}
	for i := 0; i < 60; i++ {
		c1, _ := settingsFormCtx(s, t, "race-op", "admin", url.Values{
			"log_level":         {"info"},
			"offline_threshold": {"61"},
		}, nil)
		s.handleSaveServerConfig(c1)
		c2, _ := settingsFormCtx(s, t, "race-op", "admin", url.Values{
			"interval": {"30"},
			"jitter":   {"10"},
		}, nil)
		s.handleSaveAgentConfig(c2)
	}
	close(stop)
	wg.Wait()
}
