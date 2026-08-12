package scripting

import (
	"errors"
	"strings"
	"testing"
)

type fakeBridge struct {
	sentTask     bool
	sentAgentID  string
	sentTaskType string
	sentParams   string
	agents       []map[string]interface{}
	queryCalls   []string
}

func (f *fakeBridge) SendTask(caller Caller, agentID, taskType, params string) (uint64, error) {
	f.sentTask = true
	f.sentAgentID = agentID
	f.sentTaskType = taskType
	f.sentParams = params
	if taskType == "fail" {
		return 0, errors.New("task rejected")
	}
	return 42, nil
}

func (f *fakeBridge) GetAgent(caller Caller, agentID string) (map[string]interface{}, error) {
	if agentID == "missing" {
		return nil, errors.New("agent not found")
	}
	return map[string]interface{}{"id": agentID, "hostname": "victim-01"}, nil
}

func (f *fakeBridge) ListAgents(caller Caller) ([]map[string]interface{}, error) {
	if f.agents == nil {
		return []map[string]interface{}{}, nil
	}
	return f.agents, nil
}

func (f *fakeBridge) HTTPRequest(caller Caller, method, url string, headers map[string]string, body string, timeoutSecs int) (map[string]interface{}, error) {
	if url == "blocked" {
		return nil, errors.New("request to private/local address is not allowed")
	}
	return map[string]interface{}{"status_code": 200, "body": "ok"}, nil
}

func (f *fakeBridge) Query(caller Caller, kind string, args map[string]interface{}) (interface{}, error) {
	f.queryCalls = append(f.queryCalls, kind)
	if kind == "bogus" {
		return nil, errors.New("unknown query kind: bogus")
	}
	return []map[string]interface{}{{"id": "a1"}}, nil
}

func TestEngineExecuteCodeWithBridge(t *testing.T) {
	engine := NewScriptEngine()
	bridge := &fakeBridge{}
	engine.SetBridge(bridge)
	caller := Caller{Username: "tester", Role: "admin"}

	res := engine.ExecuteCode(`
		var id = sendTask("agent-1", "shell", "whoami");
		var agent = getAgent("agent-1");
		var agents = listAgents();
		var q = query("agents", {status: "online"});
		id + ":" + agent.hostname + ":" + agents.length + ":" + q[0].id;
	`, nil, caller)
	if !res.Success {
		t.Fatalf("script failed: %s", res.Error)
	}
	if !bridge.sentTask || bridge.sentAgentID != "agent-1" || bridge.sentTaskType != "shell" || bridge.sentParams != "whoami" {
		t.Fatalf("sendTask not bridged correctly: %+v", bridge)
	}
	if res.Output != "42:victim-01:0:a1" {
		t.Fatalf("unexpected output: %q", res.Output)
	}
}

func TestEngineExecuteCodeBridgeErrors(t *testing.T) {
	engine := NewScriptEngine()
	bridge := &fakeBridge{}
	engine.SetBridge(bridge)
	caller := Caller{Username: "tester", Role: "admin"}

	res := engine.ExecuteCode(`sendTask("agent-1", "fail", "x");`, nil, caller)
	if res.Success {
		t.Fatalf("expected failure, got %#v", res)
	}
	if !strings.Contains(res.Error, "task rejected") {
		t.Fatalf("unexpected error: %q", res.Error)
	}

	res = engine.ExecuteCode(`query("bogus");`, nil, caller)
	if res.Success || !strings.Contains(res.Error, "unknown query kind") {
		t.Fatalf("unexpected query error result: %#v", res)
	}
}

func TestEngineWithoutBridgeThrows(t *testing.T) {
	engine := NewScriptEngine()
	res := engine.ExecuteCode(`sendTask("a", "shell", "x");`, nil, Caller{Role: "admin"})
	if res.Success || !strings.Contains(res.Error, "bridge is not available") {
		t.Fatalf("expected bridge-unavailable error, got %#v", res)
	}
}
