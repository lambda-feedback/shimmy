package wasm

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds the configuration for the WASM execution backend.
//
// Configuration is read from environment variables via koanf (the same
// mechanism used by the rest of shimmy). The "conf" struct tags map to
// the koanf key names derived from the FUNCTION_* env-var prefix.
type Config struct {
	// ModulePath is the path to the .wasm file to load.
	// Populated from FUNCTION_COMMAND (the command field re-used as the
	// .wasm file path when FUNCTION_INTERFACE=wasm).
	ModulePath string `conf:"cmd"`

	// AgentPythonManifestPath binds the clean Python reactor artifact to its
	// producer manifest. When empty, the agent-python dispatcher reads
	// manifest.json next to ModulePath. FUNCTION_WASM_MANIFEST overrides it.
	AgentPythonManifestPath string `conf:"wasm_manifest"`

	// MaxInstances is the maximum number of concurrently active module
	// instances. When the pool is exhausted requests block until a slot is
	// available. Defaults to runtime.NumCPU() when <= 0.
	// Populated from FUNCTION_MAX_PROCS / max_workers.
	MaxInstances int `conf:"max_workers"`

	// Timeout is the per-request deadline passed to the WASM call.
	// Populated from FUNCTION_WORKER_SEND_TIMEOUT / send.timeout.
	Timeout time.Duration `conf:"timeout"`

	// --- Sandbox limits ---

	// MaxMemoryPages limits WASM linear memory (1 page = 64KB).
	// Default: 256 pages = 16MB. 0 means use module's own max.
	MaxMemoryPages uint32 `conf:"wasm_max_memory_pages"`

	// AllowedPaths is a list of host paths the module may read (read-only).
	// Empty means no filesystem access at all.
	AllowedPaths []string `conf:"wasm_allowed_paths"`

	// AllowedEnv is a list of env var names the module may read.
	// Empty means no env vars exposed.
	AllowedEnv []string `conf:"wasm_allowed_env"`

	// PythonScriptPath is the host path to the trusted Python evaluation script.
	// Used by Python Reactor and the independent resident Python compatibility path.
	// Python Reactor scripts must define dispatch(method, payload).
	PythonScriptPath string `conf:"wasm_python_script"`

	// PythonPreloadMode controls whether Agent Python passes the trusted evaluator
	// through runtime_prepare. "evaluator" is the default; "off" executes the
	// trusted script in each fresh request namespace.
	PythonPreloadMode string `conf:"wasm_python_preload"`

	// PythonLifecycle selects whether Agent Python modules are initialized for
	// every request, consumed once from a prepared pool, or restored to their
	// prepared linear-memory snapshot and reused.
	PythonLifecycle string `conf:"wasm_python_lifecycle"`

	// PythonPreparedCapacity bounds never-served candidates retained by the
	// single-use lifecycle. The current numpy-core artifact retains 128 MiB of
	// Guest linear memory per candidate, so this surface is deliberately small.
	PythonPreparedCapacity int `conf:"wasm_python_prepared_capacity"`

	// PythonSnapshotHeadroomBytes reserves allocator capacity before Take so
	// normal requests do not immediately grow memory beyond a restorable baseline.
	PythonSnapshotHeadroomBytes uint64 `conf:"wasm_python_snapshot_headroom_bytes"`

	// CompileCacheDir, if non-empty, enables wazero's on-disk compilation cache.
	// Set via FUNCTION_WASM_COMPILE_CACHE env var. Shared across all runners and
	// processes that point at the same directory, making cold starts much faster
	// after the first compile.
	CompileCacheDir string `conf:"wasm_compile_cache"`

	// AgentPythonObserver receives optional phase evidence. Callbacks may be
	// concurrent during refill and must return promptly. It is never populated
	// from operator configuration.
	AgentPythonObserver func(AgentPythonPhaseEvent) `conf:"-"`
}

// applyDefaults fills in zero-value fields with sensible defaults.
func (c *Config) applyDefaults() {
	if c.Timeout == 0 {
		c.Timeout = 30 * time.Second
	}
	if c.MaxMemoryPages == 0 {
		c.MaxMemoryPages = 256 // 16 MB
	}
	if c.PythonPreloadMode == "" {
		c.PythonPreloadMode = "evaluator"
	}
}

func (c *Config) validatePythonPreloadMode() error {
	switch c.PythonPreloadMode {
	case "evaluator", "off":
		return nil
	default:
		return fmt.Errorf("python preload mode %q is invalid; use \"evaluator\" or \"off\"", c.PythonPreloadMode)
	}
}

func (c *Config) applyAgentPythonDefaults() {
	if c.PythonLifecycle == "" {
		c.PythonLifecycle = "snapshot"
	}
	if c.PythonPreparedCapacity == 0 {
		c.PythonPreparedCapacity = 1
	}
	if c.PythonSnapshotHeadroomBytes == 0 {
		c.PythonSnapshotHeadroomBytes = 8 * 1024 * 1024
	}

}

func (c *Config) validateAgentPythonLifecycle() error {
	switch c.PythonLifecycle {
	case "fresh", "single-use", "snapshot":
	default:
		return fmt.Errorf("agent Python lifecycle %q is invalid; use \"fresh\", \"single-use\", or \"snapshot\"", c.PythonLifecycle)
	}
	if c.PythonPreparedCapacity < 1 || c.PythonPreparedCapacity > 4 {
		return fmt.Errorf("agent Python prepared capacity %d is outside the supported range 1..4", c.PythonPreparedCapacity)
	}
	if c.MaxInstances > 4 {
		return fmt.Errorf("agent Python max instances %d exceeds the supported limit 4", c.MaxInstances)
	}
	return nil
}

// applyEnv reads sandbox fields from FUNCTION_WASM_* environment variables.
// This allows operators to configure sandbox limits without threading them
// through the full koanf config chain.
func (c *Config) applyEnv() {
	// FUNCTION_WASM_MODULE overrides FUNCTION_COMMAND as the .wasm file path.
	if v := os.Getenv("FUNCTION_WASM_MODULE"); v != "" {
		c.ModulePath = v
	}
	if v := os.Getenv("FUNCTION_WASM_MANIFEST"); v != "" {
		c.AgentPythonManifestPath = v
	}
	if v := os.Getenv("FUNCTION_WASM_MAX_MEMORY_PAGES"); v != "" {
		if n, err := strconv.ParseUint(v, 10, 32); err == nil {
			c.MaxMemoryPages = uint32(n)
		}
	}
	if v := os.Getenv("FUNCTION_WASM_ALLOWED_PATHS"); v != "" {
		c.AllowedPaths = splitNonEmpty(v, ",")
	}
	if v := os.Getenv("FUNCTION_WASM_ALLOWED_ENV"); v != "" {
		c.AllowedEnv = splitNonEmpty(v, ",")
	}

	if v := os.Getenv("FUNCTION_WASM_PYTHON_SCRIPT"); v != "" {
		c.PythonScriptPath = v
	}
	if v := os.Getenv("FUNCTION_WASM_PYTHON_PRELOAD"); v != "" {
		c.PythonPreloadMode = v
	}
	if v := os.Getenv("FUNCTION_WASM_PYTHON_LIFECYCLE"); v != "" {
		c.PythonLifecycle = strings.TrimSpace(v)
	}
	if v := os.Getenv("FUNCTION_WASM_PYTHON_PREPARED_CAPACITY"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			c.PythonPreparedCapacity = n
		}
	}
	if v := os.Getenv("FUNCTION_WASM_PYTHON_SNAPSHOT_HEADROOM_BYTES"); v != "" {
		if n, err := strconv.ParseUint(v, 10, 64); err == nil {
			c.PythonSnapshotHeadroomBytes = n
		}
	}

	if v := os.Getenv("FUNCTION_WASM_COMPILE_CACHE"); v != "" {
		c.CompileCacheDir = v
	}
}

func splitNonEmpty(s, sep string) []string {
	var out []string
	for _, p := range strings.Split(s, sep) {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}
