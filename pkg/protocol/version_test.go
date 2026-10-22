package protocol

import "testing"

// withVersionTable swaps the protocol history for the duration of a test and
// restores it afterwards. The table is the only source of truth for what the
// server may serve, so the negotiation rules are tested against it.
func withVersionTable(t *testing.T, table []VersionInfo) {
	t.Helper()
	original := versionTable
	versionTable = table
	t.Cleanup(func() { versionTable = original })
}

func TestNegotiateCurrentVersion(t *testing.T) {
	d := Negotiate(CurrentProtocolVersion, 0)
	if !d.Serve {
		t.Fatal("the current protocol version must be served")
	}
	if d.Level != SupportCurrent {
		t.Fatalf("level = %q, want %q", d.Level, SupportCurrent)
	}
	if d.NegotiatedVersion != CurrentProtocolVersion {
		t.Fatalf("negotiated version = %d, want %d", d.NegotiatedVersion, CurrentProtocolVersion)
	}
	if d.Reason != "" {
		t.Fatalf("a current agent needs no explanation, got %q", d.Reason)
	}
}

func TestNegotiateTooNewIsRejected(t *testing.T) {
	d := Negotiate(CurrentProtocolVersion+1, 2)
	if d.Serve {
		t.Fatal("an agent newer than the server must not be served")
	}
	if d.Level != SupportTooNew {
		t.Fatalf("level = %q, want %q", d.Level, SupportTooNew)
	}
	if d.Reason == "" {
		t.Fatal("a rejection must explain itself")
	}
}

func TestNegotiateTooOldIsRejected(t *testing.T) {
	// v1 changed the envelope incompatibly: no window may serve it.
	d := Negotiate(1, 5)
	if d.Serve {
		t.Fatal("an incompatible older version must not be served")
	}
	if d.Level != SupportTooOld {
		t.Fatalf("level = %q, want %q", d.Level, SupportTooOld)
	}
}

func TestNegotiateMissingVersionIsRejected(t *testing.T) {
	// An agent that does not announce a version predates the v2 handshake.
	d := Negotiate(0, 1)
	if d.Serve {
		t.Fatal("an agent without a protocol version must not be served")
	}
	if d.Level != SupportTooOld {
		t.Fatalf("level = %q, want %q", d.Level, SupportTooOld)
	}
}

func TestNegotiateCompatibilityWindow(t *testing.T) {
	// A synthetic history: v1 is wire-compatible with the current version
	// (additive fields only), so the window can serve it.
	withVersionTable(t, []VersionInfo{
		{Version: CurrentProtocolVersion, ServerCompatible: true, Since: "2.0"},
		{Version: 1, Deprecated: true, ServerCompatible: true, Since: "1.0"},
	})

	t.Run("inside the window the old agent is served but flagged", func(t *testing.T) {
		d := Negotiate(1, 1)
		if !d.Serve {
			t.Fatal("a compatible older version inside the window must be served")
		}
		if d.Level != SupportDeprecated {
			t.Fatalf("level = %q, want %q", d.Level, SupportDeprecated)
		}
		if d.NegotiatedVersion != 1 {
			t.Fatalf("negotiated version = %d, want 1", d.NegotiatedVersion)
		}
		if d.Reason == "" {
			t.Fatal("a deprecated agent must be told to rebuild")
		}
	})

	t.Run("outside the window the old agent is refused", func(t *testing.T) {
		d := Negotiate(1, 0)
		if d.Serve {
			t.Fatal("a closed window must refuse the old agent")
		}
		if d.Level != SupportTooOld {
			t.Fatalf("level = %q, want %q", d.Level, SupportTooOld)
		}
	})

	t.Run("the window never serves an incompatible version", func(t *testing.T) {
		withVersionTable(t, []VersionInfo{
			{Version: CurrentProtocolVersion, ServerCompatible: true, Since: "2.0"},
			{Version: 1, Deprecated: true, ServerCompatible: false, Since: "1.0"},
		})
		d := Negotiate(1, 10)
		if d.Serve {
			t.Fatal("the window must not serve an incompatible version")
		}
		if d.Level != SupportTooOld {
			t.Fatalf("level = %q, want %q", d.Level, SupportTooOld)
		}
	})

	t.Run("a negative window is treated as closed", func(t *testing.T) {
		d := Negotiate(1, -3)
		if d.Serve {
			t.Fatal("a negative window must not serve anything")
		}
	})
}

func TestIsOutdated(t *testing.T) {
	cases := map[uint]bool{
		CurrentProtocolVersion: false,
		1:                      true, // deprecated / incompatible
		0:                      true, // no version announced
		99:                     true, // unknown
	}
	for version, want := range cases {
		if got := IsOutdated(version); got != want {
			t.Errorf("IsOutdated(%d) = %v, want %v", version, got, want)
		}
	}
}

func TestVersionsDoesNotExposeTheTable(t *testing.T) {
	versions := Versions()
	if len(versions) == 0 {
		t.Fatal("the version history must not be empty")
	}
	versions[0].Deprecated = true
	if Versions()[0].Deprecated {
		t.Fatal("Versions must return a copy, not the table itself")
	}
}
