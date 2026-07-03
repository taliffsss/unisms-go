# Publishing `github.com/taliffsss/unisms-go`

This document is for maintainers. Unlike npm or PyPI, Go modules have
**no separate package registry to upload to** — the module *is* the git
repository, and `go get` fetches directly from it (via the public Go
module proxy/checksum database as a caching layer in front of GitHub).
"Publishing" a release is therefore just: push correctly-formed code,
push a semver tag.

## One-time setup

Nothing to configure for publishing itself — no tokens, no account.
The only prerequisite is push access to
`github.com/taliffsss/unisms-go`, and (if you want the automated
release notes / proxy-warming step) the repository's default
`GITHUB_TOKEN` already has the `contents: write` permission the
workflow needs — no extra secret required.

## How a release works

[`.github/workflows/release.yml`](.github/workflows/release.yml) runs
on every `vX.Y.Z` tag push:

1. Re-validates the tagged commit on Go 1.22 and 1.23: `go mod verify`,
   `go vet`, `go build`, `go test -race -cover`, and `golangci-lint`.
   A release can never ship code that fails this gate.
2. Creates a GitHub Release from the tag with auto-generated notes.
3. Pings `proxy.golang.org` and `sum.golang.org` for the new version so
   it's immediately resolvable via `go get`, instead of waiting for
   whoever runs `go get` first to trigger that fetch.

## Cutting a release (step by step)

1. Decide the next version per [SemVer](https://semver.org/). **Read
   the "Major version releases" section below before tagging anything
   `v2.0.0` or higher** — it requires a module path change.
2. Move the `[Unreleased]` section of `CHANGELOG.md` into a new dated
   entry, and add a fresh empty `[Unreleased]` section above it.
3. Commit:
   ```bash
   git add CHANGELOG.md
   git commit -m "Prepare v0.2.0 release"
   ```
4. Tag and push — **the tag is the release**:
   ```bash
   git tag v0.2.0
   git push origin main --tags
   ```
5. The tag push triggers the `Release` workflow (test/vet/lint gate +
   GitHub Release creation + proxy warm-up). Watch it under the repo's
   *Actions* tab.
6. Confirm it's live:
   ```bash
   GOPROXY=https://proxy.golang.org go list -m github.com/taliffsss/unisms-go@v0.2.0
   ```

### Tag format matters

Go's tooling only recognizes tags of the form `vMAJOR.MINOR.PATCH`
(optionally with a `-prerelease` or `+build` suffix), e.g. `v0.2.0`,
`v1.0.0-rc.1`. A tag like `0.2.0` (no `v` prefix) or `release-0.2.0` is
invisible to `go get` and the module proxy.

### Major version releases (`v2.0.0`+) — module path changes

Go's [semantic import versioning](https://go.dev/ref/mod#major-version-suffixes)
rule: starting at v2, the module path itself must carry the major
version suffix. Before tagging `v2.0.0`:

1. Update `go.mod`'s module line:
   ```go
   module github.com/taliffsss/unisms-go/v2
   ```
2. Update every internal import that references the old path
   (`github.com/taliffsss/unisms-go/...` → `.../v2/...`).
3. Consumers then import it as `github.com/taliffsss/unisms-go/v2` and
   `go get github.com/taliffsss/unisms-go/v2@v2.0.0`.

v0.x and v1.x require no path suffix — only v2 and above.

## Verifying a published release

```bash
go list -m -versions github.com/taliffsss/unisms-go   # list all known versions
GOFLAGS=-mod=mod go get github.com/taliffsss/unisms-go@v0.2.0  # smoke-test in a scratch module
```

pkg.go.dev documentation regenerates automatically within a few minutes
of the version becoming resolvable via the proxy; you can force an
immediate check by visiting
`https://pkg.go.dev/github.com/taliffsss/unisms-go@v0.2.0`.

## Deprecating or "unpublishing" a version

Go modules are **immutable once published** — the proxy and checksum
database cache them forever, so there is no `npm unpublish` equivalent.
The supported way to steer users away from a bad version is the
[`retract` directive](https://go.dev/ref/mod#go-mod-file-retract) in
`go.mod`:

```go
module github.com/taliffsss/unisms-go

go 1.21

retract v0.2.0 // published with a broken retry loop; use v0.2.1 instead
```

Commit that change and cut a new patch release (e.g. `v0.2.1`). From
then on, `go get`, `go list -m -u`, and `govulncheck`-style tooling will
warn anyone depending on `v0.2.0` that it's retracted, and `go get
github.com/taliffsss/unisms-go@latest` will skip straight past it.

## How users install and upgrade

```bash
go get github.com/taliffsss/unisms-go              # latest
go get github.com/taliffsss/unisms-go@v0.2.0        # pin an exact version
go get -u github.com/taliffsss/unisms-go            # upgrade within the current major version
```

`import "github.com/taliffsss/unisms-go"` in code, then run `go mod
tidy` to resolve it. Consumers should check `CHANGELOG.md` before
upgrading across a major version bump (see above — major bumps change
the import path itself).
