package hadolint

import _ "embed"

//go:embed bin/ThirdPartyNotices.txt
var hadolintThirdPartyNotices string

//go:embed bin/hadolint-LICENSE.txt
var hadolintLicense string

// ThirdPartyNotices returns the upstream hadolint `ThirdPartyNotices.txt`
// file contents — the consolidated notice/license summary for hadolint and
// its bundled dependencies.
func ThirdPartyNotices() string {
	return hadolintThirdPartyNotices
}

// License returns the full upstream hadolint LICENSE text (GNU GPL v3.0).
//
// Software that ships the binary embedded by this package must comply with
// the GPL, typically by surfacing this text in the consuming application's
// about/credits screen or shipping it alongside the distributed binary.
func License() string {
	return hadolintLicense
}
