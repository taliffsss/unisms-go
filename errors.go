package unisms

import "fmt"

// ValidationError indicates a client-side validation failure that was
// detected before any network request was attempted, or a failure to
// decode a response body that was otherwise successfully received.
//
// Typical causes include a missing secret key, a missing/blank recipient,
// content, or message ID, and malformed JSON in an API response.
type ValidationError struct {
	// Field is the name of the field or parameter that failed validation.
	// It is empty for errors that are not tied to a single field, such as
	// JSON decode failures.
	Field string

	// Message describes what went wrong.
	Message string

	// Err is the underlying cause, if any (for example a json.Unmarshal
	// error). It may be nil.
	Err error
}

// Error implements the error interface.
func (e *ValidationError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("unisms: validation error: %s: %s", e.Field, e.Message)
	}
	return fmt.Sprintf("unisms: validation error: %s", e.Message)
}

// Unwrap returns the underlying cause, if any, allowing use with
// errors.Is and errors.As.
func (e *ValidationError) Unwrap() error {
	return e.Err
}

// newValidationError builds a ValidationError for a specific field.
func newValidationError(field, message string) *ValidationError {
	return &ValidationError{Field: field, Message: message}
}

// TransportError indicates the request could not be completed at the
// network layer: connection failures, DNS errors, TLS errors, timeouts,
// or context cancellation before any HTTP response was received.
type TransportError struct {
	// Message describes what went wrong.
	Message string

	// Err is the underlying cause (e.g. a *net.OpError or context error).
	Err error
}

// Error implements the error interface.
func (e *TransportError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("unisms: transport error: %s: %v", e.Message, e.Err)
	}
	return fmt.Sprintf("unisms: transport error: %s", e.Message)
}

// Unwrap returns the underlying cause, allowing use with errors.Is and
// errors.As.
func (e *TransportError) Unwrap() error {
	return e.Err
}

// newTransportError builds a TransportError wrapping the given cause.
func newTransportError(message string, err error) *TransportError {
	return &TransportError{Message: message, Err: err}
}

// APIError indicates the UniSMS API was reached but responded with a
// non-2xx HTTP status code.
type APIError struct {
	// StatusCode is the HTTP status code returned by the API.
	StatusCode int

	// ResponseBody is the raw, undecoded response body returned by the
	// API. It is preserved as-is even if it is not valid JSON.
	ResponseBody string
}

// Error implements the error interface.
func (e *APIError) Error() string {
	return fmt.Sprintf("unisms: API responded with status %d: %s", e.StatusCode, e.ResponseBody)
}

// newAPIError builds an APIError for the given status code and body.
func newAPIError(statusCode int, responseBody string) *APIError {
	return &APIError{StatusCode: statusCode, ResponseBody: responseBody}
}
