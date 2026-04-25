//go:build linux

package batbin

import (
	_ "embed"
	"strings"
)

//go:embed BAT_VERSION
var manifestBytes []byte

// EmbeddedTag returns the upstream bat tag recorded in BAT_VERSION at
// build time. It is the source of truth for what version of bat is
// shipped inside this volt binary. If the manifest is malformed or
// missing the tag field, it returns "unknown".
func EmbeddedTag() string {
	for _, line := range strings.Split(string(manifestBytes), "\n") {
		if v, ok := strings.CutPrefix(line, "tag:"); ok {
			return strings.TrimSpace(v)
		}
	}
	return "unknown"
}
