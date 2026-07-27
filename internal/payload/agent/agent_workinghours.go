//go:build linux || windows || darwin
// +build linux windows darwin

package main

import "time"

// isWithinWorkingHours checks if the current time is within the configured working hours window.
func isWithinWorkingHours() bool {
	now := time.Now()
	loc := loadWorkingTZ()
	local := now.In(loc)

	startTime, err := time.ParseInLocation("15:04", workingStart, loc)
	if err != nil {
		return true // malformed start, allow activity
	}
	endTime, err := time.ParseInLocation("15:04", workingEnd, loc)
	if err != nil {
		return true // malformed end, allow activity
	}

	currentMinutes := local.Hour()*60 + local.Minute()
	startMinutes := startTime.Hour()*60 + startTime.Minute()
	endMinutes := endTime.Hour()*60 + endTime.Minute()

	if startMinutes <= endMinutes {
		return currentMinutes >= startMinutes && currentMinutes < endMinutes
	}
	// Overnight window (e.g. 22:00-06:00)
	return currentMinutes >= startMinutes || currentMinutes < endMinutes
}

// timeUntilNextWindow returns how long to sleep until the next working window opens.
func timeUntilNextWindow() time.Duration {
	now := time.Now()
	loc := loadWorkingTZ()
	local := now.In(loc)

	startTime, err := time.ParseInLocation("15:04", workingStart, loc)
	if err != nil {
		return 5 * time.Minute // fallback
	}
	endTime, err := time.ParseInLocation("15:04", workingEnd, loc)
	if err != nil {
		return 5 * time.Minute
	}
	_ = endTime

	startMinutes := startTime.Hour()*60 + startTime.Minute()
	currentMinutes := local.Hour()*60 + local.Minute()

	// Calculate how many minutes until start
	var minutesUntilStart int
	if startMinutes > currentMinutes {
		minutesUntilStart = startMinutes - currentMinutes
	} else {
		minutesUntilStart = (24*60 - currentMinutes) + startMinutes
	}

	return time.Duration(minutesUntilStart) * time.Minute
}

// loadWorkingTZ loads the configured timezone or returns UTC.
func loadWorkingTZ() *time.Location {
	if workingTZ == "" {
		return time.UTC
	}
	loc, err := time.LoadLocation(workingTZ)
	if err != nil {
		return time.UTC
	}
	return loc
}
