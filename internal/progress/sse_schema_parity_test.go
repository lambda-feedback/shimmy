package progress

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/lambda-feedback/shimmy/internal/server"
)

// These tests keep the hand-written Sse* component schemas in
// runtime/schema/mued_v0.1.0.yml in step with the structs this package
// actually serialises onto the SSE stream. They are pure parity checks —
// production does not validate frames per request (see handler/stream.go:
// only the terminal frame's data payload is checked, against the
// endpoint's own response schema).

func TestSSEFrameSchemaParity(t *testing.T) {
	spec := mustSpec(t)
	ctx := context.Background()
	now := time.Now().UTC()

	t.Run("chat step + terminal frames from a live reporter", func(t *testing.T) {
		rec, r := newRecorderReporter(t, "chat")
		r.Report(ctx, Event{Stage: StageThinking, Message: "Drafting a reply…", Timestamp: now})
		r.Report(ctx, Event{Stage: StageCompleted, Data: map[string]any{
			"output":   map[string]any{"role": "ASSISTANT", "content": "hi"},
			"metadata": map[string]any{"responseTimeMs": 12},
		}})

		frames := parseSSEFrames(t, rec.Body.String())
		mustValidate(t, spec, "SseProgressStep", frameByEvent(t, frames, "thinking").data)
		mustValidate(t, spec, "SseChatTerminalFrame", frameByEvent(t, frames, "completed").data)
	})

	t.Run("chat failed terminal frame", func(t *testing.T) {
		rec, r := newRecorderReporter(t, "chat")
		r.Report(ctx, Event{Stage: StageFailed, Error: "boom", Message: "We couldn't generate a response."})
		frames := parseSSEFrames(t, rec.Body.String())
		mustValidate(t, spec, "SseChatTerminalFrame", frameByEvent(t, frames, "failed").data)
	})

	t.Run("evaluate step + terminal frames from a live reporter", func(t *testing.T) {
		rec, r := newRecorderReporter(t, "evaluate")
		r.Report(ctx, Event{Stage: StageEvaluating, Message: "Checking your working…", Timestamp: now})
		r.Report(ctx, Event{Stage: StageCompleted, Data: map[string]any{
			"feedback": []map[string]any{{"feedbackId": "fb-1", "message": "ok"}},
		}})

		frames := parseSSEFrames(t, rec.Body.String())
		mustValidate(t, spec, "SseProgressStep", frameByEvent(t, frames, "evaluating").data)
		mustValidate(t, spec, "SseEvaluateTerminalFrame", frameByEvent(t, frames, "completed").data)
	})

	t.Run("evaluate failed terminal frame", func(t *testing.T) {
		rec, r := newRecorderReporter(t, "evaluate")
		r.Report(ctx, Event{Stage: StageFailed, Error: "boom", Message: "We couldn't evaluate your answer."})
		frames := parseSSEFrames(t, rec.Body.String())
		mustValidate(t, spec, "SseEvaluateTerminalFrame", frameByEvent(t, frames, "failed").data)
	})
}

// TestSSEStepSchema_CoversEveryStage asserts every Stage this package can
// put on a step frame validates against SseProgressStep — a canary if a
// stage is added, or an enum is later added to the schema without it.
func TestSSEStepSchema_CoversEveryStage(t *testing.T) {
	spec := mustSpec(t)
	for _, stage := range []Stage{
		StagePreparing, StageStarting, StageEvaluating, StageThinking,
		StageCompleted, StageFailed, StageProgress,
	} {
		step := sseStep{Stage: string(stage), Message: "x", Timestamp: time.Now().UTC()}
		mustValidate(t, spec, "SseProgressStep", toMap(t, step))
	}
}

// TestSSEEnvelopeStructTagsMatchSchema builds the envelope structs
// directly — real JSON tags, real field set — and asserts each (a)
// satisfies its schema and (b) emits no field the schema doesn't
// document. Catches a struct field rename/retag that skips the spec.
func TestSSEEnvelopeStructTagsMatchSchema(t *testing.T) {
	spec := mustSpec(t)
	now := time.Now().UTC()
	step := sseStep{Stage: "starting", Message: "Starting…", Timestamp: now}

	cases := []struct {
		schema  string
		payload any
	}{
		{"SseProgressStep", step},
		{"SseProgressStep", sseStep{Stage: "thinking", Timestamp: now, Data: map[string]any{"k": "v"}}},
		{"SseChatTerminalFrame", sseChatEnvelope{
			Command:  "chat",
			Output:   map[string]any{"role": "ASSISTANT", "content": "hi"},
			Metadata: map[string]any{"responseTimeMs": 12},
			Steps:    []sseStep{step},
		}},
		{"SseChatTerminalFrame", sseChatEnvelope{
			Command: "chat", Steps: []sseStep{step}, Error: "boom", Message: "failed",
		}},
		{"SseEvaluateTerminalFrame", sseEnvelope{
			Command:  "evaluate",
			Feedback: []map[string]any{{"feedbackId": "fb-1", "message": "ok"}},
			Steps:    []sseStep{step},
		}},
		{"SseEvaluateTerminalFrame", sseEnvelope{
			Command: "evaluate", Steps: []sseStep{step}, Error: "boom", Message: "failed",
		}},
	}
	for _, c := range cases {
		m := toMap(t, c.payload)
		mustValidate(t, spec, c.schema, m)
		assertAllFieldsDocumented(t, spec, c.schema, m)
	}
}

// --- helpers ---

func mustSpec(t *testing.T) *openapi3.T {
	t.Helper()
	spec, err := server.LoadOpenAPISpec()
	if err != nil {
		t.Fatalf("LoadOpenAPISpec: %v", err)
	}
	return spec
}

func mustValidate(t *testing.T, spec *openapi3.T, schemaName string, payload any) {
	t.Helper()
	if err := server.ValidateComponentSchema(spec, schemaName, payload); err != nil {
		t.Errorf("payload does not satisfy %s: %v\npayload: %+v", schemaName, err, payload)
	}
}

func assertAllFieldsDocumented(t *testing.T, spec *openapi3.T, schemaName string, m map[string]any) {
	t.Helper()
	ref := spec.Components.Schemas[schemaName]
	if ref == nil || ref.Value == nil {
		t.Fatalf("component schema %q not found", schemaName)
	}
	for k := range m {
		if ref.Value.Properties[k] == nil {
			t.Errorf("%s: field %q is emitted by the struct but not a documented property", schemaName, k)
		}
	}
}

func frameByEvent(t *testing.T, frames []sseFrame, event string) sseFrame {
	t.Helper()
	for _, f := range frames {
		if f.event == event {
			return f
		}
	}
	t.Fatalf("no %q frame among %d frames", event, len(frames))
	return sseFrame{}
}

func toMap(t *testing.T, v any) map[string]any {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return m
}
