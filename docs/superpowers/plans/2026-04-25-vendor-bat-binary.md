# Vendor `bat` as a Committed Binary — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the `third_party/bat` git submodule with a committed prebuilt `bat` binary plus a plain-text version manifest, so each upgrade of bundled `bat` is a deliberate, manually triggered step performed during volt release prep.

**Architecture:** The runtime embed layer (`internal/batbin`) is unchanged — `//go:embed batdata` still surfaces the binary, and `EnsureInstalled` still extracts it. What changes is the *source* of `internal/batbin/batdata/bat`: it becomes a tracked file in git instead of a build artifact. A new manifest at `internal/batbin/BAT_VERSION` records the upstream tag, sha256, source URL, and fetch date. A new `make update-bat VERSION=…` target downloads, verifies, and replaces the binary. The submodule and all build-from-source plumbing are deleted.

**Tech Stack:** Go 1.26.2, Make, GitHub Actions, `curl`, `shasum`. Linux/amd64 only.

**Pre-flight context for the implementer:**

- The spec is at `docs/superpowers/specs/2026-04-25-vendor-bat-binary-design.md`. Read it before starting.
- The submodule at `third_party/bat` is currently uninitialized in the working copy but referenced in `.gitmodules`. The binary already lives at `internal/batbin/batdata/bat` from a prior `make build`. We will keep that binary as our initial committed artifact rather than re-downloading anything in Task 1.
- Throughout, **do not** run `git submodule update --init` or anything that would re-materialize the submodule. We are removing it.
- Commit messages should be plain (no `Co-Authored-By` line, per the user's preference).

---

## File Structure

**Created:**
- `internal/batbin/BAT_VERSION` — plain-text manifest (tag, sha256, url, fetched).
- `internal/batbin/version.go` — `//go:embed BAT_VERSION` + `EmbeddedTag()`.
- `internal/batbin/manifest_test.go` — `TestManifestMatchesBinary`, `TestEmbeddedTag`.
- `docs/UPGRADING-BAT.md` — upgrade runbook.

**Modified:**
- `Makefile` — drop `bat` / `embed` / `submodule` targets and `BAT_*` vars; add `update-bat` and `verify-bat`; `build` no longer depends on `embed`; `clean` no longer touches `EMBED_BIN`.
- `.gitignore` — remove `third_party/bat/bin/` and `internal/batbin/batdata/bat`.
- `cmd/volt/main.go` — `printVersion` falls back to `batbin.EmbeddedTag()` when `Version()` is unavailable; doc comment no longer references `third_party/bat`.
- `internal/batbin/batbin.go` — `Describe` falls back to `EmbeddedTag()` when the runtime `bat -v` parse fails; doc comment updated.
- `internal/batbin/embed.go` — doc comment updated to reference `update-bat`.
- `README.md` — drop `--recurse-submodules` and the build-from-source explanation; describe the manifest workflow.
- `.github/workflows/go.yml` — no submodule init step (none currently exists; verify and add explicit `submodules: false` for safety).

**Deleted:**
- `.gitmodules`
- `third_party/bat/` (submodule reference; the worktree is already empty)

---

## Task 1: Add `BAT_VERSION` manifest seeded from the current binary

**Files:**
- Create: `internal/batbin/BAT_VERSION`

The current `internal/batbin/batdata/bat` was built from upstream commit `cd8f40925c914d575bc67e502e10adb4748b05de` (per `git submodule status`). For the initial committed manifest we must record what version that binary actually reports.

- [ ] **Step 1: Read the version the embedded binary self-reports**

Run:

```bash
./internal/batbin/batdata/bat -v
```

Expected: a line beginning with `bat ` followed by a tag, e.g. `bat 0.13`. Capture the tag (the second whitespace-separated field).

If the binary cannot execute (wrong arch, missing exec bit), run `chmod +x internal/batbin/batdata/bat` and retry. Do **not** rebuild it — we want to preserve the exact bytes that are already committed.

- [ ] **Step 2: Compute the sha256 of the embedded binary**

Run:

```bash
shasum -a 256 internal/batbin/batdata/bat | awk '{print $1}'
```

Capture the hex digest (64 chars).

- [ ] **Step 3: Determine the upstream URL**

The current binary was built from source rather than downloaded, so there is no canonical release URL for *this exact build*. Use the upstream release URL pattern for the matching tag from Step 1:

```
https://github.com/tshakalekholoane/bat/releases/download/<TAG>/bat
```

If upstream did not publish a binary release for the matching tag, fall back to the source tarball URL:

```
https://github.com/tshakalekholoane/bat/archive/refs/tags/<TAG>.tar.gz
```

It is acceptable for this seed manifest to record a URL whose binary differs from the committed bytes; the next `make update-bat` run will reconcile this. Add a one-line comment to the commit message noting the seed origin.

- [ ] **Step 4: Write `internal/batbin/BAT_VERSION`**

Create the file with these exact fields, substituting the values from Steps 1–3 and today's UTC date:

```
tag: <TAG_FROM_STEP_1>
sha256: <SHA_FROM_STEP_2>
url: <URL_FROM_STEP_3>
fetched: <TODAY_UTC_YYYY-MM-DD>
```

No trailing whitespace. Single trailing newline.

- [ ] **Step 5: Verify the file**

Run:

```bash
cat internal/batbin/BAT_VERSION
```

Expected: four lines in the order `tag:`, `sha256:`, `url:`, `fetched:`. Each value non-empty.

- [ ] **Step 6: Commit**

```bash
git add internal/batbin/BAT_VERSION
git commit -m "Add BAT_VERSION manifest for the committed bat binary"
```

---

## Task 2: Add `internal/batbin/version.go` with `EmbeddedTag()`

**Files:**
- Create: `internal/batbin/version.go`

This task introduces the compile-time embed of the manifest and a parser that returns the `tag:` field. Tests come in Task 3.

- [ ] **Step 1: Create `internal/batbin/version.go`**

```go
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
```

- [ ] **Step 2: Verify it compiles**

Run:

```bash
go build ./internal/batbin/...
```

Expected: exits 0 with no output.

- [ ] **Step 3: Commit**

```bash
git add internal/batbin/version.go
git commit -m "Add EmbeddedTag helper that reads BAT_VERSION at compile time"
```

---

## Task 3: Add `manifest_test.go` with `TestManifestMatchesBinary` and `TestEmbeddedTag`

**Files:**
- Create: `internal/batbin/manifest_test.go`

Tests catch (a) someone editing the binary without updating the manifest, and (b) a regression in `EmbeddedTag` parsing.

- [ ] **Step 1: Write the failing test file**

Create `internal/batbin/manifest_test.go`:

```go
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
		// Upstream tags are typically "vX.Y" or "X.Y" — accept either form.
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
```

- [ ] **Step 2: Run the tests**

Run:

```bash
go test ./internal/batbin/...
```

Expected: PASS for both `TestManifestMatchesBinary` and `TestEmbeddedTag`.

If `TestManifestMatchesBinary` fails, the sha256 in `BAT_VERSION` (Task 1, Step 2) was wrong. Re-compute and update the manifest. If `TestEmbeddedTag` fails, the tag field is missing or empty — fix `BAT_VERSION`.

- [ ] **Step 3: Sanity-check by deliberately breaking the manifest, then reverting**

Temporarily change one hex character in `BAT_VERSION`'s `sha256:` line, run `go test ./internal/batbin/...`, confirm `TestManifestMatchesBinary` fails with the descriptive error message, then restore the original sha. This proves the test is wired up correctly.

- [ ] **Step 4: Commit**

```bash
git add internal/batbin/manifest_test.go
git commit -m "Test that BAT_VERSION sha256 matches the committed bat binary"
```

---

## Task 4: Wire `EmbeddedTag()` fallback into `printVersion` and `Describe`

**Files:**
- Modify: `cmd/volt/main.go` (the `printVersion` function near line 78)
- Modify: `internal/batbin/batbin.go` (the `Describe` function near the bottom)

`bat -v` is the runtime source of truth, but if extraction fails or the binary cannot run, the banner should still show *which* version of bat we *intended* to ship.

- [ ] **Step 1: Modify `cmd/volt/main.go::printVersion`**

Locate this function:

```go
func printVersion() {
	fmt.Printf("volt %s\n", voltVersion)
	path, _, err := batbin.EnsureInstalled()
	if err != nil {
		fmt.Printf("bat  unavailable (%v)\n", err)
		return
	}
	v := batbin.Version(path)
	if v == "" {
		v = "unknown"
	}
	fmt.Printf("bat  %s (bundled, %s)\n", v, path)
}
```

Replace it with:

```go
func printVersion() {
	fmt.Printf("volt %s\n", voltVersion)
	path, _, err := batbin.EnsureInstalled()
	if err != nil {
		fmt.Printf("bat  %s (embedded, %v)\n", batbin.EmbeddedTag(), err)
		return
	}
	v := batbin.Version(path)
	if v == "" {
		v = batbin.EmbeddedTag()
	}
	fmt.Printf("bat  %s (bundled, %s)\n", v, path)
}
```

- [ ] **Step 2: Modify `internal/batbin/batbin.go::Describe`**

Locate this function at the bottom of the file:

```go
func Describe() string {
	if !Embedded() {
		return "bat (unbundled — run `make build`)"
	}
	_, v := resolve()
	if v == "" {
		return "bat (bundled)"
	}
	return fmt.Sprintf("bat %s (bundled)", v)
}
```

Replace the `if v == ""` branch so it falls back to the manifest tag:

```go
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
```

- [ ] **Step 3: Build volt and test the version output**

```bash
go build -o /tmp/volt-test ./cmd/volt
/tmp/volt-test version
```

Expected: two lines, e.g.

```
volt dev
bat  <tag> (bundled, /home/<user>/.cache/volt/bat)
```

The `<tag>` should match what's in `BAT_VERSION`. Clean up: `rm /tmp/volt-test`.

- [ ] **Step 4: Run all tests**

```bash
go test ./...
```

Expected: PASS across all packages.

- [ ] **Step 5: Commit**

```bash
git add cmd/volt/main.go internal/batbin/batbin.go
git commit -m "Fall back to EmbeddedTag when bat -v is unavailable"
```

---

## Task 5: Add `update-bat` and `verify-bat` Makefile targets

**Files:**
- Modify: `Makefile`

Add new targets without removing the old ones yet — keep the build working through this task.

- [ ] **Step 1: Append the new targets to the Makefile**

At the end of the Makefile (after the `clean` target), append:

```make
MANIFEST := internal/batbin/BAT_VERSION

## update-bat: download a new upstream bat release into the embed slot
update-bat:
ifndef VERSION
	$(error VERSION is required, e.g. make update-bat VERSION=v0.10.0)
endif
	@URL="https://github.com/tshakalekholoane/bat/releases/download/$(VERSION)/bat"; \
	 SHA_URL="$$URL.sha256"; \
	 echo "→ Fetching bat $(VERSION) from $$URL…"; \
	 curl -fsSL -o $(EMBED_BIN).tmp "$$URL"; \
	 echo "→ Fetching expected sha256…"; \
	 EXPECTED=$$(curl -fsSL "$$SHA_URL" | awk '{print $$1}'); \
	 ACTUAL=$$(shasum -a 256 $(EMBED_BIN).tmp | awk '{print $$1}'); \
	 if [ -n "$$EXPECTED" ] && [ "$$EXPECTED" != "$$ACTUAL" ]; then \
	   echo "✗ sha256 mismatch: expected=$$EXPECTED actual=$$ACTUAL"; \
	   rm -f $(EMBED_BIN).tmp; \
	   exit 1; \
	 fi; \
	 if [ -z "$$EXPECTED" ]; then \
	   echo "⚠ upstream did not publish $$SHA_URL; recording locally computed sha256"; \
	 fi; \
	 mv $(EMBED_BIN).tmp $(EMBED_BIN); \
	 chmod 0755 $(EMBED_BIN); \
	 DATE=$$(date -u +%Y-%m-%d); \
	 printf "tag: %s\nsha256: %s\nurl: %s\nfetched: %s\n" \
	   "$(VERSION)" "$$ACTUAL" "$$URL" "$$DATE" > $(MANIFEST); \
	 echo "→ Done. Review $(MANIFEST), run \`go test ./internal/batbin/...\`, and commit."

## verify-bat: confirm the committed binary matches the manifest sha256
verify-bat:
	@MANIFEST_SHA=$$(grep '^sha256:' $(MANIFEST) | awk '{print $$2}'); \
	 ACTUAL_SHA=$$(shasum -a 256 $(EMBED_BIN) | awk '{print $$1}'); \
	 if [ "$$MANIFEST_SHA" = "$$ACTUAL_SHA" ]; then \
	   echo "✓ $(EMBED_BIN) matches $(MANIFEST)"; \
	 else \
	   echo "✗ sha256 mismatch: manifest=$$MANIFEST_SHA actual=$$ACTUAL_SHA"; \
	   exit 1; \
	 fi
```

Also add `update-bat` and `verify-bat` to the `.PHONY` declaration at the top of the file. Locate:

```make
.PHONY: all build bat embed run tidy clean submodule
```

Change to:

```make
.PHONY: all build bat embed run tidy clean submodule update-bat verify-bat
```

(The old phony entries stay until Task 6 deletes the corresponding targets.)

- [ ] **Step 2: Verify `verify-bat` passes against the existing manifest**

Run:

```bash
make verify-bat
```

Expected: `✓ internal/batbin/batdata/bat matches internal/batbin/BAT_VERSION`. If it fails, the sha256 in `BAT_VERSION` is wrong — fix it before continuing.

- [ ] **Step 3: Verify `update-bat` errors loudly without `VERSION`**

Run:

```bash
make update-bat
```

Expected: an error message containing `VERSION is required, e.g. make update-bat VERSION=v0.10.0` and a non-zero exit code.

- [ ] **Step 4: (Skip the live `update-bat` test)**

Do **not** run `make update-bat VERSION=…` end-to-end here. We don't want to accidentally bump the bundled version as part of this refactor. The runbook (Task 10) documents that workflow and the user will run it deliberately when preparing the next release.

- [ ] **Step 5: Commit**

```bash
git add Makefile
git commit -m "Add update-bat and verify-bat Makefile targets"
```

---

## Task 6: Remove the old build-from-source pipeline from the Makefile

**Files:**
- Modify: `Makefile`

Delete every line that exists only to build `bat` from the submodule. After this task, `make build` compiles only volt.

- [ ] **Step 1: Edit the Makefile**

Open `Makefile` and make the following changes.

1. Delete these variable definitions:

```make
BAT_DIR      := third_party/bat
BAT_BUILD    := $(BAT_DIR)/bin/bat
EMBED_DIR    := internal/batbin/batdata
EMBED_BIN    := $(EMBED_DIR)/bat
```

…and re-add only the two we still need (since `update-bat` / `verify-bat` reference them):

```make
EMBED_DIR := internal/batbin/batdata
EMBED_BIN := $(EMBED_DIR)/bat
```

2. Delete the `BAT_TAG`, `VOLT_TAG`, `BAT_LDFLAGS`, and `VOLT_LDFLAGS` lines. Re-add only `VOLT_TAG` and `VOLT_LDFLAGS` (volt itself still needs them):

```make
VOLT_TAG     := $(shell git describe --always --dirty --tags 2>/dev/null || echo dev)
VOLT_LDFLAGS := -X main.voltVersion=$(VOLT_TAG)
```

3. Update `.PHONY` to remove the dead targets:

```make
.PHONY: all build run tidy clean update-bat verify-bat
```

4. Change `build` to no longer depend on `embed`:

```make
build:
	@mkdir -p bin
	go build -ldflags="$(VOLT_LDFLAGS)" -o bin/$(BIN) ./cmd/volt
```

5. Delete these targets entirely:

```make
embed: $(EMBED_BIN)

$(EMBED_BIN): $(BAT_BUILD)
	@mkdir -p $(EMBED_DIR)
	cp $(BAT_BUILD) $(EMBED_BIN)

bat: $(BAT_BUILD)

$(BAT_BUILD): | submodule
	@mkdir -p $(BAT_DIR)/bin
	cd $(BAT_DIR) && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
		go build -ldflags="$(BAT_LDFLAGS)" -o bin/bat .

submodule:
	@if [ ! -f $(BAT_DIR)/go.mod ]; then \
		echo "Initialising bat submodule…"; \
		git submodule update --init --recursive $(BAT_DIR); \
	fi
```

6. Update `clean` so it no longer touches the now-tracked binary or the deleted submodule build dir:

```make
clean:
	rm -rf bin
```

After these edits, the Makefile should contain only: variable defs (`BIN`, `EMBED_DIR`, `EMBED_BIN`, `MANIFEST`, `VOLT_TAG`, `VOLT_LDFLAGS`), `.PHONY`, and the targets `all`, `build`, `run`, `tidy`, `clean`, `update-bat`, `verify-bat`.

- [ ] **Step 2: Verify `make build` still works**

Run:

```bash
make clean
make build
ls -l bin/volt
```

Expected: `bin/volt` exists and is executable. Build is fast (no `bat` cross-compile).

- [ ] **Step 3: Verify `verify-bat` still passes**

```bash
make verify-bat
```

Expected: `✓ internal/batbin/batdata/bat matches internal/batbin/BAT_VERSION`.

- [ ] **Step 4: Run all tests**

```bash
go test ./...
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add Makefile
git commit -m "Remove submodule build pipeline from Makefile"
```

---

## Task 7: Remove the submodule, `.gitmodules`, and stale `.gitignore` lines

**Files:**
- Delete: `.gitmodules`
- Delete: `third_party/bat` (submodule reference)
- Modify: `.gitignore`

This is the destructive step. Once committed, fresh clones will not attempt to fetch the submodule.

- [ ] **Step 1: Confirm the submodule worktree state**

Run:

```bash
git submodule status
ls -la third_party/bat 2>/dev/null
```

If `third_party/bat` has tracked content (e.g. a populated worktree), de-init first:

```bash
git submodule deinit -f third_party/bat
```

It is fine if the directory is already empty/uninitialized.

- [ ] **Step 2: Remove the submodule from git's index**

Run:

```bash
git rm -f third_party/bat
```

This removes the gitlink entry from the index. If git complains that the directory contains modifications, investigate before forcing — it should be a single gitlink, no modifications.

- [ ] **Step 3: Delete `.gitmodules`**

```bash
git rm -f .gitmodules
```

- [ ] **Step 4: Remove the now-stale lines from `.gitignore`**

The current `.gitignore` is:

```
/bin/
third_party/bat/bin/
internal/batbin/batdata/bat
```

Edit it to:

```
/bin/
```

The other two lines must go: `third_party/bat/` no longer exists, and `internal/batbin/batdata/bat` is now intentionally tracked.

- [ ] **Step 5: Clean up the residual submodule git metadata**

Run:

```bash
rm -rf .git/modules/third_party/bat
```

This removes git's cached submodule metadata. It is safe — the submodule is gone.

- [ ] **Step 6: Verify the working tree is clean of submodule traces**

Run:

```bash
git status
git ls-files | grep -E '^(third_party|.gitmodules)' || echo "✓ no residual submodule files in index"
ls third_party 2>/dev/null || echo "✓ third_party/ directory is gone"
```

Expected: the staged changes are exactly `.gitignore` (modified), `.gitmodules` (deleted), `third_party/bat` (deleted). No residual files. `third_party/` either does not exist or is empty.

If `third_party/` exists but is empty, also remove it:

```bash
rmdir third_party
```

- [ ] **Step 7: Build and test once more to confirm nothing else depended on the submodule**

```bash
make clean
make build
go test ./...
```

Expected: build succeeds, all tests pass.

- [ ] **Step 8: Commit**

```bash
git add .gitignore
git commit -m "Drop third_party/bat submodule and .gitmodules"
```

(`git rm` already staged the deletions; this commit captures `.gitignore` plus the staged removals.)

---

## Task 8: Update the GitHub Actions workflow

**Files:**
- Modify: `.github/workflows/go.yml`

The current workflow does not call `git submodule update --init`, but `actions/checkout@v4` defaults to *not* fetching submodules, which happens to be what we want now. We will pin that explicitly so the intent is documented.

- [ ] **Step 1: Pin `submodules: false` in the checkout step**

Open `.github/workflows/go.yml`. Locate:

```yaml
      - uses: actions/checkout@v4
```

Change to:

```yaml
      - uses: actions/checkout@v4
        with:
          submodules: false
```

The rest of the workflow stays the same. The `Build` step (`go build -v ./...`) and `Test` step (`go test -v ./...`) work without any submodule, and `go test ./...` transitively runs `TestManifestMatchesBinary`, satisfying the manifest-matches-binary CI check.

- [ ] **Step 2: Locally simulate the CI build to confirm**

Run:

```bash
go build -v ./...
go test -v ./...
```

Expected: both succeed.

- [ ] **Step 3: Commit**

```bash
git add .github/workflows/go.yml
git commit -m "Pin submodules: false in CI checkout step"
```

---

## Task 9: Update `README.md`

**Files:**
- Modify: `README.md`

Replace the install instructions, the bundling explanation, and the "Vendored project" section to reflect the new manifest-driven workflow.

- [ ] **Step 1: Update "Install & run"**

Locate this section:

```markdown
## Install & run

\`\`\`sh
git clone --recurse-submodules https://github.com/dejanvujkov/volt.git
cd volt
make build        # compiles bat, embeds it, compiles volt
./bin/volt        # launches the TUI — bat is extracted on first run
\`\`\`

That's it. There is no separate step to install or copy `bat` anywhere.
If you forgot `--recurse-submodules`, just run `make build` — the Makefile
initialises the submodule automatically.
```

Replace with:

```markdown
## Install & run

\`\`\`sh
git clone https://github.com/dejanvujkov/volt.git
cd volt
make build        # compiles volt with the bundled bat binary baked in
./bin/volt        # launches the TUI — bat is extracted on first run
\`\`\`

That's it. The `bat` binary ships pre-built inside this repo
(`internal/batbin/batdata/bat`); the build never has to compile it.
On first run, volt extracts that binary to `~/.cache/volt/bat`.
```

- [ ] **Step 2: Replace "How the bundling works"**

Locate the section starting with `## How the bundling works` and ending before `## Vendored project`. Replace its body (keep the heading) with:

```markdown
## How the bundling works

1. The upstream `tshakalekholoane/bat` binary is committed to this repo
   at `internal/batbin/batdata/bat`. A plain-text manifest at
   `internal/batbin/BAT_VERSION` records the upstream tag, sha256,
   source URL, and the date it was fetched.
2. `internal/batbin/embed.go` declares `//go:embed batdata`, so the
   binary becomes part of the `volt` executable at compile time.
3. `internal/batbin/version.go` declares `//go:embed BAT_VERSION`, so the
   manifest tag is also available at compile time via
   `batbin.EmbeddedTag()` — used as a fallback for the banner when the
   extracted binary cannot be executed for any reason.
4. At runtime, `batbin.EnsureInstalled` writes the embedded binary to
   `~/.cache/volt/bat` (atomically, via a temp file + rename) and
   chmods it `0755`. Subsequent runs detect the cached copy and reuse
   it. A size mismatch after a `volt` upgrade triggers a re-extract.
5. `batbin.Version` runs the resolved binary with `-v` and parses
   `bat <tag>` out of stdout; the TUI banner renders that tag, falling
   back to `EmbeddedTag()` if the runtime invocation fails.

### Upgrading the bundled `bat`

Upgrading is a deliberate, manual step performed during volt release
prep. The full checklist lives at
[`docs/UPGRADING-BAT.md`](docs/UPGRADING-BAT.md). The mechanical part
is:

\`\`\`sh
make update-bat VERSION=v0.10.0
\`\`\`

This downloads the upstream release binary, verifies its sha256, swaps
the embed slot, and rewrites `BAT_VERSION`. `make verify-bat` re-checks
that the committed binary still matches the manifest at any later
point.
```

- [ ] **Step 3: Replace or remove "Vendored project"**

Locate:

```markdown
## Vendored project

The canonical `bat` sources are checked in under
[`third_party/bat/`](third_party/bat). volt re-implements the sysfs reads
directly; writes that need systemd (persist/reset) defer to the bundled
binary so its well-tested behaviour is preserved.
```

Replace with:

```markdown
## Upstream project

The upstream tool is [`tshakalekholoane/bat`][bat]. volt re-implements
the sysfs reads directly; writes that need systemd (persist/reset)
defer to the bundled binary so its well-tested behaviour is preserved.
The exact upstream version currently shipped is recorded in
[`internal/batbin/BAT_VERSION`](internal/batbin/BAT_VERSION).
```

- [ ] **Step 4: Skim the rest of the README for stale references**

Search for `submodule`, `third_party`, `recurse`, `BAT_TAG`:

```bash
grep -nE '(submodule|third_party|recurse|BAT_TAG)' README.md || echo "✓ no stale references"
```

Expected: `✓ no stale references`. If any remain, fix them.

- [ ] **Step 5: Commit**

```bash
git add README.md
git commit -m "Document the committed bat binary workflow in README"
```

---

## Task 10: Add `docs/UPGRADING-BAT.md` runbook

**Files:**
- Create: `docs/UPGRADING-BAT.md`

The runbook is the user-facing checklist for bumping bundled `bat` during release prep. The full text is the one approved during brainstorming.

- [ ] **Step 1: Create `docs/UPGRADING-BAT.md`**

Write the file with this exact content:

````markdown
# Upgrading the bundled `bat` binary

volt embeds a prebuilt `bat` binary at `internal/batbin/batdata/bat`.
The version is recorded in `internal/batbin/BAT_VERSION`. This document
is the checklist for pulling in a new upstream release.

Do this only as part of preparing a new volt release. Do not bump
`bat` opportunistically.

## 1. Pick the upstream version

1. Visit https://github.com/tshakalekholoane/bat/releases.
2. Read the release notes for **every release** between the currently
   embedded version (in `BAT_VERSION`) and the target version.
3. Note any breaking changes to:
   - CLI flags or subcommand names volt invokes (`bat persist`,
     `bat reset`, `bat -v`, `bat threshold …`).
   - Output format of `bat -v` (volt parses this in `batbin.Version`).
   - sysfs paths or kernel assumptions volt also reads directly.

## 2. Run the update target

```sh
make update-bat VERSION=v0.10.0
```

This downloads the release asset, verifies the published sha256,
replaces `internal/batbin/batdata/bat`, and rewrites
`internal/batbin/BAT_VERSION`.

If checksum verification fails, **stop**. Do not commit. Investigate
before retrying.

## 3. Sanity-check the binary

```sh
./internal/batbin/batdata/bat -v          # should print the new tag
file ./internal/batbin/batdata/bat        # ELF 64-bit LSB, x86_64
make verify-bat                           # re-checks committed sha256
```

## 4. Adapt volt code to upstream changes

Walk through your notes from step 1 and update volt where needed:

- **CLI wrappers** in `internal/…` that shell out to `bat` — adjust
  flags/args.
- **Version parsing** in `internal/batbin/batbin.go` (`Version`) —
  adjust if upstream changed `bat -v` output.
- **Banner string** in the TUI if the version line moved.
- **Tests** in `internal/…/_test.go` that assert on bat's output or
  behaviour.

If nothing in volt needed to change, that's fine — common case for
patch releases.

## 5. Build and smoke-test volt

```sh
make clean
make build
./bin/volt version             # banner shows new bat tag
./bin/volt                     # TUI launches; threshold + capacity render
./bin/volt status              # CLI subcommand still works
```

Run the test suite:

```sh
go test ./...
```

On a real ASUS/ThinkPad host (or a VM where you can mock sysfs), test:

- `s` — set a new threshold.
- `p` — `sudo bat persist`.
- `R` — `sudo bat reset`.

If any of those broke, fix them before continuing.

## 6. Update README & changelog

- In `README.md`, the banner art shows `powered by bundled bat <version>`
  — update if hard-coded. (If it reads from `bat -v` at runtime, no
  change needed.)
- Add a line to release notes / `CHANGELOG.md`:
  `Bumped bundled bat to v0.10.0.`
- Mention any user-visible behavioural changes.

## 7. Commit

```sh
git add internal/batbin/batdata/bat internal/batbin/BAT_VERSION
git add <any volt source files you changed>
git commit -m "Bump bundled bat to v0.10.0"
```

The PR diff will show:

- A binary file change (large, opaque — expected).
- The `BAT_VERSION` manifest change (reviewable: tag, sha256, URL,
  date).
- Any volt code changes.

## 8. Tag the volt release

Follow your normal volt release process. The bundled `bat` version is
now permanently associated with this volt tag in git history.

## Rollback

If a release ships with a broken `bat`, revert the upgrade commit:

```sh
git revert <commit-sha>
```

Then re-tag and re-release. Because the binary lives in git, rollback
is instantaneous — no re-download or re-build required.
````

- [ ] **Step 2: Verify it renders sensibly**

Skim `docs/UPGRADING-BAT.md` in your editor's markdown preview (or just
re-read the raw file). Confirm code blocks are balanced and the section
numbering reads top-to-bottom.

- [ ] **Step 3: Commit**

```bash
git add docs/UPGRADING-BAT.md
git commit -m "Add UPGRADING-BAT runbook"
```

---

## Task 11: Clean up doc comments referring to the deleted submodule

**Files:**
- Modify: `cmd/volt/main.go` (top-of-file doc comment)
- Modify: `internal/batbin/batbin.go` (top-of-file doc comment)
- Modify: `internal/batbin/embed.go` (top-of-file doc comment and the comment above `//go:embed batdata`)

These doc comments mention `third_party/bat` or `make build compiles bat`. Now that the submodule is gone, they're misleading.

- [ ] **Step 1: Update `cmd/volt/main.go` package doc**

Locate the package comment block at the top:

```go
// Binary volt is a Bubble Tea TUI for managing the battery charge-control
// threshold on Linux laptops. It is a thin, interactive front-end over the
// same sysfs attributes driven by the vendored tshakalekholoane/bat project
// (see third_party/bat in this repository). The upstream `bat` binary is
// bundled directly into the volt executable via //go:embed and extracted
// to the user cache directory on first run, so end users never have to
// build or install bat themselves.
package main
```

Replace with:

```go
// Binary volt is a Bubble Tea TUI for managing the battery charge-control
// threshold on Linux laptops. It is a thin, interactive front-end over the
// same sysfs attributes driven by the upstream tshakalekholoane/bat tool.
// The upstream `bat` binary is bundled directly into the volt executable
// via //go:embed (committed at internal/batbin/batdata/bat with metadata
// at internal/batbin/BAT_VERSION) and extracted to the user cache directory
// on first run, so end users never have to build or install bat themselves.
package main
```

- [ ] **Step 2: Update `internal/batbin/batbin.go` package doc**

Locate the package comment block:

```go
// Package batbin bundles the upstream tshakalekholoane/bat binary inside
// the volt executable and exposes helpers for extracting it to the user
// cache directory on first run.
//
// volt is designed so that end users never have to build or install `bat`
// themselves: `make build` compiles bat, embeds it here via //go:embed,
// and at runtime EnsureInstalled drops the binary into
// $XDG_CACHE_HOME/volt/bat with an executable bit set.
package batbin
```

Replace with:

```go
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
```

- [ ] **Step 3: Update `internal/batbin/embed.go`**

Locate:

```go
//go:build linux

package batbin

import "embed"

// batdataFS holds the files under ./batdata at build time. The `make build`
// target drops the freshly compiled upstream `bat` binary into
// batdata/bat; see batbin.go for the extraction logic.
//
//go:embed batdata
var batdataFS embed.FS
```

Replace the comment above `//go:embed batdata` so it reflects the new
workflow:

```go
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
```

- [ ] **Step 4: Build and test**

```bash
go build ./...
go test ./...
```

Expected: build and tests pass.

- [ ] **Step 5: Commit**

```bash
git add cmd/volt/main.go internal/batbin/batbin.go internal/batbin/embed.go
git commit -m "Update doc comments for the committed-binary workflow"
```

---

## Task 12: Final verification

**Files:** none (verification only)

- [ ] **Step 1: Confirm the working tree and git index are clean**

```bash
git status
```

Expected: `nothing to commit, working tree clean`.

- [ ] **Step 2: Confirm there are no remaining references to the submodule**

```bash
grep -rnE '(third_party/bat|\.gitmodules|recurse-submodules|BAT_LDFLAGS|BAT_TAG)' \
  --exclude-dir=.git \
  --exclude-dir=docs/superpowers \
  . || echo "✓ no stale references"
```

Expected: `✓ no stale references`. Matches inside `docs/superpowers/` (the spec and this plan) are fine and excluded.

- [ ] **Step 3: Full clean build + test cycle**

```bash
make clean
make build
make verify-bat
go test ./...
./bin/volt version
```

Expected:
- `make build` produces `bin/volt` without invoking any `bat` cross-compile.
- `make verify-bat` prints `✓ ...`.
- `go test ./...` is all green, including `TestManifestMatchesBinary`.
- `./bin/volt version` prints `volt <tag>` and `bat <tag> (bundled, /home/<user>/.cache/volt/bat)` where `<tag>` matches `BAT_VERSION`.

- [ ] **Step 4: Confirm the commit log tells a clean story**

```bash
git log --oneline origin/master..HEAD
```

Expected: ~10 commits, one per task, each message describing exactly what changed. No merge commits, no `WIP`.

- [ ] **Step 5: Stop here**

Do **not** push, tag, or open a PR. The user will review the branch and trigger release/PR steps themselves.

---

## Self-review notes

- **Spec coverage:** every section of the spec maps to at least one task — manifest (T1), version.go (T2), test (T3), version fallback (T4), Makefile add (T5) and remove (T6), submodule deletion (T7), CI (T8), README (T9), runbook (T10), doc comments (T11), final verification (T12).
- **Placeholders:** none. Every code/command step contains real content. The only placeholder-shaped values are `<TAG>` etc. in Task 1, which are *deliberately* derived at execution time from the binary itself.
- **Type consistency:** `EmbeddedTag()` is the same name in `version.go` (T2), the test (T3), and both consumers (T4). `manifestBytes` is the same variable referenced by `version.go` and `manifest_test.go`. `MANIFEST` is the same Make variable referenced across both `update-bat` and `verify-bat`.
- **Order safety:** new code is added before old code is removed; tests are added before the build pipeline they cover is dismantled; the submodule is deleted only after every consumer has switched off it.
