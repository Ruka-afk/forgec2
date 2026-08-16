//go:build windows

package main

import (
	"sync"
	"testing"
)

// TestRunBeaconSendSpoofedInline verifies that, with stack spoofing disabled
// (the default), runBeaconSendSpoofed executes fn inline and synchronously.
func TestRunBeaconSendSpoofedInline(t *testing.T) {
	if useStackSpoofing {
		t.Skip("stack spoofing enabled; native-thread path is covered by runtime testing")
	}
	var wg sync.WaitGroup
	wg.Add(1)
	called := false
	runBeaconSendSpoofed(func() {
		called = true
		wg.Done()
	})
	wg.Wait()
	if !called {
		t.Fatal("spoofed beacon send did not invoke fn")
	}
}
