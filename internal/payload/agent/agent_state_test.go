package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteRegistrationMarker(t *testing.T) {
	path := filepath.Join(t.TempDir(), "registered")

	if err := writeRegistrationMarker(path, true); err != nil {
		t.Fatalf("write marker: %v", err)
	}
	if got, err := os.ReadFile(path); err != nil || string(got) != "1" {
		t.Fatalf("marker = %q, %v; want \"1\"", got, err)
	}

	if err := writeRegistrationMarker(path, false); err != nil {
		t.Fatalf("remove marker: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("marker still exists after deregistration: %v", err)
	}

	// A second reregister/restart must not treat an absent marker as an error.
	if err := writeRegistrationMarker(path, false); err != nil {
		t.Fatalf("remove absent marker: %v", err)
	}
}
