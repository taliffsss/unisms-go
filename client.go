// Package unisms provides an official Go client for the UniSMS API
// (https://unismsapi.com), an SMS API for the Philippines supporting all
// major carriers (Globe, Smart, DITO, Sun, TNT).
//
// The client exposes exactly two operations, mirroring the upstream API:
// sending an SMS (Client.Send) and retrieving a previously sent message's
// delivery status (Client.GetMessage).
//
// # Authentication
//
// All requests are authenticated via HTTP Basic Auth, using the secret
// key as the username and an empty password. Obtain a secret key from
// your UniSMS dashboard at https://unismsapi.com.
//
// # Errors
//
// Failures are surfaced as one of three error types, all compatible with
// errors.Is and errors.As: ValidationError for client-side validation
// failures (and response decode failures), TransportError for
// network-level failures before a response was received, and APIError
// for non-2xx HTTP responses from the API.
//
// # Retries
//
// By default, requests are retried up to twice (three attempts total)
// with exponential backoff on network errors, timeouts, HTTP 429, and
// HTTP 5xx responses. Retrying Client.Send carries an inherent risk of
// duplicate SMS delivery if an earlier attempt actually succeeded
// server-side but its response was lost (for example due to a timeout).
// Pass WithMaxRetries(0) to New to disable retries entirely.
package unisms

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/taliffsss/unisms-go/internal/retry"
	"github.com/taliffsss/unisms-go/internal/transport"
)

// defaultSenderID is used for SendRequest.SenderID when left blank.
const defaultSenderID = "UniSMS"

// Client is a UniSMS API client. Construct one with New. A Client is
// safe for concurrent use by multiple goroutines.
type Client struct {
	secretKey string
	baseURL   string
	executor  *transport.Executor
}

// New constructs a UniSMS Client using the given secret key. If secretKey
// is empty, the UNISMS_SECRET_KEY environment variable is consulted as a
// fallback. If no non-empty secret key is available from either source,
// New returns a *ValidationError.
//
// Optional behavior (base URL, timeout, retries, custom HTTP client) can
// be configured via functional options; see WithBaseURL, WithTimeout,
// WithMaxRetries, WithRetryDelay, and WithHTTPClient. Explicit options
// always take precedence over the UNISMS_BASE_URL and UNISMS_TIMEOUT
// environment variables, which in turn take precedence over built-in
// defaults.
func New(secretKey string, opts ...Option) (*Client, error) {
	key := strings.TrimSpace(secretKey)
	if key == "" {
		key = strings.TrimSpace(secretKeyFromEnv())
	}
	if key == "" {
		return nil, newValidationError("secretKey", "secret key must not be empty")
	}

	cfg := resolveConfig(opts...)

	policy := retry.Policy{
		MaxRetries: cfg.maxRetries,
		BaseDelay:  cfg.retryDelay,
		MaxDelay:   cfg.maxDelay,
	}

	return &Client{
		secretKey: key,
		baseURL:   cfg.baseURL,
		executor:  transport.NewExecutor(cfg.httpClient, policy),
	}, nil
}

// Send sends an SMS message. req.Recipient and req.Content are required;
// a *ValidationError is returned immediately, without making any network
// call, if either is missing or blank. req.SenderID defaults to "UniSMS"
// when blank. req.Metadata is omitted entirely from the request body
// when nil.
//
// The context governs the request's deadline/cancellation, including
// across retries.
func (c *Client) Send(ctx context.Context, req SendRequest) (Response, error) {
	if strings.TrimSpace(req.Recipient) == "" {
		return nil, newValidationError("recipient", "recipient is required")
	}
	if strings.TrimSpace(req.Content) == "" {
		return nil, newValidationError("content", "content is required")
	}

	senderID := req.SenderID
	if strings.TrimSpace(senderID) == "" {
		senderID = defaultSenderID
	}

	payload := sendPayload{
		Recipient: req.Recipient,
		Content:   req.Content,
		SenderID:  senderID,
		Metadata:  req.Metadata,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, newValidationError("", fmt.Sprintf("failed to encode request body: %v", err))
	}

	result, err := c.executor.Execute(ctx, transport.Request{
		Method:      "POST",
		URL:         c.baseURL,
		AuthUser:    c.secretKey,
		Body:        body,
		ContentType: "application/json",
	})
	if err != nil {
		return nil, translateError(err)
	}

	return decodeResponse(result.Body)
}

// GetMessage retrieves the delivery status of a previously sent SMS by
// its message ID. id is required; a *ValidationError is returned
// immediately, without making any network call, if it is blank.
//
// The context governs the request's deadline/cancellation, including
// across retries.
func (c *Client) GetMessage(ctx context.Context, id string) (Response, error) {
	if strings.TrimSpace(id) == "" {
		return nil, newValidationError("id", "message id is required")
	}

	reqURL := c.baseURL + "/" + url.PathEscape(id)

	result, err := c.executor.Execute(ctx, transport.Request{
		Method:   "GET",
		URL:      reqURL,
		AuthUser: c.secretKey,
	})
	if err != nil {
		return nil, translateError(err)
	}

	return decodeResponse(result.Body)
}

// decodeResponse decodes a raw JSON response body into a Response,
// wrapping decode failures in a *ValidationError.
func decodeResponse(body []byte) (Response, error) {
	var decoded Response
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	if err := dec.Decode(&decoded); err != nil {
		return nil, &ValidationError{
			Message: fmt.Sprintf("failed to decode UniSMS API response: %v", err),
			Err:     err,
		}
	}
	return decoded, nil
}

// translateError converts an internal/transport.Error into the exported
// error hierarchy (TransportError or APIError).
func translateError(err error) error {
	tErr, ok := err.(*transport.Error)
	if !ok {
		return newTransportError("request failed", err)
	}

	switch tErr.Kind {
	case transport.FailureAPI:
		return newAPIError(tErr.StatusCode, tErr.Body)
	case transport.FailureNetwork:
		return newTransportError("request failed", tErr.Cause)
	default:
		return newTransportError(tErr.Error(), tErr.Cause)
	}
}
