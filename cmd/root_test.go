package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lambda-feedback/shimmy/config"
	"github.com/lambda-feedback/shimmy/internal/execution/supervisor"
)

func TestValidateRootConfigRequiresCommandForProcessInterfaces(t *testing.T) {
	for _, iface := range []supervisor.IOInterface{supervisor.RpcIO, supervisor.FileIO} {
		t.Run(string(iface), func(t *testing.T) {
			var cfg config.Config
			cfg.Runtime.Supervisor.IO.Interface = iface
			err := validateRootConfig(cfg, "")
			require.Error(t, err)
			assert.Contains(t, err.Error(), "--command")
		})
	}
}

func TestValidateRootConfigAcceptsWasmModuleOverride(t *testing.T) {
	var cfg config.Config
	cfg.Runtime.Supervisor.IO.Interface = supervisor.WasmIO
	require.NoError(t, validateRootConfig(cfg, "/opt/evaluator.wasm"))
}

func TestValidateRootConfigRequiresWasmModulePath(t *testing.T) {
	var cfg config.Config
	cfg.Runtime.Supervisor.IO.Interface = supervisor.WasmIO
	err := validateRootConfig(cfg, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "FUNCTION_WASM_MODULE")
}
