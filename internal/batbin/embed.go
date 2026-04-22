//go:build linux

package batbin

import "embed"

// batdataFS holds the files under ./batdata at build time. The `make build`
// target drops the freshly compiled upstream `bat` binary into
// batdata/bat; see batbin.go for the extraction logic.
//
//go:embed batdata
var batdataFS embed.FS
