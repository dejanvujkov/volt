//go:build !linux

// This stub exists so `go build ./...` on non-Linux hosts produces a clear
// error message instead of a stream of missing-symbol complaints. volt is a
// Linux-only tool.
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "volt requires Linux (reads /sys/class/power_supply/BAT?).")
	os.Exit(1)
}
