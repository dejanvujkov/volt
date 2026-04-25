# Vendor `bat` as a committed binary (replace git submodule)

**Status:** Design approved 2026-04-25
**Owner:** Dejan Vujkov

## Summary

Replace the current `third_party/bat` git-submodule-plus-build-from-source
pipeline with a **committed prebuilt `bat` binary plus a plain-text version
manifest**. Each upgrade of the bundled `bat` becomes a deliberate, manually
triggered step performed during volt release prep, not an opportunistic
dependency bump.

## Goals

- Remove the git submodule and the requirement to compile upstream `bat` as
  part of `make build`.
- Treat `bat` as a vendored release artifact, not a dependency: each upgrade
  is a discrete, reviewable event in volt's git history.
- Provide a single Make target that automates the mechanical parts of
  fetching and verifying a new upstream release.
- Provide a written runbook so the upgrade process is documented and
  repeatable.

## Non-goals

- Multi-architecture support. Only `linux/amd64` is supported, matching the
  current Makefile's cross-compile target.
- Automatic upstream version tracking (Renovate, Dependabot, etc.). Upgrades
  are intentionally manual.
- Building `bat` from source on the user's machine. The committed binary is
  the only artifact shipped.
- Changes to volt's runtime extraction logic (`EnsureInstalled`) or its TUI
  surface.

## Architecture overview

The embed layer (`internal/batbin`) keeps its current shape — `//go:embed
batdata` and `EnsureInstalled` are unchanged at runtime. What changes is
**how the binary gets into `internal/batbin/batdata/bat`**: instead of being
built from a submodule on every `make build`, it is checked into git, and a
`make update-bat VERSION=…` target replaces it during release prep.

After this change:

- **At build time:** `make build` only compiles volt. No `bat` build, no
  submodule init, no Go cross-compile of upstream.
- **At update time:** `make update-bat VERSION=v0.10.0` downloads the
  release asset from upstream, verifies its sha256 against upstream's
  published checksum, writes it to `internal/batbin/batdata/bat`, and
  rewrites the manifest at `internal/batbin/BAT_VERSION`.
- **At runtime:** unchanged — `EnsureInstalled` extracts the embedded
  binary to `~/.cache/volt/bat`; `Version` runs `bat -v` and parses the tag.

The submodule, the `third_party/bat` directory, and the `bat` / `embed` /
`submodule` Makefile targets all go away. `BAT_LDFLAGS`, `BAT_TAG`, and the
cross-compile invocation are deleted, since upstream's release binary
already has its version baked in.

## Repository layout

### Added

```
internal/batbin/
├── batbin.go              (unchanged)
├── embed.go               (unchanged)
├── version.go             (NEW — embeds BAT_VERSION, exposes EmbeddedTag)
├── manifest_test.go       (NEW — TestManifestMatchesBinary)
├── batdata/
│   └── bat                (committed binary, linux/amd64, ~3 MB)
└── BAT_VERSION            (NEW — plain-text manifest)

docs/UPGRADING-BAT.md      (NEW — upgrade runbook)
```

### `BAT_VERSION` format

Plain text, written verbatim by `make update-bat`:

```
tag: v0.10.0
sha256: 9f3c2b...e1
url: https://github.com/tshakalekholoane/bat/releases/download/v0.10.0/bat
fetched: 2026-04-25
```

Fields:

- **`tag`** — upstream git tag. Must match what `bat -v` reports at runtime.
- **`sha256`** — hex digest of the committed binary. Self-check for
  CI / `make verify-bat`.
- **`url`** — exact download URL used. Auditable in PR diffs; surfaces
  upstream asset-naming changes immediately.
- **`fetched`** — UTC date `make update-bat` ran. Provides historical context
  when reading old volt releases.

### Removed

- `.gitmodules`
- `third_party/bat/` (submodule reference and any worktree)
- `.gitignore` lines for `third_party/bat/bin/` and
  `internal/batbin/batdata/bat` (the binary is now intentionally committed)

## Code changes

### `internal/batbin/version.go` (new)

Embeds `BAT_VERSION` at compile time and exposes the embedded tag:

```go
package batbin

import (
    _ "embed"
    "strings"
)

//go:embed BAT_VERSION
var manifestBytes []byte

// EmbeddedTag returns the upstream bat tag recorded in BAT_VERSION at
// build time. If the manifest is malformed, returns "unknown".
func EmbeddedTag() string {
    for _, line := range strings.Split(string(manifestBytes), "\n") {
        if v, ok := strings.CutPrefix(line, "tag:"); ok {
            return strings.TrimSpace(v)
        }
    }
    return "unknown"
}
```

`batbin.go` and `embed.go` are not modified. `EnsureInstalled` and `Version`
behave exactly as before.

### `cmd/volt/…` (banner and `volt version`)

Where the TUI banner and `volt version` currently call `batbin.Version()`
(which runs `bat -v` against the extracted binary), they should fall back
to `batbin.EmbeddedTag()` if `Version()` returns an error or empty string.
This guarantees the banner has a real answer even if extraction fails.

## Makefile changes

### Removed targets and variables

- `bat`, `embed`, `submodule` targets
- `BAT_DIR`, `BAT_BUILD`, `EMBED_DIR` (kept for `update-bat`), `BAT_TAG`,
  `BAT_LDFLAGS`
- `embed` dependency from the `build` target
- `$(BAT_BUILD)` and `$(EMBED_BIN)` rules that built and copied the source-
  built bat
- `$(EMBED_BIN)` removal from the `clean` target (the binary is now tracked)

### Added targets

```make
EMBED_DIR := internal/batbin/batdata
EMBED_BIN := $(EMBED_DIR)/bat
MANIFEST  := internal/batbin/BAT_VERSION

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
	 if [ "$$EXPECTED" != "$$ACTUAL" ]; then \
	   echo "✗ sha256 mismatch: expected=$$EXPECTED actual=$$ACTUAL"; \
	   rm -f $(EMBED_BIN).tmp; \
	   exit 1; \
	 fi; \
	 mv $(EMBED_BIN).tmp $(EMBED_BIN); \
	 chmod 0755 $(EMBED_BIN); \
	 DATE=$$(date -u +%Y-%m-%d); \
	 printf "tag: %s\nsha256: %s\nurl: %s\nfetched: %s\n" \
	   "$(VERSION)" "$$ACTUAL" "$$URL" "$$DATE" > $(MANIFEST); \
	 echo "→ Done. Review $(MANIFEST) and commit."

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

If upstream does not publish a `bat.sha256` sidecar at the URL pattern
above, the `update-bat` target will be adjusted to compute the digest
locally and the runbook will document the change. The implementation phase
will confirm the actual asset names against upstream's release page before
finalising.

The atomic `.tmp` + `mv` pattern guarantees that a failed download or
checksum mismatch leaves `internal/batbin/batdata/bat` and `BAT_VERSION`
untouched.

## Tests

### `internal/batbin/manifest_test.go` (new)

```go
func TestManifestMatchesBinary(t *testing.T) {
    // Parse sha256 line from BAT_VERSION.
    // Compute sha256 of internal/batbin/batdata/bat.
    // Assert they match.
}
```

This catches "someone edited the binary without re-running `make
update-bat`" in CI.

Existing tests in `internal/batbin/` (`EnsureInstalled`, `Version` parsing)
remain unchanged — the embed mechanism is the same.

## CI changes (`.github/workflows/go.yml`)

- Remove `git submodule update --init --recursive` step.
- Remove any "build bat" step that invoked `make bat` or compiled
  `third_party/bat`.
- The `go test ./...` step transitively runs `TestManifestMatchesBinary`,
  which fulfils the role of an explicit `make verify-bat` CI step. No
  separate verification step is added.
- `make build` now runs faster because there is no upstream cross-compile.

## README updates

- "Install & run" no longer needs `--recurse-submodules`.
- "How the bundling works" rewritten: replace the build-from-submodule steps
  with "binary is committed to `internal/batbin/batdata/bat`; manifest at
  `internal/batbin/BAT_VERSION` records version, sha256, source URL, and
  fetch date."
- "Vendored project" section either removed or rewritten to point at the
  manifest rather than the (deleted) source tree.

## Migration plan

Implement in this order so the repo never enters a broken state and each
commit leaves the build green:

1. **Add new artifacts** — commit the prebuilt `bat` binary at
   `internal/batbin/batdata/bat` (sourced from upstream's release matching
   the tag the submodule currently points at, so the binary is
   functionally identical to today's build), write `BAT_VERSION` with the
   matching tag/sha256/url/fetched, add `version.go` with the embed and
   `EmbeddedTag()`.
2. **Add new Makefile targets** — `update-bat` and `verify-bat`. Old
   targets remain in place.
3. **Add the new test** — `TestManifestMatchesBinary` in
   `internal/batbin/`.
4. **Switch consumers** — wire `cmd/volt/…` banner and `volt version` to
   fall back to `EmbeddedTag()` when `Version()` fails.
5. **Remove the old pipeline in one commit** — delete `.gitmodules`, delete
   `third_party/bat/`, remove `bat`/`embed`/`submodule` Makefile targets and
   `BAT_LDFLAGS`/`BAT_TAG` plumbing, drop submodule-related lines from
   `.gitignore`, drop the submodule step from
   `.github/workflows/go.yml`.
6. **Update docs** — rewrite README sections, add
   `docs/UPGRADING-BAT.md`.

## Error handling

- **`update-bat` failures** (network, 404, checksum mismatch, disk full):
  the target exits non-zero before any `mv` runs. The committed binary and
  manifest stay intact.
- **`EmbeddedTag` on malformed manifest:** returns `"unknown"`. The runtime
  banner falls back to `bat -v`, which is the source of truth anyway.
- **`TestManifestMatchesBinary` failing in CI:** indicates someone edited
  `internal/batbin/batdata/bat` without re-running `make update-bat`. Fix
  by re-running the target with the correct version.

## Upgrade runbook (delivered as `docs/UPGRADING-BAT.md`)

The runbook is a deliverable of this design. Its full contents:

> # Upgrading the bundled `bat` binary
>
> volt embeds a prebuilt `bat` binary at `internal/batbin/batdata/bat`. The
> version is recorded in `internal/batbin/BAT_VERSION`. This document is
> the checklist for pulling in a new upstream release.
>
> Do this only as part of preparing a new volt release. Do not bump `bat`
> opportunistically.
>
> ## 1. Pick the upstream version
>
> 1. Visit https://github.com/tshakalekholoane/bat/releases.
> 2. Read the release notes for **every release** between the currently
>    embedded version (in `BAT_VERSION`) and the target version.
> 3. Note any breaking changes to:
>    - CLI flags or subcommand names volt invokes (`bat persist`,
>      `bat reset`, `bat -v`, `bat threshold …`).
>    - Output format of `bat -v` (volt parses this in `batbin.Version`).
>    - sysfs paths or kernel assumptions volt also reads directly.
>
> ## 2. Run the update target
>
> ```sh
> make update-bat VERSION=v0.10.0
> ```
>
> This downloads the release asset, verifies the published sha256, replaces
> `internal/batbin/batdata/bat`, and rewrites `internal/batbin/BAT_VERSION`.
>
> If checksum verification fails, **stop**. Do not commit. Investigate
> before retrying.
>
> ## 3. Sanity-check the binary
>
> ```sh
> ./internal/batbin/batdata/bat -v          # should print the new tag
> file ./internal/batbin/batdata/bat        # ELF 64-bit LSB, x86_64
> make verify-bat                           # re-checks committed sha256
> ```
>
> ## 4. Adapt volt code to upstream changes
>
> Walk through your notes from step 1 and update volt where needed:
>
> - **CLI wrappers** in `internal/…` that shell out to `bat` — adjust
>   flags/args.
> - **Version parsing** in `internal/batbin/batbin.go` (`Version`) —
>   adjust the regex if upstream changed `bat -v` output.
> - **Banner string** in the TUI if the version line moved.
> - **Tests** in `internal/…/_test.go` that assert on bat's output or
>   behaviour.
>
> If nothing in volt needed to change, that's fine — common case for patch
> releases.
>
> ## 5. Build and smoke-test volt
>
> ```sh
> make clean
> make build
> ./bin/volt version             # banner shows new bat tag
> ./bin/volt                     # TUI launches; threshold + capacity render
> ./bin/volt status              # CLI subcommand still works
> ```
>
> Run the test suite:
>
> ```sh
> go test ./...
> ```
>
> On a real ASUS/ThinkPad host (or a VM where you can mock sysfs), test:
>
> - `s` — set a new threshold.
> - `p` — `sudo bat persist`.
> - `R` — `sudo bat reset`.
>
> If any of those broke, fix them before continuing.
>
> ## 6. Update README & changelog
>
> - In `README.md`, the banner art shows `powered by bundled bat <version>`
>   — update if hard-coded. (If it reads from `bat -v` at runtime, no
>   change needed.)
> - Add a line to release notes / `CHANGELOG.md`:
>   `Bumped bundled bat to v0.10.0.`
> - Mention any user-visible behavioural changes.
>
> ## 7. Commit
>
> ```sh
> git add internal/batbin/batdata/bat internal/batbin/BAT_VERSION
> git add <any volt source files you changed>
> git commit -m "Bump bundled bat to v0.10.0"
> ```
>
> The PR diff will show:
>
> - A binary file change (large, opaque — expected).
> - The `BAT_VERSION` manifest change (reviewable: tag, sha256, URL, date).
> - Any volt code changes.
>
> ## 8. Tag the volt release
>
> Follow your normal volt release process. The bundled `bat` version is
> now permanently associated with this volt tag in git history.
>
> ## Rollback
>
> If a release ships with a broken `bat`, revert the upgrade commit:
>
> ```sh
> git revert <commit-sha>
> ```
>
> Then re-tag and re-release. Because the binary lives in git, rollback is
> instantaneous — no re-download or re-build required.

## Open questions

None. All design decisions resolved during brainstorming:

- Architecture: option C (committed prebuilt binary).
- Architectures supported: `linux/amd64` only.
- Update workflow: scripted via `make update-bat VERSION=…`, no CI
  re-verification beyond `TestManifestMatchesBinary`.
- Manifest format: plain text (`BAT_VERSION`).
- Version helper: compile-time embed (`//go:embed BAT_VERSION` in
  `version.go`).
- Runbook location: `docs/UPGRADING-BAT.md`.
