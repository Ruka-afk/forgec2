//go:build !windows

package main

func DetectEDR() *EDRInfo {
	return &EDRInfo{Detected: false}
}

// ApplyStrategy is a no-op off windows: the evasion flags it would populate
// are declared in windows-tagged files only, and non-windows builds carry
// neither AMSI/ETW patches nor PEB-based DLL blocking.
func ApplyStrategy(EvasionStrategy) {}

func (e *EDRInfo) GetStrategy() EvasionStrategy {
	return EvasionStrategy{Name: "none"}
}

// applyAdaptiveEvasion keeps the main()-side call site uniform across
// platforms. Off windows there is no strategy to apply and no anti-debug
// sweep to run; the EDR scan itself is the harmless stub above.
func applyAdaptiveEvasion() {
	edrInfo := DetectEDR()
	ApplyStrategy(edrInfo.GetStrategy())
}
