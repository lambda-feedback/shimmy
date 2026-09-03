package runtime

import (
	"encoding/json"
	"fmt"
)

// muEdV010 is the MuEdAdapter for µEd API version 0.1.0. Every method delegates
// to the package-level transform functions in evaluate.go / chat.go, so 0.1.0
// behaviour is exactly what it was before the adapter layer existed.
type muEdV010 struct{}

var _ MuEdAdapter = muEdV010{}

func init() {
	defaultMuEdRegistry.Register(muEdV010{})
}

func (muEdV010) Version() string { return "0.1.0" }

func (muEdV010) DecodeEvaluate(body []byte) (map[string]any, Command, error) {
	var req MuEdEvaluateRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, "", fmt.Errorf("invalid request body")
	}

	if req.PreSubmissionFeedback != nil && req.PreSubmissionFeedback.Enabled {
		legacy, err := MuEdBuildLegacyPreviewRequest(req)
		return legacy, CommandPreview, err
	}

	legacy, err := MuEdBuildLegacyEvaluateRequest(req)
	return legacy, CommandEvaluate, err
}

func (muEdV010) EncodeEvaluateFeedback(command Command, result map[string]any) ([]map[string]any, error) {
	if command == CommandPreview {
		return MuEdToPreviewFeedback(result), nil
	}
	return MuEdToEvaluateFeedback(result), nil
}

func (muEdV010) EncodeHealth(legacyResult map[string]any, streamingEnabled bool) map[string]any {
	return MuEdToHealthResponse(legacyResult, streamingEnabled)
}

func (muEdV010) DecodeChat(body []byte) (map[string]any, error) {
	var req MuEdChatRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, fmt.Errorf("invalid request body")
	}
	return MuEdBuildChatRequest(req)
}

func (muEdV010) EncodeChat(result map[string]any) (map[string]any, error) {
	return MuEdToChatResponse(result)
}

func (muEdV010) EncodeChatHealth(result map[string]any, streamingEnabled bool) map[string]any {
	return MuEdToChatHealthResponse(result, streamingEnabled)
}
