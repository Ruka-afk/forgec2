//go:build linux || windows || darwin
// +build linux windows darwin

package main

func handleSessionRecon(task Task, res *TaskResult) {
	_ = task
	out, err := sessionRecon()
	if err != nil {
		res.Error = err.Error()
		return
	}
	res.Output = out
}
