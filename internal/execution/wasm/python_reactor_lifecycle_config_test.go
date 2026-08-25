package wasm

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPythonReactorLifecycleDefaultsToSnapshotMemcpy(t *testing.T) {
	cfg := Config{}
	cfg.applyPythonReactorDefaults()

	assert.Equal(t, "snapshot", cfg.PythonLifecycle)

	assert.Equal(t, 1, cfg.PythonPreparedCapacity)
	assert.Equal(t, uint64(8*1024*1024), cfg.PythonSnapshotHeadroomBytes)
	assert.Equal(t, 2*time.Minute, cfg.PythonPrepareTimeout)
	assert.Equal(t, uint32(1024*1024), cfg.PythonPayloadMaxBytes)
	require.NoError(t, cfg.validatePythonReactorLifecycle())
}

func TestPythonReactorPrepareTimeoutReadsEnvironment(t *testing.T) {
	t.Setenv("FUNCTION_WASM_PYTHON_PREPARE_TIMEOUT", "3m30s")
	cfg := Config{}
	cfg.applyEnv()
	cfg.applyPythonReactorDefaults()

	assert.Equal(t, 3*time.Minute+30*time.Second, cfg.PythonPrepareTimeout)
}

func TestPythonReactorPayloadLimitReadsEnvironment(t *testing.T) {
	t.Setenv("FUNCTION_WASM_PYTHON_MAX_PAYLOAD_BYTES", "262144")
	cfg := Config{}
	cfg.applyEnv()
	cfg.applyPythonReactorDefaults()

	assert.Equal(t, uint32(262144), cfg.PythonPayloadMaxBytes)
	require.NoError(t, cfg.validatePythonReactorLifecycle())
}

func TestPythonReactorPayloadLimitRejectsValueAboveArtifactContract(t *testing.T) {
	cfg := Config{PythonPayloadMaxBytes: 1024*1024 + 1}
	cfg.applyPythonReactorDefaults()

	require.ErrorContains(t, cfg.validatePythonReactorLifecycle(), "payload limit")
}

func TestPythonReactorLifecycleReadsExplicitSingleUseCapacity(t *testing.T) {
	t.Setenv("FUNCTION_WASM_PYTHON_LIFECYCLE", "single-use")
	t.Setenv("FUNCTION_WASM_PYTHON_PREPARED_CAPACITY", "2")
	cfg := Config{}
	cfg.applyEnv()
	cfg.applyPythonReactorDefaults()

	assert.Equal(t, "single-use", cfg.PythonLifecycle)
	assert.Equal(t, 2, cfg.PythonPreparedCapacity)
	require.NoError(t, cfg.validatePythonReactorLifecycle())
}

func TestPythonReactorLifecycleRejectsUnknownAndOversizedCapacity(t *testing.T) {
	for _, cfg := range []Config{
		{PythonLifecycle: "reuse-maybe"},
		{PythonLifecycle: "single-use", PythonPreparedCapacity: 5},
		{PythonLifecycle: "snapshot", MaxInstances: 5},
	} {
		cfg.applyPythonReactorDefaults()
		require.Error(t, cfg.validatePythonReactorLifecycle())
	}
}
