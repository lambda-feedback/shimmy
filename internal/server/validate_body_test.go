package server

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateResponseBody_NilSpec(t *testing.T) {
	assert.NoError(t, ValidateResponseBody(nil, "chat", map[string]any{"anything": true}))
}

func TestValidateResponseBody_Chat(t *testing.T) {
	spec, err := LoadOpenAPISpec()
	require.NoError(t, err)

	valid := map[string]any{
		"output":   map[string]any{"role": "ASSISTANT", "content": "hello"},
		"metadata": nil,
	}
	assert.NoError(t, ValidateResponseBody(spec, "chat", valid))

	// role not in the Message enum
	badRole := map[string]any{
		"output": map[string]any{"role": "ROBOT", "content": "hello"},
	}
	assert.Error(t, ValidateResponseBody(spec, "chat", badRole))

	// missing required "output"
	assert.Error(t, ValidateResponseBody(spec, "chat", map[string]any{"metadata": nil}))
}

func TestValidateResponseBody_EvaluateSubmission(t *testing.T) {
	spec, err := LoadOpenAPISpec()
	require.NoError(t, err)

	feedback := []map[string]any{
		{"feedbackId": "fb-1", "message": "looks good"},
	}
	assert.NoError(t, ValidateResponseBody(spec, "evaluateSubmission", feedback))

	// 200 schema is an array, not an object
	assert.Error(t, ValidateResponseBody(spec, "evaluateSubmission", map[string]any{"nope": true}))
}

func TestValidateResponseBody_UnknownOperation(t *testing.T) {
	spec, err := LoadOpenAPISpec()
	require.NoError(t, err)

	assert.Error(t, ValidateResponseBody(spec, "noSuchOperation", map[string]any{}))
}
