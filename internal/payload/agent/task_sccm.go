//go:build linux || windows || darwin

package main

func handleSccmRecon(task Task, res *TaskResult) {
	_ = task
	out, err := sccmRecon()
	if err != nil {
		res.Error = err.Error()
		return
	}
	res.Output = out
}

func handleEntraPRT(task Task, res *TaskResult) {
	_ = task
	out, err := entraPRTRecon()
	if err != nil {
		res.Error = err.Error()
		return
	}
	res.Output = out
}
