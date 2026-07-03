// Package transport implements the low-level, retry-aware HTTP request
// execution used by the UniSMS client. It is deliberately decoupled from
// the public error types in the root package: it classifies failures into
// a small Result/Error shape that the root package translates into its
// exported error hierarchy (ValidationError, TransportError, APIError).
package transport

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/taliffsss/unisms-go/internal/retry"
)

// Doer is the minimal interface required to execute an *http.Request.
// *http.Client satisfies this interface, and consumers may supply their
// own implementation (for custom transports, proxies, or test mocks).
type Doer interface {
	Do(req *http.Request) (*http.Response, error)
}

// FailureKind classifies why a request ultimately failed, so the caller
// can translate it into the appropriate exported error type.
type FailureKind int

const (
	// FailureNone indicates no failure occurred.
	FailureNone FailureKind = iota
	// FailureNetwork indicates a transport-level failure: the request
	// never received an HTTP response (connection error, timeout, DNS
	// failure, context cancellation, etc.).
	FailureNetwork
	// FailureAPI indicates an HTTP response was received but its status
	// code was outside the 2xx range.
	FailureAPI
)

// Error is returned by Execute when a request ultimately fails after
// exhausting retries. It carries enough information for the caller to
// build the appropriate exported error type without internal/transport
// needing to know about it.
type Error struct {
	Kind       FailureKind
	Message    string
	Cause      error  // set when Kind == FailureNetwork
	StatusCode int    // set when Kind == FailureAPI
	Body       string // set when Kind == FailureAPI
}

// Error implements the error interface.
func (e *Error) Error() string {
	switch e.Kind {
	case FailureAPI:
		return fmt.Sprintf("transport: API error status=%d", e.StatusCode)
	case FailureNetwork:
		return fmt.Sprintf("transport: network error: %v", e.Cause)
	default:
		return e.Message
	}
}

// Unwrap exposes the underlying network cause, if any.
func (e *Error) Unwrap() error {
	return e.Cause
}

// Request describes a single HTTP request to execute, before retries.
type Request struct {
	Method string
	URL    string
	// AuthUser is sent as the HTTP Basic Auth username; the password is
	// always empty, per the UniSMS API contract.
	AuthUser string
	// Body is the raw request body to send, or nil for none.
	Body []byte
	// ContentType is set as the Content-Type header when Body is non-nil.
	ContentType string
}

// Result is the successful outcome of Execute: a 2xx HTTP response.
type Result struct {
	StatusCode int
	Body       []byte
}

// Executor performs HTTP requests with retry and backoff.
type Executor struct {
	Doer   Doer
	Policy retry.Policy
}

// NewExecutor builds an Executor with the given Doer and retry policy.
func NewExecutor(doer Doer, policy retry.Policy) *Executor {
	return &Executor{Doer: doer, Policy: policy}
}

// Execute performs the request, retrying according to the configured
// retry policy on network errors, timeouts, HTTP 429, and HTTP 5xx
// responses. It does not retry on other non-2xx status codes. Context
// cancellation is respected between attempts and during backoff sleeps.
func (e *Executor) Execute(ctx context.Context, req Request) (*Result, error) {
	var lastErr error

	maxAttempts := e.Policy.MaxRetries + 1

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			if lastErr != nil {
				return nil, lastErr
			}
			return nil, &Error{Kind: FailureNetwork, Message: "context error", Cause: err}
		}

		result, err := e.doOnce(ctx, req)
		if err == nil {
			return result, nil
		}

		lastErr = err

		if !isRetryable(err) || attempt == maxAttempts {
			return nil, err
		}

		delay := e.Policy.Delay(attempt)
		if sleepErr := retry.Sleep(ctx, delay); sleepErr != nil {
			return nil, &Error{Kind: FailureNetwork, Message: "context error during backoff", Cause: sleepErr}
		}
	}

	return nil, lastErr
}

// doOnce performs a single HTTP round trip without retrying.
func (e *Executor) doOnce(ctx context.Context, req Request) (*Result, error) {
	var bodyReader io.Reader
	if req.Body != nil {
		bodyReader = bytes.NewReader(req.Body)
	}

	httpReq, err := http.NewRequestWithContext(ctx, req.Method, req.URL, bodyReader)
	if err != nil {
		return nil, &Error{Kind: FailureNetwork, Message: "failed to build request", Cause: err}
	}

	httpReq.SetBasicAuth(req.AuthUser, "")
	if req.Body != nil && req.ContentType != "" {
		httpReq.Header.Set("Content-Type", req.ContentType)
	}

	resp, err := e.Doer.Do(httpReq)
	if err != nil {
		return nil, &Error{Kind: FailureNetwork, Message: "request failed", Cause: err}
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &Error{Kind: FailureNetwork, Message: "failed to read response body", Cause: err}
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &Error{
			Kind:       FailureAPI,
			StatusCode: resp.StatusCode,
			Body:       string(bodyBytes),
		}
	}

	return &Result{StatusCode: resp.StatusCode, Body: bodyBytes}, nil
}

// isRetryable reports whether a failed attempt should be retried:
// network-layer errors (connection/timeout/DNS/context), HTTP 429, and
// HTTP 5xx. Other 4xx API errors are not retried.
func isRetryable(err error) bool {
	var tErr *Error
	if !asTransportError(err, &tErr) {
		return false
	}

	switch tErr.Kind {
	case FailureNetwork:
		return true
	case FailureAPI:
		return tErr.StatusCode == http.StatusTooManyRequests || tErr.StatusCode >= 500
	default:
		return false
	}
}

// asTransportError is a tiny local errors.As to avoid importing errors
// just for this one assertion (kept explicit for clarity/testability).
func asTransportError(err error, target **Error) bool {
	e, ok := err.(*Error)
	if !ok {
		return false
	}
	*target = e
	return true
}
