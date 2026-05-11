//go:build !(linux && amd64) && !(linux && arm64) && !(darwin && amd64) && !(darwin && arm64) && !(windows && amd64)

package hadolint

// hadolintBinary is empty on platforms without a bundled hadolint binary.
// NewHadolinter will fall back to a `hadolint` executable found on PATH.
var hadolintBinary []byte
