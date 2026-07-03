# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.0] - 2026-07-03

### Added

- Initial release of the official Go SDK for the UniSMS API.
- `Client.Send` — send an SMS (`POST /api/sms`).
- `Client.GetMessage` — retrieve a message's delivery status (`GET /api/sms/{id}`).
- HTTP Basic Auth using the secret key, matching the upstream API contract.
- Functional-options configuration: `WithBaseURL`, `WithTimeout`, `WithMaxRetries`,
  `WithRetryDelay`, `WithHTTPClient`.
- Environment variable fallbacks: `UNISMS_SECRET_KEY`, `UNISMS_BASE_URL`, `UNISMS_TIMEOUT`.
- Automatic retry with exponential backoff on network errors, timeouts, HTTP 429,
  and HTTP 5xx responses (default: 2 retries, 250ms base delay, 5s cap).
- Typed error hierarchy: `ValidationError`, `TransportError`, `APIError`, all
  compatible with `errors.Is` / `errors.As`.
- Full `context.Context` support on all API calls.
- Unit test suite using `httptest.Server` and a mock `HTTPDoer`.
- `examples/send-test` runnable example.
- GitHub Actions CI (`go vet`, `go build`, `go test`, `golangci-lint`).

[Unreleased]: https://github.com/taliffsss/unisms-go/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/taliffsss/unisms-go/releases/tag/v0.1.0
