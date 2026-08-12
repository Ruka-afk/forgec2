//go:build windows
// +build windows

package plugin

import (
	"errors"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

// pluginProcessMemoryMB caps a plugin process's working memory. Jumps above
// this limit fail the allocation and terminate the process.
const pluginProcessMemoryMB = 512

// windowsJob is a Windows Job Object configured with KILL_ON_JOB_CLOSE and a
// per-process memory cap. Closing the handle (release) terminates every
// surviving process in the job, which prevents plugin children from escaping
// timeouts or turning into orphans.
type windowsJob struct {
	handle windows.Handle
}

func newWindowsJob(memoryLimitMB uint32) (*windowsJob, error) {
	handle, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, err
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{
		BasicLimitInformation: windows.JOBOBJECT_BASIC_LIMIT_INFORMATION{
			LimitFlags: windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE |
				windows.JOB_OBJECT_LIMIT_PROCESS_MEMORY,
		},
		ProcessMemoryLimit: uintptr(memoryLimitMB) * 1024 * 1024,
	}
	if _, err := windows.SetInformationJobObject(
		handle,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)),
		uint32(unsafe.Sizeof(info)),
	); err != nil {
		windows.CloseHandle(handle)
		return nil, err
	}
	return &windowsJob{handle: handle}, nil
}

func (j *windowsJob) assign(process windows.Handle) error {
	return windows.AssignProcessToJobObject(j.handle, process)
}

// release closes the job handle; with KILL_ON_JOB_CLOSE this terminates any
// process still running inside the job (orphaned children included).
func (j *windowsJob) release() {
	windows.CloseHandle(j.handle)
}

// processGuard attaches a started plugin process to a job object so the whole
// process tree dies with the plugin run.
type processGuard struct {
	job *windowsJob
}

// attachProcessGuard binds the process to a fresh job object. Non-fatal on
// failure: the plugin still runs, just without tree-kill guarantees.
func attachProcessGuard(p *os.Process) (*processGuard, error) {
	if p == nil || p.Pid <= 0 {
		return nil, errors.New("no process to guard")
	}
	h, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(p.Pid))
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(h)
	job, err := newWindowsJob(pluginProcessMemoryMB)
	if err != nil {
		return nil, err
	}
	if err := job.assign(h); err != nil {
		job.release()
		return nil, err
	}
	return &processGuard{job: job}, nil
}

// release kills the process tree. Safe to call multiple times.
func (g *processGuard) release() {
	if g != nil && g.job != nil {
		g.job.release()
		g.job = nil
	}
}
