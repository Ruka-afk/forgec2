//go:build windows

package main

import (
	"testing"
	"time"
)

func TestParseWeChatRangeDefaultsEndToNow(t *testing.T) {
	before := time.Now().UTC()
	_, endTime := parseWeChatRange(wechatFilter{})
	after := time.Now().UTC()

	if endTime.IsZero() {
		t.Fatal("empty end time must default to now")
	}
	if endTime.Before(before) || endTime.After(after) {
		t.Fatalf("end time %v is outside [%v, %v]", endTime, before, after)
	}
}

func TestParseWeChatRangeKeepsExplicitBounds(t *testing.T) {
	start := "2026-09-01T00:00:00Z"
	end := "2026-09-13T23:59:59Z"
	startTime, endTime := parseWeChatRange(wechatFilter{StartTime: start, EndTime: end})

	if got := startTime.UTC().Format(time.RFC3339); got != start {
		t.Fatalf("start=%q", got)
	}
	if got := endTime.UTC().Format(time.RFC3339); got != end {
		t.Fatalf("end=%q", got)
	}
}
