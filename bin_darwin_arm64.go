//go:build darwin && arm64

package hadolint

import _ "embed"

//go:embed bin/darwin-arm64
var hadolintBinary []byte
