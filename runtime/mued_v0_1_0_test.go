package runtime_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lambda-feedback/shimmy/runtime"
)

// The v0.1.0 adapter must be a pure delegation to the package-level transform
// functions — these tests are the byte-for-byte regression guard for that.

func v010(t *testing.T) runtime.MuEdAdapter {
	t.Helper()
	a := runtime.DefaultMuEdRegistry().Adapter("0.1.0")
	require.NotNil(t, a)
	return a
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return string(b)
}

func TestMuEdV010_DecodeEvaluate_MatchesFreeFunctions(t *testing.T) {
	a := v010(t)

	evalReq := runtime.MuEdEvaluateRequest{
		Submission: runtime.MuEdSubmission{Type: runtime.MuEdMath, Content: map[string]any{"expression": "x^2"}},
		Task:       &runtime.MuEdTask{ReferenceSolution: map[string]any{"expression": "x^2"}},
	}
	evalBody := mustJSON(t, evalReq)

	gotLegacy, gotCmd, err := a.DecodeEvaluate([]byte(evalBody))
	require.NoError(t, err)
	assert.Equal(t, runtime.CommandEvaluate, gotCmd)
	wantLegacy, err := runtime.MuEdBuildLegacyEvaluateRequest(evalReq)
	require.NoError(t, err)
	assert.Equal(t, wantLegacy, gotLegacy)

	previewReq := runtime.MuEdEvaluateRequest{
		Submission:            runtime.MuEdSubmission{Type: runtime.MuEdMath, Content: map[string]any{"expression": "x^2"}},
		PreSubmissionFeedback: &runtime.MuEdPreSubmissionFeedback{Enabled: true},
	}
	previewBody := mustJSON(t, previewReq)

	gotLegacy, gotCmd, err = a.DecodeEvaluate([]byte(previewBody))
	require.NoError(t, err)
	assert.Equal(t, runtime.CommandPreview, gotCmd)
	wantLegacy, err = runtime.MuEdBuildLegacyPreviewRequest(previewReq)
	require.NoError(t, err)
	assert.Equal(t, wantLegacy, gotLegacy)
}

func TestMuEdV010_DecodeEvaluate_Errors(t *testing.T) {
	a := v010(t)

	_, _, err := a.DecodeEvaluate([]byte("not json"))
	assert.Error(t, err)

	missingRef := mustJSON(t, runtime.MuEdEvaluateRequest{
		Submission: runtime.MuEdSubmission{Type: runtime.MuEdMath, Content: map[string]any{"expression": "x^2"}},
	})
	_, _, err = a.DecodeEvaluate([]byte(missingRef))
	assert.Error(t, err, "missing task.referenceSolution is a decode error")
}

func TestMuEdV010_EncodeEvaluateFeedback_MatchesFreeFunctions(t *testing.T) {
	a := v010(t)

	evalResult := map[string]any{"is_correct": true, "feedback": "Well done"}
	gotFb, err := a.EncodeEvaluateFeedback(runtime.CommandEvaluate, evalResult)
	require.NoError(t, err)
	assert.Equal(t, runtime.MuEdToEvaluateFeedback(evalResult), gotFb)

	previewResult := map[string]any{"preview": map[string]any{"latex": "x^{2}"}}
	gotFb, err = a.EncodeEvaluateFeedback(runtime.CommandPreview, previewResult)
	require.NoError(t, err)
	assert.Equal(t, runtime.MuEdToPreviewFeedback(previewResult), gotFb)
}

func TestMuEdV010_EncodeHealth_MatchesFreeFunction(t *testing.T) {
	a := v010(t)

	for _, passed := range []bool{true, false} {
		result := map[string]any{"tests_passed": passed}
		assert.Equal(t, runtime.MuEdToHealthResponse(result), a.EncodeHealth(result))
	}
}

func TestMuEdV010_Chat_MatchesFreeFunctions(t *testing.T) {
	a := v010(t)

	chatReq := runtime.MuEdChatRequest{Messages: []runtime.MuEdChatMessage{{Role: runtime.MuEdChatRoleUser, Content: "hi"}}}
	gotData, err := a.DecodeChat([]byte(mustJSON(t, chatReq)))
	require.NoError(t, err)
	wantData, err := runtime.MuEdBuildChatRequest(chatReq)
	require.NoError(t, err)
	assert.Equal(t, wantData, gotData)

	_, err = a.DecodeChat([]byte(`{"messages":[]}`))
	assert.Error(t, err, "empty messages is a decode error")

	chatResult := map[string]any{"output": map[string]any{"role": "ASSISTANT", "content": "hello"}}
	gotResp, err := a.EncodeChat(chatResult)
	require.NoError(t, err)
	wantResp, err := runtime.MuEdToChatResponse(chatResult)
	require.NoError(t, err)
	assert.Equal(t, wantResp, gotResp)

	healthResult := map[string]any{}
	assert.Equal(t, runtime.MuEdToChatHealthResponse(healthResult), a.EncodeChatHealth(healthResult))
}
