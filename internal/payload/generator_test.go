package payload

import "testing"

func TestResolveBuildArch(t *testing.T) {
	cases := []struct {
		goos, arch   string
		want         string
		wantErr      bool
	}{
		{"windows", "amd64", "amd64", false},
		{"windows", "x86_64", "amd64", false},
		{"windows", "arm64", "arm64", false},
		{"windows", "386", "", true},   // 32-bit Windows not supported
		{"windows", "x86", "", true},
		{"linux", "arm", "arm", false},
		{"linux", "386", "", true},
		{"darwin", "arm64", "arm64", false},
		{"darwin", "386", "", true},
		{"windows", "riscv64", "", true},
	}
	for _, c := range cases {
		got, err := resolveBuildArch(c.goos, c.arch)
		if c.wantErr {
			if err == nil {
				t.Errorf("resolveBuildArch(%s,%s): expected error, got %q", c.goos, c.arch, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("resolveBuildArch(%s,%s): unexpected error %v", c.goos, c.arch, err)
			continue
		}
		if got != c.want {
			t.Errorf("resolveBuildArch(%s,%s) = %q, want %q", c.goos, c.arch, got, c.want)
		}
	}
}

func TestClampBeaconTiming(t *testing.T) {
	cases := []struct {
		inI, inJ       int
		wantI, wantJ   int
	}{
		{10, 20, 10, 20},
		{10, 200, 10, 100}, // jitter capped at 100
		{10, -5, 10, 0},    // negative jitter -> 0
		{0, 10, 1, 10},     // interval < 1 -> 1
		{-3, 10, 1, 10},    // negative interval -> 1
		{100000, 50, 86400, 50}, // interval capped at 86400
	}
	for _, c := range cases {
		gotI, gotJ := clampBeaconTiming(c.inI, c.inJ)
		if gotI != c.wantI || gotJ != c.wantJ {
			t.Errorf("clampBeaconTiming(%d,%d) = (%d,%d), want (%d,%d)",
				c.inI, c.inJ, gotI, gotJ, c.wantI, c.wantJ)
		}
	}
}

// TestSafeBuildFileNameStripsDirectoryComponents guards A1: a user-supplied
// output filename containing ".." segments or an absolute path must never be
// able to escape the output directory when joined with filepath.Join.
func TestSafeBuildFileNameStripsDirectoryComponents(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"agent.exe", "agent.exe"},
		{"../../evil.exe", "evil.exe"},
		{"sub/dir/agent.exe", "agent.exe"},
		{"C:\\windows\\system32\\agent.exe", "agent.exe"},
		{"/abs/path/agent.exe", "agent.exe"},
		{"..\\..\\evil.exe", "evil.exe"},
		{"./agent.exe", "agent.exe"},
	}
	for _, c := range cases {
		if got := safeBuildFileName(c.in); got != c.want {
			t.Errorf("safeBuildFileName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
