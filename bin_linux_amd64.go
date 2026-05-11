//go:build linux && amd64

package hadolint

import _ "embed"

//go:embed bin/linux-amd64
var hadolintBinary []byte
