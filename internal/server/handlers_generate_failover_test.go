package server

import (
	"net/http"
	"net/url"
	"strconv"
	"testing"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/payload"
)

func TestMergeC2URLList(t *testing.T) {
	tests := []struct {
		name      string
		primary   string
		requested string
		want      string
		wantErr   bool
	}{
		{
			name:      "keeps operator failovers",
			primary:   "https://primary.example.com:8443",
			requested: "https://primary.example.com:8443,https://backup.example.com:9443",
			want:      "https://primary.example.com:8443,https://backup.example.com:9443",
		},
		{
			name:      "dedupes primary and failovers",
			primary:   "https://primary.example.com:8443",
			requested: "https://primary.example.com:8443,https://backup.example.com:9443,https://backup.example.com:9443",
			want:      "https://primary.example.com:8443,https://backup.example.com:9443",
		},
		{
			name:      "defaults bare host to http",
			primary:   "https://primary.example.com:8443",
			requested: "backup.example.com:9443",
			want:      "https://primary.example.com:8443,http://backup.example.com:9443",
		},
		{
			name:      "trims whitespace",
			primary:   "https://primary.example.com:8443",
			requested: " https://backup.example.com:9443 , tcp://backup2:4444 ",
			want:      "https://primary.example.com:8443,https://backup.example.com:9443,tcp://backup2:4444",
		},
		{
			name:      "rejects unsupported scheme",
			primary:   "https://primary.example.com:8443",
			requested: "javascript://evil.example.com",
			wantErr:   true,
		},
		{
			name:      "rejects missing host",
			primary:   "https://primary.example.com:8443",
			requested: "https:///missing",
			wantErr:   true,
		},
		{
			name:      "rejects userinfo",
			primary:   "https://primary.example.com:8443",
			requested: "https://user:pass@backup.example.com:9443",
			wantErr:   true,
		},
		{
			name:      "rejects query",
			primary:   "https://primary.example.com:8443",
			requested: "https://backup.example.com:9443?x=1",
			wantErr:   true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := mergeC2URLList(tc.primary, tc.requested)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("mergeC2URLList() = %q, want error", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("mergeC2URLList() error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("mergeC2URLList() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestParseBinaryFormPreservesFailover(t *testing.T) {
	s := mustTenantServer(t)
	l := db.Listener{
		Name:    "primary",
		Scheme:  "https",
		Host:    "primary.example.com",
		Port:    8443,
		Enabled: true,
	}
	if err := s.db.Create(&l).Error; err != nil {
		t.Fatalf("create listener: %v", err)
	}

	body := url.Values{
		"listener_id": {strconv.FormatUint(uint64(l.ID), 10)},
		"c2_url":      {"https://primary.example.com:8443,https://backup.example.com:9443"},
		"interval":    {"10"},
	}
	c, w := tenantCtx(http.MethodPost, "/api/generate/exe", "failover-op", body.Encode())
	form, ok := s.parseBinaryForm(c)
	if !ok {
		t.Fatalf("parseBinaryForm rejected valid failover list: %s", w.Body.String())
	}
	want := "https://primary.example.com:8443,https://backup.example.com:9443"
	if form.C2URL != want {
		t.Fatalf("C2URL = %q, want %q", form.C2URL, want)
	}
}

func TestParseBinaryFormRejectsInvalidFailover(t *testing.T) {
	s := mustTenantServer(t)
	l := db.Listener{
		Name:    "primary-invalid",
		Scheme:  "https",
		Host:    "primary.example.com",
		Port:    8443,
		Enabled: true,
	}
	if err := s.db.Create(&l).Error; err != nil {
		t.Fatalf("create listener: %v", err)
	}

	body := url.Values{
		"listener_id": {strconv.FormatUint(uint64(l.ID), 10)},
		"c2_url":      {"https://primary.example.com:8443,javascript://evil.example.com"},
		"interval":    {"10"},
	}
	c, w := tenantCtx(http.MethodPost, "/api/generate/exe", "failover-op", body.Encode())
	if form, ok := s.parseBinaryForm(c); ok || form != nil {
		t.Fatal("parseBinaryForm accepted an invalid failover scheme")
	}
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestBindStagerFormPreservesFailover(t *testing.T) {
	s := mustTenantServer(t)
	l := db.Listener{
		Name:    "stager-primary",
		Scheme:  "https",
		Host:    "primary.example.com",
		Port:    8443,
		Enabled: true,
	}
	if err := s.db.Create(&l).Error; err != nil {
		t.Fatalf("create listener: %v", err)
	}

	body := url.Values{
		"listener_id": {strconv.FormatUint(uint64(l.ID), 10)},
		"c2_url":      {"https://primary.example.com:8443,https://backup.example.com:9443"},
	}
	c, _ := tenantCtx(http.MethodPost, "/api/generate/stager", "failover-op", body.Encode())
	form, err := s.bindStagerForm(c)
	if err != nil {
		t.Fatalf("bindStagerForm rejected valid failover list: %v", err)
	}
	want := "https://primary.example.com:8443,https://backup.example.com:9443"
	if form.C2URL != want {
		t.Fatalf("C2URL = %q, want %q", form.C2URL, want)
	}
	if got := payload.OriginFromC2URL(form.C2URL); got != "https://primary.example.com:8443" {
		t.Fatalf("stage fetch origin = %q, want primary", got)
	}
}
