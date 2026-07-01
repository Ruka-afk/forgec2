package plugin

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeTestScript(t *testing.T, dir, name, source string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(source), 0644); err != nil {
		t.Fatalf("failed to write test script %s: %v", name, err)
	}
	return path
}

func TestExecutorEcho(t *testing.T) {
	dir := t.TempDir()
	writeTestScript(t, dir, "echo.go", `package main
import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)
func main() {
	var in map[string]interface{}
	b, _ := io.ReadAll(os.Stdin)
	json.Unmarshal(b, &in)
	out, _ := json.Marshal(map[string]interface{}{"success": true, "data": in["params"]})
	fmt.Println(string(out))
}`)

	exec := &executor{}
	manifest := &Manifest{Name: "echo", Interpreter: "go", Entry: "echo.go", Timeout: 10}
	res, err := exec.run(context.Background(), dir, manifest, map[string]interface{}{
		"agent_id": "agent-1",
		"params":   map[string]interface{}{"target": "127.0.0.1"},
		"config":   map[string]interface{}{"foo": "bar"},
	}, 0)
	if err != nil {
		t.Fatalf("unexpected run error: %v", err)
	}

	result, err := parseResult(res.Stdout)
	if err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected success, got %+v", result)
	}
	if result.Data["target"] != "127.0.0.1" {
		t.Fatalf("params mismatch: got %+v", result.Data)
	}
}

func TestExecutorTimeout(t *testing.T) {
	dir := t.TempDir()
	writeTestScript(t, dir, "sleep.go", `package main
import "time"
func main() {
	time.Sleep(2 * time.Second)
}`)

	exec := &executor{}
	manifest := &Manifest{Name: "sleeper", Interpreter: "go", Entry: "sleep.go"}
	start := time.Now()
	_, err := exec.run(context.Background(), dir, manifest, map[string]interface{}{}, 1)
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("expected timeout message, got %q", err.Error())
	}
	if elapsed < 900*time.Millisecond || elapsed > 3*time.Second {
		t.Fatalf("unexpected elapsed time: %v", elapsed)
	}
}

func TestExecutorNonZeroExit(t *testing.T) {
	dir := t.TempDir()
	writeTestScript(t, dir, "fail.go", `package main
import (
	"fmt"
	"os"
)
func main() {
	fmt.Fprintln(os.Stderr, "something went wrong")
	os.Exit(1)
}`)

	exec := &executor{}
	manifest := &Manifest{Name: "fail", Interpreter: "go", Entry: "fail.go"}
	res, err := exec.run(context.Background(), dir, manifest, map[string]interface{}{}, 10)
	if err == nil {
		t.Fatal("expected error")
	}
	if res == nil || res.ExitCode != 1 {
		t.Fatalf("expected exit code 1, got %+v", res)
	}
	if !strings.Contains(err.Error(), "something went wrong") {
		t.Fatalf("expected stderr in error, got %q", err.Error())
	}
}

func TestExecutorHookNoOutput(t *testing.T) {
	dir := t.TempDir()
	writeTestScript(t, dir, "noop.go", `package main
func main() {}`)

	exec := &executor{}
	manifest := &Manifest{Name: "noop", Interpreter: "go", Entry: "noop.go", Timeout: 10}
	res, err := exec.run(context.Background(), dir, manifest, map[string]interface{}{"event": map[string]interface{}{"type": "agent.connect"}}, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.Stdout) != 0 {
		t.Fatalf("expected empty stdout, got %q", string(res.Stdout))
	}
}
