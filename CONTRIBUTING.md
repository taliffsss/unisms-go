# Contributing to unisms-go

Thanks for considering a contribution to the UniSMS Go SDK.

## Scope

This SDK is a deliberate, faithful port of the
[`taliffsss/unisms-php`](https://github.com/taliffsss/unisms-php) reference
implementation. It intentionally supports **only** the two operations
implemented upstream:

1. Sending an SMS (`POST /api/sms`)
2. Retrieving a message's delivery status (`GET /api/sms/{id}`)

Please do not open PRs adding new endpoints (bulk SMS, balance inquiry,
message history listing, webhook verification, etc.) unless the PHP
reference implementation adds them first. Keeping the two SDKs in
functional lock-step is a project goal.

## Development setup

Requirements: Go 1.21 or later.

```bash
git clone https://github.com/taliffsss/unisms-go.git
cd unisms-go
go build ./...
go test ./...
```

## Before submitting a pull request

1. Run the full test suite:

   ```bash
   go vet ./...
   go build ./...
   go test ./... -race -cover
   ```

2. Run the linter (install from
   [golangci-lint.run](https://golangci-lint.run/usage/install/) if you
   don't have it locally):

   ```bash
   golangci-lint run
   ```

3. Format your code:

   ```bash
   gofmt -l .
   goimports -l .
   ```

4. Add or update unit tests for any behavioral change. New code paths
   should be covered by table-driven tests using `httptest.Server` or a
   mock `HTTPDoer`/`http.RoundTripper` — no real network calls in tests.

5. Add GoDoc comments to all new exported identifiers.

6. Update `CHANGELOG.md` under an `[Unreleased]` section.

## Commit style

Keep commits focused and descriptive. Explain *why* a change was made,
not just what changed.

## Code of conduct

Be respectful and constructive. Issues and PRs that are abusive or
off-topic will be closed.

## License

By contributing, you agree that your contributions will be licensed
under the project's [MIT License](LICENSE).
