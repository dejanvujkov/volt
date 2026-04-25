//go:build linux

package batbin

import "embed"

// batdataFS holds the files under ./batdata at build time. The `bat`
// binary inside it is committed to the repository (see BAT_VERSION for
// its upstream tag and sha256) and refreshed via `make update-bat
// VERSION=…`; see batbin.go for the extraction logic.
//
//go:embed batdata
var batdataFS embed.FS
