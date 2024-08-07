package runtime

import (
	"fmt"

	"github.com/lambda-feedback/shimmy/runtime/schema"
	"github.com/xeipuuv/gojsonschema"
	"go.uber.org/zap"
)

// validationType is the type of validation.
type validationType int

const (
	// validationTypeRequest is the type of validation for requests.
	validationTypeRequest validationType = iota

	// validationTypeResponse is the type of validation for responses.
	validationTypeResponse
)

func (t validationType) String() string {
	switch t {
	case validationTypeRequest:
		return "request"
	case validationTypeResponse:
		return "response"
	default:
		return "unknown"
	}
}

// validationError is an error that occurs during validation.
type validationError struct {
	Type   validationType
	Result *gojsonschema.Result
}

// newValidationError creates a new validation error.
func newValidationError(t validationType, result *gojsonschema.Result) *validationError {
	return &validationError{
		Type:   t,
		Result: result,
	}
}

func (e *validationError) Error() string {
	// TODO: check if this is the correct format
	return fmt.Sprintf("%s validation error", e.Type)
}

// validate validates the data against the schema for the given command.
func (r *RuntimeHandler) validate(t validationType, command Command, data map[string]any) error {
	log := r.log.With(
		zap.String("command", string(command)),
		zap.Stringer("type", t),
	)

	schema, ok := r.schemas[t]
	if !ok {
		log.Error("validation schema not found")
		return errSchemaNotFound
	}

	schemaType, err := getSchemaType(command)
	if err != nil {
		log.Error("could not get schema type", zap.Error(err))
		return errSchemaNotFound
	}

	res, err := schema.Validate(schemaType, data)
	if err != nil {
		log.Error("validation failed", zap.Error(err))
		return errValidationFailed
	}

	if res.Valid() {
		return nil
	}

	return newValidationError(t, res)
}

// getSchemaType returns the schema type for the given command.
func getSchemaType(command Command) (schema.SchemaType, error) {
	switch command {
	case CommandEvaluate:
		return schema.SchemaTypeEval, nil
	case CommandPreview:
		return schema.SchemaTypePreview, nil
	default:
		return 0, errInvalidCommand
	}
}
