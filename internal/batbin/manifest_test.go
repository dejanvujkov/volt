//go:build linux

package batbin

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"strings"
	"testing"
)

func TestManifestMatchesBinary(t *testing.T) {
	manifestSHA := manifestField(t, "sha256:")
	if manifestSHA == "" {
		t.Fatal("BAT_VERSION is missing a sha256: field")
	}

	binBytes, err := os.ReadFile("batdata/bat")
	if err != nil {
		t.Fatalf("read embedded bat binary: %v", err)
	}

	sum := sha256.Sum256(binBytes)
	actual := hex.EncodeToString(sum[:])

	if actual != manifestSHA {
		t.Fatalf("sha256 mismatch:\n  manifest: %s\n  actual:   %s\n\n"+
			"The committed binary at internal/batbin/batdata/bat does not match\n"+
			"the sha256 in internal/batbin/BAT_VERSION. Re-run\n"+
			"`make update-bat VERSION=<tag>` to refresh both atomically.",
			manifestSHA, actual)
	}
}

func TestEmbeddedTag(t *testing.T) {
	tag := EmbeddedTag()
	if tag == "" || tag == "unknown" {
		t.Fatalf("EmbeddedTag returned %q; expected a real tag from BAT_VERSION", tag)
	}
	if !strings.HasPrefix(tag, "v") && !looksLikeVersion(tag) {
		t.Logf("warning: tag %q does not look like a version", tag)
	}
}

// manifestField returns the trimmed value following the given prefix
// (e.g. "sha256:") in BAT_VERSION, or "" if absent.
func manifestField(t *testing.T, prefix string) string {
	t.Helper()
	for _, line := range strings.Split(string(manifestBytes), "\n") {
		if v, ok := strings.CutPrefix(line, prefix); ok {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func looksLikeVersion(s string) bool {
	for _, r := range s {
		if r >= '0' && r <= '9' {
			return true
		}
	}
	return false
}
