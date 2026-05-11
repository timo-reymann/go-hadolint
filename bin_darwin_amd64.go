//go:build darwin && amd64

package hadolint

import _ "embed"

//go:embed bin/darwin-amd64
var hadolintBinary []byte
