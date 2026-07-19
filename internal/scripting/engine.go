package scripting

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/console"
	"github.com/dop251/goja_nodejs/require"
)

type ScriptEngine struct {
	vm      *goja.Runtime
	mu      sync.Mutex
	scripts []*LoadedScript
	events  map[string][]goja.Callable
	onEvent func(string, interface{})
}

type LoadedScript struct {
	ID     uint
	Name   string
	Code   string
	Events []string
	Active bool
}

var globalEngine *ScriptEngine

func init() {
	globalEngine = NewScriptEngine()
}

func NewScriptEngine() *ScriptEngine {
	vm := goja.New()
	registry := new(require.Registry)
	registry.Enable(vm)
	console.Enable(vm)

	e := &ScriptEngine{
		vm:      vm,
		scripts: make([]*LoadedScript, 0),
		events:  make(map[string][]goja.Callable),
	}
	e.registerAPI()
	return e
}

func GetEngine() *ScriptEngine {
	return globalEngine
}

func (e *ScriptEngine) registerAPI() {
	vm := e.vm

	vm.Set("on", func(call goja.FunctionCall) goja.Value {
		eventType := call.Argument(0).String()
		cb, ok := goja.AssertFunction(call.Argument(1))
		if !ok {
			slog.Error("script: on() requires a function callback")
			return goja.Undefined()
		}
		e.mu.Lock()
		e.events[eventType] = append(e.events[eventType], cb)
		e.mu.Unlock()
		return goja.Undefined()
	})

	vm.Set("off", func(call goja.FunctionCall) goja.Value {
		eventType := call.Argument(0).String()
		e.mu.Lock()
		delete(e.events, eventType)
		e.mu.Unlock()
		return goja.Undefined()
	})

	vm.Set("sendTask", func(call goja.FunctionCall) goja.Value {
		agentID := call.Argument(0).String()
		taskType := call.Argument(1).String()
		params := call.Argument(2).String()

		if e.onEvent != nil {
			e.onEvent("sendTask", map[string]interface{}{
				"agent_id":  agentID,
				"task_type": taskType,
				"params":    params,
			})
		}
		return goja.Undefined()
	})

	vm.Set("getAgent", func(call goja.FunctionCall) goja.Value {
		agentID := call.Argument(0).String()
		if e.onEvent != nil {
			e.onEvent("getAgent", agentID)
		}
		return goja.Undefined()
	})

	vm.Set("listAgents", func(call goja.FunctionCall) goja.Value {
		if e.onEvent != nil {
			e.onEvent("listAgents", nil)
		}
		return goja.Undefined()
	})

	vm.Set("httpRequest", func(call goja.FunctionCall) goja.Value {
		url := call.Argument(0).String()
		opts := call.Argument(1).ToObject(vm)
		_ = opts
		if e.onEvent != nil {
			e.onEvent("httpRequest", map[string]interface{}{
				"url":     url,
				"options": opts,
			})
		}
		return goja.Undefined()
	})

	vm.Set("log", func(call goja.FunctionCall) goja.Value {
		msg := call.Argument(0).String()
		slog.Info("script", "message", msg)
		return goja.Undefined()
	})

	vm.Set("dbQuery", func(call goja.FunctionCall) goja.Value {
		query := call.Argument(0).String()
		if e.onEvent != nil {
			e.onEvent("dbQuery", query)
		}
		return goja.Undefined()
	})

	vm.Set("sleep", func(call goja.FunctionCall) goja.Value {
		ms := call.Argument(0).ToInteger()
		if ms > 0 && ms <= 60000 {
			time.Sleep(time.Duration(ms) * time.Millisecond)
		}
		return goja.Undefined()
	})
}

func (e *ScriptEngine) SetEventHandler(handler func(string, interface{})) {
	e.onEvent = handler
}

func (e *ScriptEngine) LoadScript(id uint, name, code string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	for _, s := range e.scripts {
		if s.Name == name {
			s.Code = code
			return e.reparseEvents(s)
		}
	}

	ls := &LoadedScript{
		ID:     id,
		Name:   name,
		Code:   code,
		Active: true,
	}
	if err := e.reparseEvents(ls); err != nil {
		return err
	}
	e.scripts = append(e.scripts, ls)
	return nil
}

func (e *ScriptEngine) reparseEvents(s *LoadedScript) error {
	vm := goja.New()
	events := make(map[string]bool)
	vm.Set("on", func(call goja.FunctionCall) goja.Value {
		eventType := call.Argument(0).String()
		events[eventType] = true
		return goja.Undefined()
	})
	_, err := vm.RunString(s.Code)
	if err != nil {
		return fmt.Errorf("script parse error: %w", err)
	}
	s.Events = make([]string, 0, len(events))
	for evt := range events {
		s.Events = append(s.Events, evt)
	}
	return nil
}

func (e *ScriptEngine) UnloadScript(name string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	for i, s := range e.scripts {
		if s.Name == name {
			e.scripts = append(e.scripts[:i], e.scripts[i+1:]...)
			return
		}
	}
}

func (e *ScriptEngine) FireEvent(eventType string, data interface{}) {
	e.mu.Lock()
	callbacks := make([]goja.Callable, 0)
	for _, cb := range e.events[eventType] {
		callbacks = append(callbacks, cb)
	}
	e.mu.Unlock()

	var dataValue goja.Value
	if data != nil {
		jsonData, err := json.Marshal(data)
		if err == nil {
			var parsed interface{}
			if json.Unmarshal(jsonData, &parsed) == nil {
				dataValue = e.vm.ToValue(parsed)
			}
		}
	}
	if dataValue == nil {
		dataValue = goja.Undefined()
	}

	for _, cb := range callbacks {
		_, err := cb(goja.Undefined(), dataValue)
		if err != nil {
			slog.Error("script event handler error", "event", eventType, "error", err)
		}
	}
}

func (e *ScriptEngine) ListScripts() []Script {
	e.mu.Lock()
	defer e.mu.Unlock()
	result := make([]Script, 0, len(e.scripts))
	for _, s := range e.scripts {
		result = append(result, Script{
			ID:        fmt.Sprintf("%d", s.ID),
			Name:      s.Name,
			Code:      s.Code,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		})
	}
	return result
}

type Script struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Code        string    `json:"code"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	RunCount    int       `json:"run_count"`
	LastRun     time.Time `json:"last_run"`
}

type ExecutionResult struct {
	Success bool   `json:"success"`
	Output  string `json:"output"`
	Error   string `json:"error"`
}

func (e *ScriptEngine) SaveScript(script *Script) {
	e.mu.Lock()
	defer e.mu.Unlock()
	script.UpdatedAt = time.Now()
	if script.CreatedAt.IsZero() {
		script.CreatedAt = time.Now()
	}

	ls := &LoadedScript{
		ID:     0,
		Name:   script.Name,
		Code:   script.Code,
		Active: true,
	}
	e.reparseEvents(ls)
	e.scripts = append(e.scripts, ls)
}

func (e *ScriptEngine) GetScript(id string) (*Script, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, s := range e.scripts {
		if fmt.Sprintf("%d", s.ID) == id || s.Name == id {
			return &Script{
				ID:   fmt.Sprintf("%d", s.ID),
				Name: s.Name,
				Code: s.Code,
			}, true
		}
	}
	return nil, false
}

func (e *ScriptEngine) DeleteScript(id string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	for i, s := range e.scripts {
		if fmt.Sprintf("%d", s.ID) == id || s.Name == id {
			e.scripts = append(e.scripts[:i], e.scripts[i+1:]...)
			return true
		}
	}
	return false
}

func (e *ScriptEngine) Execute(scriptID string, context map[string]interface{}) ExecutionResult {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, s := range e.scripts {
		if fmt.Sprintf("%d", s.ID) == scriptID || s.Name == scriptID {
			return e.executeScript(s.Code, context)
		}
	}
	return ExecutionResult{Success: false, Error: "script not found"}
}

func (e *ScriptEngine) ExecuteCode(code string, context map[string]interface{}) ExecutionResult {
	return e.executeScript(code, context)
}

func (e *ScriptEngine) executeScript(code string, context map[string]interface{}) ExecutionResult {
	vm := goja.New()
	registry := new(require.Registry)
	registry.Enable(vm)
	console.Enable(vm)

	for k, v := range context {
		vm.Set(k, v)
	}

	done := make(chan ExecutionResult, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- ExecutionResult{Success: false, Error: fmt.Sprintf("panic: %v", r)}
			}
		}()
		v, err := vm.RunString(code)
		if err != nil {
			done <- ExecutionResult{Success: false, Error: err.Error()}
			return
		}
		if v != nil {
			done <- ExecutionResult{Success: true, Output: v.String()}
		} else {
			done <- ExecutionResult{Success: true, Output: ""}
		}
	}()

	select {
	case result := <-done:
		return result
	case <-time.After(30 * time.Second):
		return ExecutionResult{Success: false, Error: "script execution timeout (30s)"}
	}
}
