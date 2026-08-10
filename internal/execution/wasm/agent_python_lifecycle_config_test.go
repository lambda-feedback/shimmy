package wasm

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAgentPythonLifecycleDefaultsToSnapshotMemcpy(t *testing.T) {
	cfg := Config{}
	cfg.applyAgentPythonDefaults()

	assert.Equal(t, "snapshot", cfg.PythonLifecycle)

	assert.Equal(t, 1, cfg.PythonPreparedCapacity)
	assert.Equal(t, uint64(8*1024*1024), cfg.PythonSnapshotHeadroomBytes)
	require.NoError(t, cfg.validateAgentPythonLifecycle())
}

func TestAgentPythonLifecycleReadsExplicitSingleUseCapacity(t *testing.T) {
	t.Setenv("FUNCTION_WASM_PYTHON_LIFECYCLE", "single-use")
	t.Setenv("FUNCTION_WASM_PYTHON_PREPARED_CAPACITY", "2")
	cfg := Config{}
	cfg.applyEnv()
	cfg.applyAgentPythonDefaults()

	assert.Equal(t, "single-use", cfg.PythonLifecycle)
	assert.Equal(t, 2, cfg.PythonPreparedCapacity)
	require.NoError(t, cfg.validateAgentPythonLifecycle())
}

func TestAgentPythonLifecycleRejectsUnknownAndOversizedCapacity(t *testing.T) {
	for _, cfg := range []Config{
		{PythonLifecycle: "reuse-maybe"},
		{PythonLifecycle: "single-use", PythonPreparedCapacity: 5},
		{PythonLifecycle: "snapshot", MaxInstances: 5},
	} {
		cfg.applyAgentPythonDefaults()
		require.Error(t, cfg.validateAgentPythonLifecycle())
	}
}
