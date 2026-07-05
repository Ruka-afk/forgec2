package scripting

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	lua "github.com/yuin/gopher-lua"
)

// Engine manages Lua script execution
type Engine struct {
	mu      sync.Mutex
	scripts map[string]*Script
}

// Script represents a stored Lua script
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

// ExecutionResult represents the result of a script execution
type ExecutionResult struct {
	Success bool   `json:"success"`
	Output  string `json:"output"`
	Error   string `json:"error"`
}

var globalEngine = &Engine{
	scripts: make(map[string]*Script),
}

// NewEngine creates a new scripting engine
func NewEngine() *Engine {
	return &Engine{
		scripts: make(map[string]*Script),
	}
}

// ListScripts returns all stored scripts
func (e *Engine) ListScripts() []Script {
	e.mu.Lock()
	defer e.mu.Unlock()

	result := make([]Script, 0, len(e.scripts))
	for _, s := range e.scripts {
		result = append(result, *s)
	}
	return result
}

// GetScript returns a script by ID
func (e *Engine) GetScript(id string) (*Script, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	s, ok := e.scripts[id]
	return s, ok
}

// SaveScript saves a script
func (e *Engine) SaveScript(script *Script) {
	e.mu.Lock()
	defer e.mu.Unlock()

	script.UpdatedAt = time.Now()
	if script.CreatedAt.IsZero() {
		script.CreatedAt = time.Now()
	}
	e.scripts[script.ID] = script
}

// DeleteScript deletes a script
func (e *Engine) DeleteScript(id string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, ok := e.scripts[id]; ok {
		delete(e.scripts, id)
		return true
	}
	return false
}

// Execute runs a Lua script with the provided context
func (e *Engine) Execute(scriptID string, context map[string]interface{}) ExecutionResult {
	e.mu.Lock()
	script, ok := e.scripts[scriptID]
	e.mu.Unlock()

	if !ok {
		return ExecutionResult{Success: false, Error: "script not found"}
	}

	return e.runLua(script.Code, context)
}

// ExecuteCode runs arbitrary Lua code
func (e *Engine) ExecuteCode(code string, context map[string]interface{}) ExecutionResult {
	return e.runLua(code, context)
}

func (e *Engine) runLua(code string, context map[string]interface{}) ExecutionResult {
	L := lua.NewState(lua.Options{SkipOpenLibs: true})
	defer L.Close()

	// Open safe standard libraries
	for _, pair := range []struct {
		n string
		f lua.LGFunction
	}{
		{lua.LoadLibName, lua.OpenPackage},
		{lua.BaseLibName, lua.OpenBase},
		{lua.TabLibName, lua.OpenTable},
		{lua.StringLibName, lua.OpenString},
		{lua.MathLibName, lua.OpenMath},
	} {
		L.Push(L.NewFunction(pair.f))
		L.Push(lua.LString(pair.n))
		L.Call(1, 0)
	}

	// Register ForgeC2 API
	e.registerAPI(L, context)

	// Execute with timeout (30 seconds)
	done := make(chan error, 1)
	var output string

	go func() {
		err := L.DoString(code)
		if err != nil {
			done <- err
		}
		done <- nil
	}()

	select {
	case err := <-done:
		if err != nil {
			return ExecutionResult{Success: false, Error: err.Error()}
		}
	case <-time.After(30 * time.Second):
		L.Close()
		return ExecutionResult{Success: false, Error: "script execution timeout (30s)"}
	}

	// Get output from Lua global "output"
	if lv := L.GetGlobal("output"); lv != lua.LNil {
		output = lv.String()
	}

	return ExecutionResult{Success: true, Output: output}
}

// registerAPI exposes ForgeC2 functions to Lua scripts
func (e *Engine) registerAPI(L *lua.LState, ctx map[string]interface{}) {
	// forgec2.log(message)
	L.SetGlobal("forgec2", L.NewTable())
	mod := L.GetGlobal("forgec2")

	L.SetField(mod, "log", L.NewFunction(func(L *lua.LState) int {
		msg := L.ToString(1)
		slog.Info("Lua script", "message", msg)
		return 0
	}))

	L.SetField(mod, "get_agents", L.NewFunction(func(L *lua.LState) int {
		agents, ok := ctx["agents"]
		if !ok {
			L.Push(lua.LNil)
			return 1
		}
		L.Push(lua.LString(fmt.Sprintf("%v", agents)))
		return 1
	}))

	L.SetField(mod, "create_task", L.NewFunction(func(L *lua.LState) int {
		agentID := L.ToString(1)
		taskType := L.ToString(2)
		command := L.ToString(3)

		result := map[string]interface{}{
			"agent_id": agentID,
			"type":     taskType,
			"command":  command,
			"status":   "queued",
		}
		if tasks, ok := ctx["pending_tasks"].([]map[string]interface{}); ok {
			ctx["pending_tasks"] = append(tasks, result)
		} else {
			ctx["pending_tasks"] = []map[string]interface{}{result}
		}

		L.Push(lua.LString(fmt.Sprintf("Task created: %s %s %s", agentID, taskType, command)))
		return 1
	}))

	L.SetField(mod, "get_tasks", L.NewFunction(func(L *lua.LState) int {
		tasks, ok := ctx["tasks"]
		if !ok {
			L.Push(lua.LNil)
			return 1
		}
		L.Push(lua.LString(fmt.Sprintf("%v", tasks)))
		return 1
	}))

	L.SetField(mod, "get_credentials", L.NewFunction(func(L *lua.LState) int {
		creds, ok := ctx["credentials"]
		if !ok {
			L.Push(lua.LNil)
			return 1
		}
		L.Push(lua.LString(fmt.Sprintf("%v", creds)))
		return 1
	}))

	L.SetField(mod, "sleep", L.NewFunction(func(L *lua.LState) int {
		ms := L.ToInt(1)
		if ms > 0 && ms <= 60000 {
			time.Sleep(time.Duration(ms) * time.Millisecond)
		}
		return 0
	}))

	L.SetField(mod, "set_output", L.NewFunction(func(L *lua.LState) int {
		L.SetGlobal("output", L.Get(1))
		return 0
	}))
}

// GetEngine returns the global scripting engine instance
func GetEngine() *Engine {
	return globalEngine
}
