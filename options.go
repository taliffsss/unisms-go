package unisms

import (
	"net/http"
	"os"
	"strconv"
	"time"
)

const (
	// DefaultBaseURL is the base URL for the UniSMS API used when no
	// override is supplied via WithBaseURL or the UNISMS_BASE_URL
	// environment variable.
	DefaultBaseURL = "https://unismsapi.com/api/sms"

	// DefaultTimeout is the default per-request timeout applied to the
	// underlying HTTP client when no custom client or timeout is
	// supplied.
	DefaultTimeout = 30 * time.Second

	// DefaultMaxRetries is the default number of retries (in addition to
	// the initial attempt) performed on retryable failures.
	DefaultMaxRetries = 2

	// DefaultRetryDelay is the default base backoff delay used between
	// retries. It doubles after each attempt, capped at
	// DefaultMaxRetryDelay.
	DefaultRetryDelay = 250 * time.Millisecond

	// DefaultMaxRetryDelay caps the exponential backoff delay.
	DefaultMaxRetryDelay = 5 * time.Second

	// EnvSecretKey is the environment variable consulted for the secret
	// key when one is not passed explicitly to New.
	EnvSecretKey = "UNISMS_SECRET_KEY"

	// EnvBaseURL is the environment variable consulted for the base URL
	// when WithBaseURL is not used.
	EnvBaseURL = "UNISMS_BASE_URL"

	// EnvTimeout is the environment variable consulted for the request
	// timeout (as a Go duration string, e.g. "30s") when WithTimeout is
	// not used.
	EnvTimeout = "UNISMS_TIMEOUT"
)

// HTTPDoer is the minimal interface required to execute HTTP requests.
// *http.Client satisfies this interface. Consumers may supply their own
// implementation to inject custom transports, proxies, instrumentation,
// or mocks for testing.
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// config holds the fully resolved client configuration after applying
// defaults, environment variables, and functional options.
type config struct {
	baseURL    string
	timeout    time.Duration
	maxRetries int
	retryDelay time.Duration
	maxDelay   time.Duration
	httpClient HTTPDoer
}

// Option configures a Client constructed via New. Options are applied in
// the order given and take precedence over environment variables and
// built-in defaults.
type Option func(*config)

// WithBaseURL overrides the API base URL. Intended primarily for testing
// against a mock server.
func WithBaseURL(baseURL string) Option {
	return func(c *config) {
		if baseURL != "" {
			c.baseURL = baseURL
		}
	}
}

// WithTimeout sets the per-request timeout applied when no custom
// HTTPDoer is supplied via WithHTTPClient. It has no effect if
// WithHTTPClient is also used, since the caller-supplied client owns its
// own timeout configuration.
func WithTimeout(timeout time.Duration) Option {
	return func(c *config) {
		if timeout > 0 {
			c.timeout = timeout
		}
	}
}

// WithMaxRetries sets the number of retries (in addition to the initial
// attempt) performed on retryable failures: network errors, timeouts,
// HTTP 429, and HTTP 5xx. A value of 0 disables retries entirely.
//
// Retrying Send carries a risk of duplicate SMS delivery if an earlier
// attempt actually succeeded server-side but its response was lost (for
// example due to a client-side timeout). Set MaxRetries to 0 if
// at-most-once delivery semantics are required.
func WithMaxRetries(maxRetries int) Option {
	return func(c *config) {
		if maxRetries >= 0 {
			c.maxRetries = maxRetries
		}
	}
}

// WithRetryDelay sets the base exponential backoff delay used between
// retries. The delay doubles after each failed attempt, capped at 5
// seconds.
func WithRetryDelay(delay time.Duration) Option {
	return func(c *config) {
		if delay > 0 {
			c.retryDelay = delay
		}
	}
}

// WithHTTPClient supplies a custom HTTPDoer (typically an *http.Client)
// used to execute requests. Use this to inject proxies, custom
// transports, instrumentation, or mocks for testing. When set, WithTimeout
// has no effect; configure the timeout on the supplied client instead.
func WithHTTPClient(client HTTPDoer) Option {
	return func(c *config) {
		if client != nil {
			c.httpClient = client
		}
	}
}

// resolveConfig builds the effective configuration from built-in
// defaults, environment variables, and the given functional options, in
// that order of increasing precedence.
func resolveConfig(opts ...Option) *config {
	cfg := &config{
		baseURL:    DefaultBaseURL,
		timeout:    DefaultTimeout,
		maxRetries: DefaultMaxRetries,
		retryDelay: DefaultRetryDelay,
		maxDelay:   DefaultMaxRetryDelay,
	}

	if v := os.Getenv(EnvBaseURL); v != "" {
		cfg.baseURL = v
	}
	if v := os.Getenv(EnvTimeout); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			cfg.timeout = d
		} else if secs, err := strconv.Atoi(v); err == nil && secs > 0 {
			cfg.timeout = time.Duration(secs) * time.Second
		}
	}

	for _, opt := range opts {
		opt(cfg)
	}

	if cfg.httpClient == nil {
		cfg.httpClient = &http.Client{Timeout: cfg.timeout}
	}

	return cfg
}

// secretKeyFromEnv returns the secret key from the UNISMS_SECRET_KEY
// environment variable, used as a fallback when New is called with an
// empty string.
func secretKeyFromEnv() string {
	return os.Getenv(EnvSecretKey)
}
