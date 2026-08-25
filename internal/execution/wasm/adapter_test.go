package wasm

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateWasm32RequestLength(t *testing.T) {
	require.NoError(t, validateWasm32RequestLength(1<<32-1))
	require.Error(t, validateWasm32RequestLength(1<<32))
}
