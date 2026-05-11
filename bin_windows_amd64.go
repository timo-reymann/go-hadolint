//go:build windows && amd64

package hadolint

import _ "embed"

//go:embed bin/windows.exe
var hadolintBinary []byte
