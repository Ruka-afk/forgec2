package main

import (
	"testing"
)

// TestRotatedBeaconURI verifies round-robin rotation across the v2 URI list.
func TestRotatedBeaconURI(t *testing.T) {
	beaconURICounter.Store(0)
	list := "/a,/b,/c"
	if got := rotatedBeaconURI("/a", list); got != "/a" {
		t.Fatalf("first = %s, want /a", got)
	}
	if got := rotatedBeaconURI("/a", list); got != "/b" {
		t.Fatalf("second = %s, want /b", got)
	}
	if got := rotatedBeaconURI("/a", list); got != "/c" {
		t.Fatalf("third = %s, want /c", got)
	}
	if got := rotatedBeaconURI("/a", list); got != "/a" {
		t.Fatalf("wrap = %s, want /a", got)
	}
	// Single-entry and empty lists fall back to the base URI.
	if got := rotatedBeaconURI("/only", "/only"); got != "/only" {
		t.Fatalf("single = %s, want /only", got)
	}
	if got := rotatedBeaconURI("/base", ""); got != "/base" {
		t.Fatalf("empty = %s, want /base", got)
	}
	if got := rotatedBeaconURI("/base", " , "); got != "/base" {
		t.Fatalf("blank = %s, want /base", got)
	}
}

// TestGetActiveBeaconURIRotation verifies the override still wins and the
// rotation list is honored otherwise.
func TestGetActiveBeaconURIRotation(t *testing.T) {
	prevList, prevBase := BeaconURIsStr, BeaconURI
	prevOverride := configOverrides.beaconURI
	defer func() {
		BeaconURIsStr, BeaconURI = prevList, prevBase
		configOverrides.beaconURI = prevOverride
	}()

	BeaconURI = "/primary"
	BeaconURIsStr = "/u1,/u2"
	configOverrides.beaconURI = ""
	beaconURICounter.Store(0)
	if got := getActiveBeaconURIFromConfig(); got != "/u1" {
		t.Fatalf("rot0 = %s, want /u1", got)
	}
	if got := getActiveBeaconURIFromConfig(); got != "/u2" {
		t.Fatalf("rot1 = %s, want /u2", got)
	}
	// Explicit override (profile_rotate / config_push) always wins.
	configOverrides.beaconURI = "/override"
	if got := getActiveBeaconURIFromConfig(); got != "/override" {
		t.Fatalf("override = %s, want /override", got)
	}
}
