package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

// WASM plugin tier (untrusted code).
//
// Native plugins (python/powershell/bash/go) run as real child processes: they
// inherit the server account's reach, whatever the manifest claims. The WASM
// tier exists for code you do NOT trust to run natively:
//
//   - no WASI: the module cannot open files, sockets, environment or clocks
//   - no imports except the tiny `forgec2` host module below
//   - linear memory capped (default 16 MiB)
//   - wall-clock timeout enforced by the runtime
//   - stdout equivalent capped (log lines + result size)
//
// ABI v1 (deliberately tiny so it is implementable from C/Rust/TinyGo without
// a host SDK):
//
//	exports:
//	  memory              linear memory
//	  run(ptr, len) -> (ptr, len)
//	                      executes the plugin. ptr/len address the input JSON
//	                      the host wrote. The two results locate the JSON the
//	                      module produced, anywhere in its own memory.
//	imports (module "forgec2"):
//	  log(ptr, len)       append one diagnostic line (capped, length-prefixed)
//
// The returned pointer/length is never trusted blindly: it is clamped to the
// module's memory bounds and to maxWasmResultBytes, and a module may return
// ptr=0/len=0 to mean "no result".
const (
	// wasmInputOffset is where the host writes the input JSON. The first KiB is
	// reserved for the module's own scratch/globals; a large input simply grows
	// linear memory, and the module picks its own result location afterwards.
	wasmInputOffset = 1024
	// maxWasmResultBytes bounds the returned result.
	maxWasmResultBytes = 2 << 20 // 2 MiB, matching the native stdout cap
	// maxWasmLogBytes bounds total log output per run.
	maxWasmLogBytes = 256 << 10 // 256 KiB
	// wasmMemoryPages is the linear-memory cap (16 MiB).
	wasmMemoryPages = 256
	// maxWasmBinaryBytes bounds the module size we are willing to compile.
	maxWasmBinaryBytes = 16 << 20 // 16 MiB
)

// wasmRunner owns a shared wazero runtime plus a compiled-module cache so a
// report that runs on every page view does not recompile the module each time.
type wasmRunner struct {
	mu       sync.Mutex
	runtime  wazero.Runtime
	cache    map[string]wazero.CompiledModule
	hostMod  api.Module
	disabled bool
}

var (
	sharedWasmOnce sync.Once
	sharedWasm     *wasmRunner
)

// getWasmRunner returns the process-wide runner, initializing it on first use.
func getWasmRunner() (*wasmRunner, error) {
	var initErr error
	sharedWasmOnce.Do(func() {
		sharedWasm, initErr = newWasmRunner(context.Background())
	})
	if sharedWasm == nil && initErr != nil {
		return nil, initErr
	}
	return sharedWasm, nil
}

func newWasmRunner(ctx context.Context) (*wasmRunner, error) {
	// WithCloseOnContextDone makes wazero abort execution when the context is
	// cancelled, which is how the per-run timeout is enforced.
	rt := wazero.NewRuntimeWithConfig(ctx,
		wazero.NewRuntimeConfig().
			WithMemoryLimitPages(wasmMemoryPages).
			WithMemoryCapacityFromMax(true).
			WithCloseOnContextDone(true).
			WithCompilationCache(wazero.NewCompilationCache()),
	)
	r := &wasmRunner{runtime: rt, cache: map[string]wazero.CompiledModule{}}

	// The only host module a plugin may import.
	builder := r.runtime.NewHostModuleBuilder("forgec2")
	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, mod api.Module, ptr, length uint32) {
			run := activeLogSink(ctx)
			if run == nil {
				return
			}
			run.addLog(mod, ptr, length)
		}).
		Export("log")
	hostMod, err := builder.Instantiate(ctx)
	if err != nil {
		rt.Close(ctx)
		return nil, fmt.Errorf("failed to instantiate forgec2 host module: %w", err)
	}
	r.hostMod = hostMod
	return r, nil
}

// Close releases the shared runtime (tests and shutdown).
func (r *wasmRunner) Close(ctx context.Context) {
	if r == nil || r.runtime == nil {
		return
	}
	_ = r.runtime.Close(ctx)
}

// runWasmPlugin executes a WASM plugin and returns its JSON result bytes as
// stdout, so the existing Result/Report parsers work unchanged.
func (e *executor) runWasmPlugin(ctx context.Context, pluginDir string, m *Manifest, input map[string]interface{}) (*execResult, error) {
	runner, err := getWasmRunner()
	if err != nil {
		return nil, err
	}
	entry := filepath.Join(pluginDir, filepath.FromSlash(m.Entry))
	info, err := os.Stat(entry)
	if err != nil {
		return nil, fmt.Errorf("wasm plugin %q: cannot stat entry: %w", m.Name, err)
	}
	if info.Size() > maxWasmBinaryBytes {
		return nil, fmt.Errorf("wasm plugin %q: module exceeds %d bytes", m.Name, maxWasmBinaryBytes)
	}
	binary, err := os.ReadFile(entry)
	if err != nil {
		return nil, fmt.Errorf("wasm plugin %q: cannot read module: %w", m.Name, err)
	}

	run := &wasmRun{}
	ctx = withLogSink(ctx, run)

	compiled, err := runner.compile(ctx, entry, binary)
	if err != nil {
		return nil, fmt.Errorf("wasm plugin %q: %w", m.Name, err)
	}
	mod, err := runner.runtime.InstantiateModule(ctx, compiled, wazero.NewModuleConfig().WithName(""))
	if err != nil {
		return nil, fmt.Errorf("wasm plugin %q: instantiate: %w", m.Name, err)
	}
	defer mod.Close(ctx)

	payload, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("wasm plugin %q: marshal input: %w", m.Name, err)
	}
	if err := ensureWasmRegion(mod.Memory(), wasmInputOffset, len(payload)); err != nil {
		return nil, fmt.Errorf("wasm plugin %q: input does not fit in memory: %w", m.Name, err)
	}
	if !mod.Memory().Write(wasmInputOffset, payload) {
		return nil, fmt.Errorf("wasm plugin %q: could not write input into memory", m.Name)
	}

	fn := mod.ExportedFunction("run")
	if fn == nil {
		return nil, fmt.Errorf("wasm plugin %q: module does not export run(ptr,len)->(ptr,len)", m.Name)
	}
	results, err := fn.Call(ctx, uint64(wasmInputOffset), uint64(len(payload)))
	if err != nil {
		if ctx.Err() != nil {
			return &execResult{TimedOut: true}, fmt.Errorf("wasm plugin %q timed out: %w", m.Name, ctx.Err())
		}
		return &execResult{Stderr: run.stderr()}, fmt.Errorf("wasm plugin %q failed: %w", m.Name, err)
	}
	if len(results) < 2 {
		return nil, fmt.Errorf("wasm plugin %q: run() must return (ptr, len)", m.Name)
	}
	resultPtr := uint32(results[0])
	length := uint32(results[1])
	if length == 0 || resultPtr == 0 {
		res := &execResult{Stderr: run.stderr()}
		if ctx.Err() != nil {
			res.TimedOut = true
		}
		return res, nil
	}
	if length > maxWasmResultBytes {
		return &execResult{Stderr: run.stderr()}, fmt.Errorf("wasm plugin %q: result of %d bytes exceeds the %d byte cap", m.Name, length, maxWasmResultBytes)
	}
	mem := mod.Memory()
	size := uint32(mem.Size())
	if resultPtr+length > size || resultPtr+length < resultPtr {
		return &execResult{Stderr: run.stderr()}, fmt.Errorf("wasm plugin %q: result region out of bounds (ptr=%d len=%d, memory %d)", m.Name, resultPtr, length, size)
	}
	result, ok := mem.Read(resultPtr, length)
	if !ok {
		return &execResult{Stderr: run.stderr()}, fmt.Errorf("wasm plugin %q: result region unreadable", m.Name)
	}

	res := &execResult{Stdout: result, Stderr: run.stderr()}
	if ctx.Err() != nil {
		res.TimedOut = true
	}
	return res, nil
}

func (r *wasmRunner) compile(ctx context.Context, path string, binary []byte) (wazero.CompiledModule, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if c, ok := r.cache[path]; ok {
		return c, nil
	}
	c, err := r.runtime.CompileModule(ctx, binary)
	if err != nil {
		return nil, fmt.Errorf("invalid wasm module: %w", err)
	}
	if len(r.cache) >= 64 {
		// Bounded cache: drop everything rather than grow without limit.
		r.cache = map[string]wazero.CompiledModule{}
	}
	r.cache[path] = c
	return c, nil
}

// ensureWasmRegion grows memory so [offset, offset+length) is addressable.
func ensureWasmRegion(mem api.Memory, offset, length int) error {
	if length < 0 || offset < 0 {
		return fmt.Errorf("invalid region")
	}
	need := offset + length
	pageSize := 65536
	have := int(mem.Size())
	if need <= have {
		return nil
	}
	missing := (need - have + pageSize - 1) / pageSize
	if _, ok := mem.Grow(uint32(missing)); !ok {
		return fmt.Errorf("needs %d bytes, only %d available (cap %d pages)", need, have, wasmMemoryPages)
	}
	return nil
}

// ── per-run log sink ────────────────────────────────────────────────────────
// wazero host functions receive a context, so the sink travels with the run
// instead of being shared mutable state.

type logSinkKey struct{}

type wasmLogSink struct {
	mu       sync.Mutex
	lines    []byte
	dropped  int
	maxBytes int
}

type wasmRun struct {
	sink *wasmLogSink
}

func (r *wasmRun) addLog(mod api.Module, ptr, length uint32) {
	if length > 64*1024 {
		r.sink.dropped += int(length)
		return
	}
	data, ok := mod.Memory().Read(ptr, length)
	if !ok {
		r.sink.dropped += int(length)
		return
	}
	r.sink.mu.Lock()
	defer r.sink.mu.Unlock()
	if len(r.sink.lines)+len(data) > r.sink.maxBytes {
		r.sink.dropped += len(data)
		return
	}
	r.sink.lines = append(r.sink.lines, data...)
	r.sink.lines = append(r.sink.lines, '\n')
}

func (r *wasmRun) stderr() []byte {
	if r == nil || r.sink == nil {
		return nil
	}
	r.sink.mu.Lock()
	defer r.sink.mu.Unlock()
	out := append([]byte(nil), r.sink.lines...)
	if r.sink.dropped > 0 {
		out = append(out, fmt.Sprintf("\n[forgec2: %d log bytes dropped]\n", r.sink.dropped)...)
	}
	return out
}

func withLogSink(ctx context.Context, run *wasmRun) context.Context {
	run.sink = &wasmLogSink{maxBytes: maxWasmLogBytes}
	return context.WithValue(ctx, logSinkKey{}, run)
}

func activeLogSink(ctx context.Context) *wasmRun {
	run, _ := ctx.Value(logSinkKey{}).(*wasmRun)
	return run
}

// isWasmInterpreter reports whether the manifest selects the WASM tier.
func isWasmInterpreter(interpreter string) bool {
	return strings.EqualFold(strings.TrimSpace(interpreter), "wasm")
}

// logWasmUnsupported is emitted once when a package claims the WASM tier but
// the runtime is unavailable, so operators can see why it did not run.
func logWasmUnavailable(name string, err error) {
	slog.Error("WASM plugin runtime unavailable", "plugin", name, "err", err)
}
