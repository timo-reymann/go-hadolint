go-hadolint
===
[![LICENSE](https://img.shields.io/github/license/timo-reymann/go-hadolint)](https://github.com/timo-reymann/go-hadolint/blob/main/LICENSE)
[![GitHub Actions](https://github.com/timo-reymann/go-hadolint/actions/workflows/test.yml/badge.svg)](https://github.com/timo-reymann/go-hadolint/actions/workflows/test.yml)
[![GitHub Release](https://img.shields.io/github/v/tag/timo-reymann/go-hadolint?label=version)](https://github.com/timo-reymann/go-hadolint/releases)
[![Go Reference](https://pkg.go.dev/badge/github.com/timo-reymann/go-hadolint.svg)](https://pkg.go.dev/github.com/timo-reymann/go-hadolint)
[![Go Report Card](https://goreportcard.com/badge/github.com/timo-reymann/go-hadolint)](https://goreportcard.com/report/github.com/timo-reymann/go-hadolint)
[![Renovate](https://img.shields.io/badge/renovate-enabled-green)](https://renovatebot.com)

Standalone Go package that bundles [hadolint](https://github.com/hadolint/hadolint) — the Dockerfile linter — and exposes a typed API for linting Dockerfiles from your Go code, with no system-level install required.

## Features

- **Embedded hadolint binary** for `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`, and `windows/amd64` — extracted to memory at runtime via [`go-memexec`](https://github.com/amenzhinsky/go-memexec), no temp file on disk
- **Automatic `$PATH` fallback** when the embedded binary is unavailable for the current platform, plus an explicit `NewHadolinterFromPATH` constructor when callers want to prefer a system install
- **Typed JSON output** — hadolint findings parsed into Go structs (`Result`, `Finding`)
- **Pinned configuration** — pass an existing `.hadolint.yaml` path *or* let the library serialize a typed `Config` struct to a managed temp file; either way hadolint's default config discovery is short-circuited
- **Embedded license + third-party notices** for compliance, accessible via `License()` and `ThirdPartyNotices()`

## Requirements

- Go 1.24+
- The embedded binary works out of the box on the platforms listed above. Other platforms (FreeBSD, OpenBSD, 32-bit Linux, …) compile cleanly and fall through to `$PATH`.

## Installation

```bash
go get github.com/timo-reymann/go-hadolint
```

## Usage

### Lint a Dockerfile

```go
package main

import (
	"fmt"
	"log"

	"github.com/timo-reymann/go-hadolint"
)

func main() {
	h, err := hadolint.NewHadolinter()
	if err != nil {
		log.Fatal(err)
	}
	defer h.Close()

	res, err := h.AnalyzeFile("Dockerfile", "")
	if err != nil {
		log.Fatal(err)
	}

	for _, f := range res.Findings {
		fmt.Printf("%s:%d %s %s: %s\n", f.File, f.Line, f.Level, f.Code, f.Message)
	}
}
```

### Lint an in-memory snippet

```go
res, err := h.AnalyzeSnippet([]byte("FROM ubuntu\nRUN cd /tmp\n"), "")
```

### Pin a typed config (skips default discovery)

```go
strict := true
h.Config = &hadolint.Config{
	Ignored:           []string{"DL3000"},
	TrustedRegistries: []string{"my-company.com:5000"},
	StrictLabels:      &strict,
	FailureThreshold:  "warning",
}
res, _ := h.AnalyzeFile("Dockerfile", "")
```

The struct is serialized to a `go-hadolint-*.yaml` temp file on every invocation and removed immediately after the run — no persistent state on the `Hadolinter`.

### Use an existing config file

```go
h.ConfigFile = "/etc/hadolint.yaml"
```

`Config` takes precedence over `ConfigFile` when both are set.

### Prefer the system `hadolint` over the embedded one

```go
h, err := hadolint.NewHadolinterFromPATH()
```

Returns `ErrNotAvailable` if `hadolint` isn't on `$PATH`.

### Surface third-party notices

```go
fmt.Println(hadolint.License())            // hadolint's GPL-3.0 LICENSE text
fmt.Println(hadolint.ThirdPartyNotices())  // upstream ThirdPartyNotices.txt
```

Applications that ship the embedded binary must comply with hadolint's GPL-3.0 license — typically by surfacing these notices in an about/credits screen or alongside the distributed binary.

## Motivation

Tools that want to lint Dockerfiles from Go (CI orchestrators, custom linters, IDE plugins, devcontainers) usually shell out to a `hadolint` binary the user is expected to install. This package follows the same in-process pattern as [gitlab-ci-verify](https://github.com/timo-reymann/gitlab-ci-verify) embeds for ShellCheck: ship the binary with the Go module, execute it from memory, and parse JSON output into typed findings — making the package self-contained while still allowing callers to defer to a system install when they want a newer hadolint than the one embedded.

## Documentation

API docs are published at [pkg.go.dev/github.com/timo-reymann/go-hadolint](https://pkg.go.dev/github.com/timo-reymann/go-hadolint).

For hadolint's CLI flags and rule catalog, see the [upstream README](https://github.com/hadolint/hadolint).

## Contributing

I love your input! I want to make contributing to this project as easy and transparent as possible, whether it's:

- Reporting a bug
- Discussing the current state of the code
- Submitting a fix
- Proposing new features
- Becoming a maintainer

To get started please read the [Contribution Guidelines](./CONTRIBUTING.md).

## Development

### Requirements

- [Go](https://go.dev/) 1.24+
- [GNU make](https://www.gnu.org/software/make/)
- `curl` (used by the `fetch-binaries` make target)

### Refresh the bundled hadolint binaries

The hadolint executables and accompanying notice/license files are committed under `bin/`. To bump the bundled version, override `HADOLINT_VERSION` and re-run the fetch target:

```bash
make fetch-binaries HADOLINT_VERSION=v2.12.0
```

This downloads every supported platform's binary plus the upstream `LICENSE` and `ThirdPartyNotices.txt`. Upstream does not publish a native `darwin/arm64` build; the make target copies the `darwin/amd64` binary into place (which runs under Rosetta on Apple Silicon).

### Test

```bash
make test
```

CI runs the suite on `ubuntu-latest`, `ubuntu-24.04-arm`, `macos-latest`, `macos-13`, and `windows-latest`, with and without a system `hadolint` available — see `.github/workflows/test.yml`. The full behavioral suite always runs against the embedded binary; the `$PATH` fallback path is exercised in the "installed" matrix legs and skipped otherwise.

> **Note for Apple Silicon developers:** hadolint v2.12.0's `darwin/amd64` binary is known to segfault under Rosetta on some Apple Silicon hosts ([upstream issue](https://github.com/hadolint/hadolint/issues/925)). When this happens the embedded test suite is skipped with a clear message; install hadolint via `brew install hadolint` to exercise the `$PATH` fallback locally.

### Build

```bash
go build ./...
```

To verify that the unsupported-platform catch-all still compiles:

```bash
GOOS=freebsd GOARCH=amd64 go build ./...
GOOS=linux   GOARCH=386   go build ./...
```

### Credits

- [hadolint](https://github.com/hadolint/hadolint) — the Dockerfile linter this package bundles. Licensed under GPL-3.0.
- [go-memexec](https://github.com/amenzhinsky/go-memexec) — execute embedded binaries without writing to disk.
- The embedding pattern follows [`gitlab-ci-verify/internal/shellcheck`](https://github.com/timo-reymann/gitlab-ci-verify/tree/main/internal/shellcheck).

### Alternatives

- Shell out to a system-installed `hadolint` directly via `os/exec` — works, but ships no fallback and pushes the install burden onto every user of your tool.
- Run the official `hadolint/hadolint` Docker image — heavyweight for a single subprocess call.
