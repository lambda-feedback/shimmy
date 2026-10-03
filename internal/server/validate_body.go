package server

import (
	"encoding/json"
	"fmt"

	"github.com/getkin/kin-openapi/openapi3"
)

// ValidateResponseBody checks payload against the 200 application/json
// schema of the given operation in the spec. It gives the SSE terminal
// frame — whose payload mirrors the non-streaming response body — the
// same schema guarantee the buffered path gets from the OpenAPI response
// filter (which can't run on a streamed response).
//
// A nil spec means "no schema available" and returns nil, so callers that
// may run without the spec loaded (e.g. under AWS Lambda) need no extra
// guard.
func ValidateResponseBody(spec *openapi3.T, operationID string, payload any) error {
	if spec == nil {
		return nil
	}

	schema, err := responseSchemaFor(spec, operationID)
	if err != nil {
		return err
	}

	return validateAgainstSchema(schema, payload)
}

// ValidateComponentSchema checks payload against the named schema in
// components/schemas. Like ValidateResponseBody, a nil spec is a no-op.
// It is used by the SSE frame parity test to keep the hand-written
// Sse* schemas in step with the structs progress emits.
func ValidateComponentSchema(spec *openapi3.T, schemaName string, payload any) error {
	if spec == nil {
		return nil
	}

	if spec.Components == nil || spec.Components.Schemas == nil {
		return fmt.Errorf("spec has no component schemas")
	}
	ref := spec.Components.Schemas[schemaName]
	if ref == nil || ref.Value == nil {
		return fmt.Errorf("component schema %q not found", schemaName)
	}

	return validateAgainstSchema(ref.Value, payload)
}

// validateAgainstSchema round-trips payload through JSON so VisitJSON
// sees the generic shapes it expects (map[string]any, []any, float64)
// rather than concrete Go types such as []map[string]any.
func validateAgainstSchema(schema *openapi3.Schema, payload any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encoding response payload: %w", err)
	}
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return fmt.Errorf("decoding response payload: %w", err)
	}
	return schema.VisitJSON(decoded)
}

// responseSchemaFor returns the 200 application/json schema for the
// operation with the given operationId.
func responseSchemaFor(spec *openapi3.T, operationID string) (*openapi3.Schema, error) {
	if spec.Paths == nil {
		return nil, fmt.Errorf("spec has no paths")
	}
	for _, item := range spec.Paths.Map() {
		for _, op := range item.Operations() {
			if op == nil || op.OperationID != operationID {
				continue
			}
			if op.Responses == nil {
				return nil, fmt.Errorf("operation %q has no responses", operationID)
			}
			resp := op.Responses.Status(200)
			if resp == nil || resp.Value == nil {
				return nil, fmt.Errorf("operation %q has no 200 response", operationID)
			}
			mt := resp.Value.Content.Get("application/json")
			if mt == nil || mt.Schema == nil || mt.Schema.Value == nil {
				return nil, fmt.Errorf("operation %q 200 response has no application/json schema", operationID)
			}
			return mt.Schema.Value, nil
		}
	}
	return nil, fmt.Errorf("operation %q not found in spec", operationID)
}
