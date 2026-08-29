package scripting

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/console"
	"github.com/dop251/goja_nodejs/require"
)

// Caller identifies who triggered a script execution. The bridge uses it to
// enforce permissions: admin callers get full access, everyone else is
// limited to the permissions of their role.
type Caller struct {
	Username string
	Role     string
}

// Bridge is the server-side capability layer scripts call through. Every
// method is synchronous: results are returned to the script, errors are
// thrown as JavaScript exceptions. Implementations must enforce permissions
// via Caller before touching the database or the network.
type Bridge interface {
	SendTask(caller Caller, agentID, taskType, params string) (uint64, error)
	GetAgent(caller Caller, agentID string) (map[string]interface{}, error)
	ListAgents(caller Caller) ([]map[string]interface{}, error)
	HTTPRequest(caller Caller, method, url string, headers map[string]string, body string, timeoutSecs int) (map[string]interface{}, error)
	Query(caller Caller, kind string, args map[string]interface{}) (interface{}, error)
}

type ScriptEngine struct {
	vm      *goja.Runtime
	mu      sync.Mutex
	bridge  Bridge
	scripts []*LoadedScript
	events  map[string][]*eventCallback
	execMu  sync.Mutex // serializes event callback execution (goja VMs are not goroutine-safe)
}

// eventCallback stores a registered event handler together with the VM it was
// registered on: goja values are VM-bound, so callbacks must always run on
// their owner VM.
type eventCallback struct {
	vm *goja.Runtime
	cb goja.Callable
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
		vm:     vm,
		events: make(map[string][]*eventCallback),
	}
	e.registerAPI(vm, Caller{Role: "admin"})
	return e
}

func GetEngine() *ScriptEngine {
	return globalEngine
}

// SetBridge installs the server-side capability layer. Called once at server
// startup before any script can execute.
func (e *ScriptEngine) SetBridge(b Bridge) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.bridge = b
}

func (e *ScriptEngine) bridgeFor(caller Caller) Bridge {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.bridge
}

// registerAPI installs the JavaScript API surface on the given VM. The bridge
// calls are synchronous: on failure the script sees a thrown exception.
func (e *ScriptEngine) registerAPI(vm *goja.Runtime, caller Caller) {
	vm.Set("on", func(call goja.FunctionCall) goja.Value {
		eventType := call.Argument(0).String()
		cb, ok := goja.AssertFunction(call.Argument(1))
		if !ok {
			slog.Error("script: on() requires a function callback")
			return goja.Undefined()
		}
		e.mu.Lock()
		e.events[eventType] = append(e.events[eventType], &eventCallback{vm: vm, cb: cb})
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
		bridge := e.bridgeFor(caller)
		if bridge == nil {
			panic(vm.NewGoError(errors.New("script bridge is not available")))
		}
		agentID := call.Argument(0).String()
		taskType := call.Argument(1).String()
		params := call.Argument(2).String()
		id, err := bridge.SendTask(caller, agentID, taskType, params)
		if err != nil {
			panic(vm.NewGoError(err))
		}
		return vm.ToValue(id)
	})

	vm.Set("getAgent", func(call goja.FunctionCall) goja.Value {
		bridge := e.bridgeFor(caller)
		if bridge == nil {
			panic(vm.NewGoError(errors.New("script bridge is not available")))
		}
		agentID := call.Argument(0).String()
		agent, err := bridge.GetAgent(caller, agentID)
		if err != nil {
			panic(vm.NewGoError(err))
		}
		return vm.ToValue(agent)
	})

	vm.Set("listAgents", func(call goja.FunctionCall) goja.Value {
		bridge := e.bridgeFor(caller)
		if bridge == nil {
			panic(vm.NewGoError(errors.New("script bridge is not available")))
		}
		agents, err := bridge.ListAgents(caller)
		if err != nil {
			panic(vm.NewGoError(err))
		}
		return vm.ToValue(agents)
	})

	vm.Set("httpRequest", func(call goja.FunctionCall) goja.Value {
		bridge := e.bridgeFor(caller)
		if bridge == nil {
			panic(vm.NewGoError(errors.New("script bridge is not available")))
		}
		rawURL := call.Argument(0).String()
		opts := map[string]interface{}{}
		if obj, ok := call.Argument(1).(*goja.Object); ok {
			for _, k := range obj.Keys() {
				opts[k] = obj.Get(k).Export()
			}
		}
		method := "GET"
		if m, ok := opts["method"].(string); ok && m != "" {
			method = m
		}
		headers := map[string]string{}
		if h, ok := opts["headers"].(map[string]interface{}); ok {
			for k, v := range h {
				headers[k] = fmt.Sprintf("%v", v)
			}
		}
		body := ""
		if b, ok := opts["body"].(string); ok {
			body = b
		}
		timeout := 10
		if t, ok := opts["timeout"]; ok {
			if n, ok := toInt64(t); ok && n > 0 {
				timeout = int(n)
			}
		}
		res, err := bridge.HTTPRequest(caller, method, rawURL, headers, body, timeout)
		if err != nil {
			panic(vm.NewGoError(err))
		}
		return vm.ToValue(res)
	})

	vm.Set("query", func(call goja.FunctionCall) goja.Value {
		bridge := e.bridgeFor(caller)
		if bridge == nil {
			panic(vm.NewGoError(errors.New("script bridge is not available")))
		}
		kind := call.Argument(0).String()
		args := map[string]interface{}{}
		if obj, ok := call.Argument(1).(*goja.Object); ok {
			for _, k := range obj.Keys() {
				args[k] = obj.Get(k).Export()
			}
		}
		result, err := bridge.Query(caller, kind, args)
		if err != nil {
			panic(vm.NewGoError(err))
		}
		return vm.ToValue(result)
	})

	vm.Set("log", func(call goja.FunctionCall) goja.Value {
		msg := call.Argument(0).String()
		slog.Info("script", "message", msg)
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

func toInt64(v interface{}) (int64, bool) {
	switch n := v.(type) {
	case int64:
		return n, true
	case int:
		return int64(n), true
	case float64:
		return int64(n), true
	}
	return 0, false
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

// FireEvent dispatches an event to all registered handlers. Callbacks always
// run on their owner VM, serialized so concurrent events cannot race a single
// goja runtime.
func (e *ScriptEngine) FireEvent(eventType string, data interface{}) {
	e.mu.Lock()
	callbacks := make([]*eventCallback, 0)
	for _, cb := range e.events[eventType] {
		callbacks = append(callbacks, cb)
	}
	e.mu.Unlock()

	e.execMu.Lock()
	defer e.execMu.Unlock()
	for _, ec := range callbacks {
		var dataValue goja.Value
		if data != nil {
			if jsonData, err := json.Marshal(data); err == nil {
				var parsed interface{}
				if json.Unmarshal(jsonData, &parsed) == nil {
					dataValue = ec.vm.ToValue(parsed)
				}
			}
		}
		if dataValue == nil {
			dataValue = goja.Undefined()
		}
		if _, err := ec.cb(goja.Undefined(), dataValue); err != nil {
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

func (e *ScriptEngine) Execute(scriptID string, context map[string]interface{}, caller Caller) ExecutionResult {
	// Snapshot the code under the lock and run WITHOUT holding it:
	// executeScript installs an on() API whose callback re-locks e.mu, so
	// running under the lock deadlocked every hook-declaring script until
	// the 30s timeout fired — while the zombie VM then resumed detached.
	e.mu.Lock()
	var code string
	found := false
	for _, s := range e.scripts {
		if fmt.Sprintf("%d", s.ID) == scriptID || s.Name == scriptID {
			code = s.Code
			found = true
			break
		}
	}
	e.mu.Unlock()
	if !found {
		return ExecutionResult{Success: false, Error: "script not found"}
	}
	return e.executeScript(code, context, caller)
}

func (e *ScriptEngine) ExecuteCode(code string, context map[string]interface{}, caller Caller) ExecutionResult {
	return e.executeScript(code, context, caller)
}

func (e *ScriptEngine) executeScript(code string, scriptCtx map[string]interface{}, caller Caller) ExecutionResult {
	vm := goja.New()
	registry := new(require.Registry)
	registry.Enable(vm)
	console.Enable(vm)

	for k, v := range scriptCtx {
		vm.Set(k, v)
	}

	e.registerAPI(vm, caller)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	done := make(chan ExecutionResult, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				select {
				case done <- ExecutionResult{Success: false, Error: fmt.Sprintf("panic: %v", r)}:
				default:
				}
			}
		}()
		v, err := vm.RunString(code)
		if err != nil {
			select {
			case done <- ExecutionResult{Success: false, Error: err.Error()}:
			default:
			}
			return
		}
		if v != nil {
			select {
			case done <- ExecutionResult{Success: true, Output: v.String()}:
			default:
			}
		} else {
			select {
			case done <- ExecutionResult{Success: true, Output: ""}:
			default:
			}
		}
	}()

	select {
	case result := <-done:
		return result
	case <-ctx.Done():
		// goja is only preemptible via Interrupt: without this, an infinite
		// loop script keeps a 100%-CPU goroutine spinning forever (and keeps
		// firing bridge side effects after the caller already saw failure).
		vm.Interrupt("script execution timeout (30s)")
		return ExecutionResult{Success: false, Error: "script execution timeout (30s)"}
	}
}
