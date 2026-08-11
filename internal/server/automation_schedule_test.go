package server

import (
	"testing"
	"time"
)

func TestNextScheduleTime(t *testing.T) {
	base := time.Date(2026, 8, 11, 10, 0, 0, 0, time.UTC)

	cases := []struct {
		name     string
		schedule string
		from     time.Time
		want     time.Time
	}{
		{"every 5 minutes", "every 5 minutes", base, base.Add(5 * time.Minute)},
		{"every 2 hours", "every 2 hours", base, base.Add(2 * time.Hour)},
		{"hourly", "hourly", base, base.Add(time.Hour)},
		{"daily later today", "daily 14:30", base, time.Date(2026, 8, 11, 14, 30, 0, 0, time.UTC)},
		{"daily next day", "daily 09:00", base, time.Date(2026, 8, 12, 9, 0, 0, 0, time.UTC)},
		{"weekly", "weekly wed 10:00", time.Date(2026, 8, 9, 10, 0, 0, 0, time.UTC), time.Date(2026, 8, 12, 10, 0, 0, 0, time.UTC)},
		{"cron hourly", "0 * * * *", base, time.Date(2026, 8, 11, 11, 0, 0, 0, time.UTC)},
		{"cron daily", "30 6 * * *", base, time.Date(2026, 8, 12, 6, 30, 0, 0, time.UTC)},
		{"cron step", "*/15 * * * *", time.Date(2026, 8, 11, 10, 7, 0, 0, time.UTC), time.Date(2026, 8, 11, 10, 15, 0, 0, time.UTC)},
		{"cron range list", "0 9,18 * * mon-fri", time.Date(2026, 8, 11, 19, 0, 0, 0, time.UTC), time.Date(2026, 8, 12, 9, 0, 0, 0, time.UTC)},
	}
	for _, tc := range cases {
		got, err := nextScheduleTime(tc.schedule, tc.from)
		if err != nil {
			t.Errorf("%s: unexpected error: %v", tc.name, err)
			continue
		}
		if !got.Equal(tc.want) {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestNextScheduleTimeErrors(t *testing.T) {
	base := time.Date(2026, 8, 11, 10, 0, 0, 0, time.UTC)
	bad := []string{"", "every", "every 0 minutes", "every 5 parsecs", "daily", "daily 25:00", "weekly monday", "weekly xyz 10:00", "not a schedule at all", "61 * * * *"}
	for _, s := range bad {
		if _, err := nextScheduleTime(s, base); err == nil {
			t.Errorf("expected error for %q", s)
		}
	}
}

func TestNextScheduleTimeRunsAfterNow(t *testing.T) {
	from := time.Date(2026, 8, 11, 10, 0, 0, 0, time.UTC)
	for _, s := range []string{"every 5 minutes", "hourly", "daily 10:00", "weekly fri 09:00", "0 0 * * *"} {
		next, err := nextScheduleTime(s, from)
		if err != nil {
			t.Fatalf("unexpected error for %q: %v", s, err)
		}
		if !next.After(from) {
			t.Errorf("%q: next %v is not after from %v", s, next, from)
		}
	}
}
