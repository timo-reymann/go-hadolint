//go:build linux && arm64

package hadolint

import _ "embed"

//go:embed bin/linux-arm64
var hadolintBinary []byte
