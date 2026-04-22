# embedded bat binary

The `make build` target copies the compiled `bat` binary into this directory
(as `bat`) so that `internal/batbin/embed.go` can `//go:embed` it into the
volt executable.

This README exists only to guarantee the directory is non-empty when the
embed directive is evaluated before the Makefile has produced the binary
(e.g. on a fresh clone, under `go vet`). The `bat` binary itself is
gitignored; run `make build` to materialise it.
