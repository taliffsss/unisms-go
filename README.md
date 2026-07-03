# unisms-go

Official Go SDK for the [UniSMS API](https://unismsapi.com) — a powerful,
reliable, and developer-friendly SMS API for the Philippines, supporting
all major carriers (Globe, Smart, DITO, Sun, TNT).

This is a Go port of the [`taliffsss/unisms-php`](https://github.com/taliffsss/unisms-php)
reference implementation, scoped to the same two operations: sending an
SMS and checking a message's delivery status.

Current version: **v0.1.0** (see [CHANGELOG.md](CHANGELOG.md)).

## Requirements

- Go 1.21 or later

## Installation

```bash
go get github.com/taliffsss/unisms-go
```

## Quick start

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/taliffsss/unisms-go"
)

func main() {
	client, err := unisms.New("your-secret-key")
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()

	resp, err := client.Send(ctx, unisms.SendRequest{
		Recipient: "+639171234567",
		Content:   "Hello world",
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(resp)
}
```

## Authentication

Obtain a secret key from your UniSMS dashboard at
[unismsapi.com](https://unismsapi.com). Every request is authenticated
with HTTP Basic Auth, using the secret key as the username and an empty
password — this is handled for you automatically.

You can supply the secret key explicitly:

```go
client, err := unisms.New("sk_live_...")
```

or via the `UNISMS_SECRET_KEY` environment variable, by passing an empty
string:

```go
// Reads UNISMS_SECRET_KEY from the environment.
client, err := unisms.New("")
```

An explicit, non-empty secret key argument always takes precedence over
the environment variable. If neither yields a non-empty value, `New`
returns a `*unisms.ValidationError` immediately — no network call is
made.

## Sending an SMS

```go
resp, err := client.Send(ctx, unisms.SendRequest{
	Recipient: "09171234567",
	Content:   "Your order has shipped!",
	SenderID:  "MyBrand",                     // optional, defaults to "UniSMS"
	Metadata:  map[string]interface{}{         // optional, echoed back by the API
		"order_id": 12345,
	},
})
if err != nil {
	// handle error, see "Error handling" below
}

fmt.Println(resp["message"])
```

`Recipient` and `Content` are required; `Send` returns a
`*unisms.ValidationError` before making any network call if either is
missing or blank.

## Checking a message's status

```go
status, err := client.GetMessage(ctx, "msg_84e8b93b-6315-46af-a686")
if err != nil {
	// handle error
}

fmt.Println(status)
```

`id` is required; `GetMessage` returns a `*unisms.ValidationError` before
making any network call if it is blank.

## The `Response` type

Both `Send` and `GetMessage` return a `unisms.Response`, which is a thin
`map[string]interface{}` alias — the UniSMS API does not guarantee a
fixed response schema, so the SDK avoids inventing rigid struct fields.

```go
type Response map[string]interface{}

func (r Response) String(key string) string        // safe string accessor
func (r Response) Get(key string) (interface{}, bool) // raw value + presence
```

Inspect fields directly (`resp["message"]`) or use the convenience
accessors for top-level string fields.

## Error handling

All errors are compatible with the standard `errors` package. Three
error types cover every failure mode:

- **`*unisms.ValidationError`** — a client-side validation failure
  (missing secret key, missing/blank recipient, content, or message ID)
  or a failure to decode the API's JSON response. No network call was
  made, or the response could not be parsed.
- **`*unisms.TransportError`** — the request could not be completed at
  the network layer: connection failure, DNS error, timeout, or context
  cancellation before any HTTP response was received. Wraps the
  underlying cause (`Unwrap() error`).
- **`*unisms.APIError`** — the API was reached but responded with a
  non-2xx HTTP status. Exposes `StatusCode int` and `ResponseBody string`
  (the raw, undecoded body).

```go
import "errors"

resp, err := client.Send(ctx, unisms.SendRequest{
	Recipient: "09171234567",
	Content:   "Hello world",
})
if err != nil {
	var apiErr *unisms.APIError
	var transportErr *unisms.TransportError
	var validationErr *unisms.ValidationError

	switch {
	case errors.As(err, &apiErr):
		fmt.Printf("API error (%d): %s\n", apiErr.StatusCode, apiErr.ResponseBody)
	case errors.As(err, &transportErr):
		fmt.Println("Transport error:", transportErr.Error())
		fmt.Println("Underlying cause:", errors.Unwrap(transportErr))
	case errors.As(err, &validationErr):
		fmt.Println("Validation error:", validationErr.Error())
	default:
		fmt.Println("Unexpected error:", err)
	}
}
```

## Configuration options

`New` accepts functional options:

```go
client, err := unisms.New(
	"sk_live_...",
	unisms.WithBaseURL("https://unismsapi.com/api/sms"), // default shown
	unisms.WithTimeout(30 * time.Second),                // default: 30s
	unisms.WithMaxRetries(2),                            // default: 2
	unisms.WithRetryDelay(250 * time.Millisecond),        // default: 250ms, doubles up to 5s cap
	unisms.WithHTTPClient(myCustomHTTPClient),           // inject your own *http.Client or HTTPDoer
)
```

| Option              | Default                          | Notes                                                                 |
|---------------------|-----------------------------------|------------------------------------------------------------------------|
| `WithBaseURL`        | `https://unismsapi.com/api/sms`  | Mainly useful for pointing at a test server.                          |
| `WithTimeout`        | `30s`                             | Ignored if `WithHTTPClient` is also set — configure the client itself. |
| `WithMaxRetries`     | `2`                               | Retries in addition to the initial attempt. `0` disables retries.     |
| `WithRetryDelay`     | `250ms`                           | Base backoff delay; doubles each attempt, capped at `5s`.              |
| `WithHTTPClient`     | internal `*http.Client`           | Any type satisfying `HTTPDoer` (`Do(*http.Request) (*http.Response, error)`) — inject proxies, custom transports, instrumentation, or test mocks. |

### Environment variables

These are consulted as fallbacks, with explicit constructor arguments and
options always taking precedence:

| Variable              | Purpose                                   |
|------------------------|--------------------------------------------|
| `UNISMS_SECRET_KEY`    | Used when `New` is called with `""`.       |
| `UNISMS_BASE_URL`      | Used when `WithBaseURL` is not passed.     |
| `UNISMS_TIMEOUT`       | Used when `WithTimeout` is not passed. Accepts a Go duration string (`"30s"`) or a plain integer number of seconds. |

### Context support

Every API call takes a `context.Context` as its first argument and
respects its deadline/cancellation — including between retry attempts.

```go
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()

resp, err := client.Send(ctx, req)
```

## Retry behavior — read this before relying on retries

Requests are retried automatically on:

- network/connection errors, DNS failures, and timeouts
- HTTP `429 Too Many Requests`
- HTTP `5xx` server errors

Requests are **not** retried on other `4xx` responses (e.g. `400`, `401`,
`404`) — those indicate a problem with the request itself that a retry
won't fix.

**Important — duplicate SMS risk:** retrying `Send` carries an inherent
risk of **duplicate SMS delivery**. If the first attempt actually
succeeded on the server side but the client never received the response
(for example, the connection timed out after the API had already
processed and queued the message), a retry will send the message again.
This SDK has no way to detect that scenario, since the UniSMS API surface
in scope does not include an idempotency-key mechanism.

If duplicate delivery is unacceptable for your use case, disable retries
entirely:

```go
client, err := unisms.New("sk_live_...", unisms.WithMaxRetries(0))
```

`GetMessage` (a `GET` request) does not carry this risk, since it has no
side effects.

## Testing

```bash
go build ./...
go vet ./...
go test ./... -race -cover
```

The test suite uses `httptest.Server` and a mock `HTTPDoer` — no real
network calls are made.

## API reference

Full GoDoc-style documentation is available on every exported identifier
in the source. Generate it locally with:

```bash
go doc github.com/taliffsss/unisms-go
```

Or browse it on [pkg.go.dev](https://pkg.go.dev/github.com/taliffsss/unisms-go)
once published.

Summary of the public surface:

- `unisms.New(secretKey string, opts ...Option) (*Client, error)`
- `(*Client) Send(ctx context.Context, req SendRequest) (Response, error)`
- `(*Client) GetMessage(ctx context.Context, id string) (Response, error)`
- `SendRequest{ Recipient, Content, SenderID, Metadata }`
- `Response` (`map[string]interface{}`) with `String(key)` / `Get(key)` accessors
- `ValidationError`, `TransportError`, `APIError`
- `Option` constructors: `WithBaseURL`, `WithTimeout`, `WithMaxRetries`, `WithRetryDelay`, `WithHTTPClient`

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md). Note that this SDK's scope is
intentionally limited to mirror the PHP reference implementation.

## License

The MIT License (MIT). See [LICENSE](LICENSE) for details.
