package runtime

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/lambda-feedback/shimmy/internal/execution/supervisor"
)

// getErrorStatusCode returns the status code for the given error.
func getErrorStatusCode(err error) int {
	if status, ok := wellKnownErrors[err]; ok {
		return status
	}

	var invalidSubmissionErr *supervisor.InvalidSubmissionError
	if errors.As(err, &invalidSubmissionErr) {
		return http.StatusUnprocessableEntity
	}

	// upstream failures and worker timeouts are retryable, so they are
	// reported as 424 rather than 5xx
	if _, ok := supervisor.RetryableErrorCode(err); ok {
		return http.StatusFailedDependency
	}

	if err, ok := err.(*validationError); ok && err.Type == validationTypeRequest {
		return http.StatusUnprocessableEntity
	}

	return http.StatusInternalServerError
}

// NewErrorResponse creates an error response for an error returned by
// the runtime, using the same status and body as the runtime handler.
func NewErrorResponse(err error) Response {
	return newErrorResponse(err)
}

// newErrorResponse creates a new error response.
func newErrorResponse(err error) Response {
	statusCode := getErrorStatusCode(err)

	type responseError struct {
		Message   string              `json:"message"`
		Error     string              `json:"error_thrown,omitempty"`
		Fields    map[string][]string `json:"fields,omitempty"`
		Code      string              `json:"code,omitempty"`
		Retryable bool                `json:"retryable,omitempty"`
	}

	responseErr := responseError{
		Message: err.Error(),
		Fields:  make(map[string][]string),
	}

	// report only the evaluation function's message, without the
	// context added while the error travelled up from the worker
	var invalidSubmissionErr *supervisor.InvalidSubmissionError
	if errors.As(err, &invalidSubmissionErr) {
		responseErr.Message = invalidSubmissionErr.Message
	}

	if code, ok := supervisor.RetryableErrorCode(err); ok {
		responseErr.Message, _ = supervisor.RetryableErrorMessage(err)
		responseErr.Code = code
		responseErr.Retryable = true
	}

	if validationErr, ok := err.(*validationError); ok {
		for _, err := range validationErr.Result.Errors() {
			responseErr.Fields[err.Field()] = append(responseErr.Fields[err.Field()], err.Description())
		}
	}

	body, err := json.Marshal(struct {
		Error responseError `json:"error"`
	}{
		Error: responseErr,
	})
	if err != nil {
		return Response{StatusCode: http.StatusInternalServerError}
	}

	return newResponse(statusCode, body)
}

// newResponse creates a new response.
func newResponse(status int, body []byte) Response {
	header := make(http.Header)
	header.Add("Content-Type", "application/json")

	return Response{
		StatusCode: status,
		Body:       body,
		Header:     header,
	}
}
