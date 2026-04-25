//go:build linux

// Package batbin bundles the upstream tshakalekholoane/bat binary inside
// the volt executable and exposes helpers for extracting it to the user
// cache directory on first run.
//
// volt is designed so that end users never have to build or install `bat`
// themselves: a prebuilt bat binary is committed to this repository at
// internal/batbin/batdata/bat (see internal/batbin/BAT_VERSION for the
// upstream tag and sha256). It is embedded into volt via //go:embed, and
// at runtime EnsureInstalled drops the binary into $XDG_CACHE_HOME/volt/bat
// with an executable bit set. To upgrade the bundled bat, see
// docs/UPGRADING-BAT.md.
package batbin

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	// BinaryName is the filename used both inside the embed FS and for the
	// cached extraction. Kept short because it is rendered in the TUI.
	BinaryName = "bat"

	// embedPath is the path of the binary inside batdataFS. Must stay in
	// lock-step with the Makefile.
	embedPath = "batdata/bat"
)

// ErrNotEmbedded is returned when volt was built without a real bat
// binary in the embed FS (e.g. `go build` run directly instead of
// `make build`). The TUI degrades gracefully in this case: the dashboard
// still renders but mutating actions are disabled.
var ErrNotEmbedded = errors.New("volt was built without an embedded bat binary; run `make build`")

// Embedded reports whether a real bat binary was baked into this volt
// executable.
func Embedded() bool {
	f, err := batdataFS.Open(embedPath)
	if err != nil {
		return false
	}
	defer f.Close()
	// Guard against the embedded file being an accidental placeholder:
	// the upstream binary is ≥ 1 MiB, so anything substantially smaller
	// almost certainly isn't a real ELF.
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Size() > 4096
}

// CachedPath returns the on-disk path that EnsureInstalled will write to.
// It is derived from os.UserCacheDir (typically ~/.cache/volt/bat).
func CachedPath() (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "volt", BinaryName), nil
}

// EnsureInstalled extracts the embedded bat binary into the user cache
// directory on first run and returns the path to it. Subsequent calls are
// no-ops. `installedNow` is true only when the binary was freshly written
// during this invocation — the caller can use it to surface a friendly
// "first-time setup" message.
func EnsureInstalled() (path string, installedNow bool, err error) {
	if !Embedded() {
		return "", false, ErrNotEmbedded
	}

	path, err = CachedPath()
	if err != nil {
		return "", false, err
	}

	if ok, err := upToDate(path); err != nil {
		return "", false, err
	} else if ok {
		return path, false, nil
	}

	if err := extract(path); err != nil {
		return "", false, err
	}
	return path, true, nil
}

// upToDate reports whether the cached binary is at least as large as the
// embedded one. A size mismatch triggers a re-extract, which keeps the
// cache in sync after volt itself is upgraded.
func upToDate(cachePath string) (bool, error) {
	cached, err := os.Stat(cachePath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	src, err := batdataFS.Open(embedPath)
	if err != nil {
		return false, err
	}
	defer src.Close()
	info, err := src.Stat()
	if err != nil {
		return false, err
	}
	return cached.Size() == info.Size(), nil
}

func extract(dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}

	src, err := batdataFS.Open(embedPath)
	if err != nil {
		return err
	}
	defer src.Close()

	// Write to a temp file in the same directory, then rename, so a crashed
	// extract never leaves a half-written binary at the final path.
	tmp, err := os.CreateTemp(filepath.Dir(dst), BinaryName+".*")
	if err != nil {
		return err
	}
	cleanup := func() { _ = os.Remove(tmp.Name()) }

	if _, err := io.Copy(tmp, src); err != nil {
		tmp.Close()
		cleanup()
		return err
	}
	if err := tmp.Chmod(0o755); err != nil {
		tmp.Close()
		cleanup()
		return err
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return err
	}
	if err := os.Rename(tmp.Name(), dst); err != nil {
		cleanup()
		return err
	}
	return nil
}

// Version returns the tag baked into the bundled bat binary via
// `bat -v`. The upstream format is:
//
//	bat <tag>
//	Copyright (c) 2021 Tshaka Lekholoane.
//	MIT Licence.
//
// An empty string is returned when the binary is missing, unreadable, or
// doesn't follow that format.
func Version(binPath string) string {
	if binPath == "" {
		return ""
	}
	out, err := exec.Command(binPath, "-v").CombinedOutput()
	if err != nil {
		return ""
	}
	line := strings.SplitN(strings.TrimSpace(string(out)), "\n", 2)[0]
	fields := strings.Fields(line)
	if len(fields) >= 2 && fields[0] == "bat" {
		return fields[1]
	}
	return ""
}

// resolve is a tiny convenience that returns a usable binary path together
// with its version. It never returns an error; callers inspect `path == ""`
// to detect a missing binary.
func resolve() (path, version string) {
	p, _, err := EnsureInstalled()
	if err != nil {
		return "", ""
	}
	return p, Version(p)
}

// Describe returns a one-line summary suitable for rendering in the TUI
// banner, e.g. "bat 1.2 (bundled)" or "bat (unbundled — rebuild with make)".
func Describe() string {
	if !Embedded() {
		return "bat (unbundled — run `make build`)"
	}
	_, v := resolve()
	if v == "" {
		v = EmbeddedTag()
	}
	if v == "" || v == "unknown" {
		return "bat (bundled)"
	}
	return fmt.Sprintf("bat %s (bundled)", v)
}
