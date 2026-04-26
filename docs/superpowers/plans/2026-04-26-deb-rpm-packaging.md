# Volt deb/rpm Packaging — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Publish `.deb` and `.rpm` packages of `volt` (amd64) as GitHub Release assets on every `v*` tag push, so users can install with `dpkg -i` / `rpm -i` instead of building from source.

**Architecture:** Add three files to the repo (`LICENSE`, `nfpm.yaml`, `.github/workflows/release.yml`). The new workflow triggers on tag pushes, calls the existing `make build` to compile `bin/volt`, then uses `nfpm` to wrap that single binary in `.deb` and `.rpm` packages. A `SHA256SUMS` file is generated and all three artifacts are attached to the GitHub Release. No changes to the Makefile, the existing `go.yml` workflow, or any application code.

**Tech Stack:** Go 1.26.2 (existing), [nfpm](https://nfpm.goreleaser.com/) (new — deb/rpm packager from a single YAML), GitHub Actions, [softprops/action-gh-release](https://github.com/softprops/action-gh-release).

**Spec:** [`docs/superpowers/specs/2026-04-26-deb-rpm-packaging-design.md`](../specs/2026-04-26-deb-rpm-packaging-design.md)

---

## File Structure

| File | Status | Purpose |
|------|--------|---------|
| `LICENSE` | NEW | MIT license text. Required by `nfpm.yaml` (mapped to `/usr/share/doc/volt/copyright` in installed packages); also a basic legal/distribution requirement. |
| `nfpm.yaml` | NEW | Single nfpm config that produces both `.deb` and `.rpm`. Defines package identity, runtime dependencies, and the file mappings from the repo into the installed filesystem layout. |
| `.github/workflows/release.yml` | NEW | Tag-triggered (`v*`) GitHub Actions workflow. Verifies the embedded `bat`, builds the binary via `make build`, installs a pinned `nfpm`, runs nfpm twice (once per packager), generates `SHA256SUMS`, smoke-tests package metadata, and attaches everything to a GitHub Release. |
| `README.md` | MODIFY | Add an "Install from a release" subsection alongside the existing "Install & run" build-from-source instructions. |
| `.github/workflows/go.yml` | UNCHANGED | The existing build+test workflow stays exactly as it is. |
| `Makefile` | UNCHANGED | `make build` and `make verify-bat` are reused unchanged. |
| `cmd/`, `internal/` | UNCHANGED | No application code changes. |

Each new file has a single, focused responsibility:
- `LICENSE` — declares the legal terms.
- `nfpm.yaml` — declares what a `volt` package *is*.
- `release.yml` — declares *when* and *how* packages are produced and published.

---

## Tooling Note: nfpm Version & Checksum

Several tasks below need the pinned `nfpm` version and the sha256 of its `Linux_x86_64.tar.gz` release asset. **Determine these once in Task 4 and use the results in Task 5.** Do not guess; do not use a version older than the latest stable v2.x — older nfpm versions sometimes generate packages that fail `dpkg-deb --info` on newer dpkg.

The plan refers to these values as `<NFPM_VERSION>` (e.g. `v2.43.0`) and `<NFPM_SHA256>` (the 64-char hex digest). Substitute the real values when you reach Task 5.

---

## Task 1: Add the LICENSE file

**Files:**
- Create: `LICENSE`

**Why:** `nfpm.yaml` (Task 2) maps `LICENSE` → `/usr/share/doc/volt/copyright`. Without this file, nfpm fails with `lstat LICENSE: no such file or directory` at packaging time. The MIT text below matches upstream `tshakalekholoane/bat`'s license.

- [ ] **Step 1: Confirm no LICENSE exists yet**

Run:
```sh
ls LICENSE
```
Expected: `ls: LICENSE: No such file or directory` (exit 1). If the file already exists, stop and read it before proceeding — do not overwrite an existing license.

- [ ] **Step 2: Create the LICENSE file**

Create `LICENSE` with this exact content:

```
MIT License

Copyright (c) 2026 Dejan Vujkov

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```

- [ ] **Step 3: Verify the file is well-formed**

Run:
```sh
wc -l LICENSE && head -1 LICENSE
```
Expected: line count ≥ 20 (the file is ~21 lines), and the first line is `MIT License`.

- [ ] **Step 4: Commit**

```sh
git add LICENSE
git commit -m "Add MIT LICENSE file

Required as a prerequisite for the deb/rpm packaging work; nfpm maps
this file to /usr/share/doc/volt/copyright in the installed package."
```

---

## Task 2: Create nfpm.yaml

**Files:**
- Create: `nfpm.yaml`

**Why:** This is the package definition. `nfpm` reads it twice (once with `--packager deb`, once with `--packager rpm`) and produces both formats from the same source of truth. The config below uses env-var interpolation for `version`, so the same file is used for local dry-runs (`VERSION=0.1.0-test`) and the real release (`VERSION=0.1.0`).

- [ ] **Step 1: Write a failing dry-run check**

Run:
```sh
VERSION=0.1.0-test nfpm pkg --packager deb --target /tmp/ 2>&1 | head -5
```

Expected: either `nfpm: command not found` (if nfpm isn't installed locally yet) OR an error about a missing config file like `open nfpm.yaml: no such file or directory`. This confirms there's no nfpm.yaml yet.

If `nfpm` is not installed locally, install it for development convenience:
- macOS: `brew install nfpm`
- Linux: download from https://github.com/goreleaser/nfpm/releases (any recent v2.x is fine for local dry-runs; CI pinning is handled in Task 4)

Then re-run the command and confirm the missing-config error.

- [ ] **Step 2: Create nfpm.yaml**

Create `nfpm.yaml` with this exact content:

```yaml
# nfpm config for volt — produces both .deb and .rpm from this single file.
# See docs/superpowers/specs/2026-04-26-deb-rpm-packaging-design.md.

name: volt
arch: amd64
platform: linux
version: ${VERSION}
version_schema: semver
release: "1"
maintainer: "Dejan Vujkov <vujkovdejan@gmail.com>"
description: |
  A compact Bubble Tea TUI for managing laptop battery charging thresholds
  on Linux. Wraps and bundles tshakalekholoane/bat for an interactive
  terminal interface plus a CLI mode that mirrors the original tool.
vendor: "Dejan Vujkov"
homepage: https://github.com/dejanvujkov/volt
license: MIT
section: utils
priority: optional

depends:
  - sudo
  - systemd

rpm:
  group: Applications/System

contents:
  - src: ./bin/volt
    dst: /usr/bin/volt
    file_info:
      mode: 0755

  - src: ./LICENSE
    dst: /usr/share/doc/volt/copyright
    type: doc
    file_info:
      mode: 0644

  - src: ./README.md
    dst: /usr/share/doc/volt/README.md
    type: doc
    file_info:
      mode: 0644
```

Notes on specific fields (do not change unless you understand the consequence):
- `arch: amd64` — nfpm normalizes this to `amd64` in `.deb` filenames and `x86_64` in `.rpm` filenames. Do not write `x86_64` here.
- `version: ${VERSION}` — nfpm reads this from the `VERSION` environment variable at packaging time. No quoting needed.
- `release: "1"` — RPM-only "package release" number. Quoted to prevent YAML from coercing the bare `1` to a YAML integer (nfpm wants a string).
- `priority: optional` — Debian's standard priority for non-essential utilities; lintian will warn if it's missing.
- `depends:` at top level applies to both `.deb` (as `Depends:`) and `.rpm` (as `Requires:`). Confirmed by reading nfpm v2 docs.
- `type: doc` on the LICENSE and README mappings tells nfpm these are documentation files. Debian uses this to mark them as "conffiles-style optional content"; if a sysadmin has set `path-exclude=/usr/share/doc/*` in `dpkg.cfg`, these get skipped instead of erroring.

- [ ] **Step 3: Verify the YAML parses**

Run:
```sh
VERSION=0.1.0-test nfpm pkg --packager deb --target /tmp/ 2>&1
```

Expected: a successful run that produces `/tmp/volt_0.1.0-test_amd64.deb`. No YAML-parse errors. If you see `bin/volt: no such file`, that's expected at this point — you haven't built the binary yet. Run `make build` first, then re-run.

- [ ] **Step 4: Commit**

```sh
git add nfpm.yaml
git commit -m "Add nfpm config for volt deb/rpm packaging

Single config drives both packagers via VERSION env interpolation.
Maps bin/volt to /usr/bin/volt, LICENSE to /usr/share/doc/volt/copyright,
README.md to /usr/share/doc/volt/README.md. Hard-requires sudo and systemd."
```

---

## Task 3: Local end-to-end dry-run verification

**Files:**
- (No files created or modified — this is a verification task.)

**Why:** Before wiring CI, prove the nfpm.yaml actually produces structurally-correct packages. Catches typos in paths, wrong file modes, missing description, etc., in seconds rather than after pushing a tag.

- [ ] **Step 1: Build the binary**

Run:
```sh
make build
ls -lh bin/volt
```

Expected: `bin/volt` exists, ~5-10 MB. (If you're on macOS, this still produces a Linux binary because `cmd/volt/main.go` has `//go:build linux` and `go build` will fail on non-Linux. If you're on macOS for development, run this step inside a Linux VM, container, or skip directly to Task 5 and let CI verify.)

For macOS users without a Linux dev environment: substitute the local steps in this task with `docker run --rm -v "$PWD":/src -w /src golang:1.26.2 make build` and pipe the package commands through the same container.

- [ ] **Step 2: Build both packages**

Run:
```sh
mkdir -p /tmp/volt-pkgtest
VERSION=0.1.0-test nfpm pkg --packager deb --target /tmp/volt-pkgtest/
VERSION=0.1.0-test nfpm pkg --packager rpm --target /tmp/volt-pkgtest/
ls -lh /tmp/volt-pkgtest/
```

Expected: two files exist:
- `/tmp/volt-pkgtest/volt_0.1.0-test_amd64.deb`
- `/tmp/volt-pkgtest/volt-0.1.0-test-1.x86_64.rpm`

- [ ] **Step 3: Inspect the .deb metadata**

Run:
```sh
dpkg-deb --info /tmp/volt-pkgtest/volt_0.1.0-test_amd64.deb
```

Expected output should contain (among other fields):
```
 Package: volt
 Version: 0.1.0-test
 Architecture: amd64
 Maintainer: Dejan Vujkov <vujkovdejan@gmail.com>
 Section: utils
 Priority: optional
 Depends: sudo, systemd
 Homepage: https://github.com/dejanvujkov/volt
```

Then list contents:
```sh
dpkg-deb --contents /tmp/volt-pkgtest/volt_0.1.0-test_amd64.deb
```

Expected: lines for `./usr/bin/volt`, `./usr/share/doc/volt/copyright`, `./usr/share/doc/volt/README.md`, with `volt` mode `-rwxr-xr-x` and the docs `-rw-r--r--`.

If `dpkg-deb` is not installed (macOS without homebrew dpkg): `brew install dpkg` or skip to Task 5 and verify in CI.

- [ ] **Step 4: Inspect the .rpm metadata**

Run:
```sh
rpm -qpi /tmp/volt-pkgtest/volt-0.1.0-test-1.x86_64.rpm
```

Expected output should contain:
```
Name        : volt
Version     : 0.1.0-test
Release     : 1
Architecture: x86_64
License     : MIT
URL         : https://github.com/dejanvujkov/volt
Group       : Applications/System
```

Then list contents:
```sh
rpm -qpl /tmp/volt-pkgtest/volt-0.1.0-test-1.x86_64.rpm
```

Expected: same three paths as the .deb.

If `rpm` CLI is not installed (macOS): `brew install rpm` or skip to Task 5 and verify in CI.

- [ ] **Step 5: Inspect dependencies in the .rpm explicitly**

The .rpm depends-list isn't in the default `-qpi` output. Run:
```sh
rpm -qpR /tmp/volt-pkgtest/volt-0.1.0-test-1.x86_64.rpm | grep -E "^(sudo|systemd)"
```

Expected: both `sudo` and `systemd` appear in the output. (There may also be auto-detected `rpmlib(...)` requirements — those are normal.)

- [ ] **Step 6: Clean up test artifacts**

Run:
```sh
rm -rf /tmp/volt-pkgtest
```

- [ ] **Step 7: No commit needed**

This task only verified existing files. If verification revealed problems with `nfpm.yaml`, fix them, re-run from Step 2, then go back and amend the Task 2 commit:
```sh
git add nfpm.yaml
git commit --amend --no-edit
```

---

## Task 4: Determine pinned nfpm version and sha256

**Files:**
- (No files created — this task produces two values used in Task 5.)

**Why:** Task 5's workflow downloads a pinned nfpm tarball and verifies its sha256 before extracting. Pinning protects the release pipeline from upstream nfpm changing package-format defaults between runs. This task determines the actual values to commit into the workflow.

- [ ] **Step 1: Find the latest stable nfpm v2.x release**

Open https://github.com/goreleaser/nfpm/releases in a browser, or run:
```sh
curl -fsSL https://api.github.com/repos/goreleaser/nfpm/releases/latest | grep '"tag_name"'
```

Expected: a tag of the form `"tag_name": "v2.XX.Y"`. Record this value as `<NFPM_VERSION>` (e.g. `v2.43.0`).

If the latest release is a v1.x (very unlikely) or a pre-release, walk back through `https://api.github.com/repos/goreleaser/nfpm/releases` to find the most recent stable v2.x.

- [ ] **Step 2: Download the Linux x86_64 tarball and compute its sha256**

Substitute your `<NFPM_VERSION>` (with the leading `v`) and the bare version (without `v`) below. Example for `v2.43.0`:

```sh
NFPM_VERSION=v2.43.0
NFPM_VERSION_BARE=${NFPM_VERSION#v}
URL="https://github.com/goreleaser/nfpm/releases/download/${NFPM_VERSION}/nfpm_${NFPM_VERSION_BARE}_Linux_x86_64.tar.gz"
echo "URL: $URL"
curl -fsSL -o /tmp/nfpm.tar.gz "$URL"
shasum -a 256 /tmp/nfpm.tar.gz
```

Expected: a 64-character hex digest, e.g. `abc123...def`. Record this as `<NFPM_SHA256>`.

- [ ] **Step 3: Cross-check against upstream-published checksums**

The nfpm release page also publishes a `checksums.txt`. Verify your locally-computed sha256 matches:
```sh
curl -fsSL "https://github.com/goreleaser/nfpm/releases/download/${NFPM_VERSION}/checksums.txt" | grep "Linux_x86_64.tar.gz"
```

Expected: a line containing the same sha256 you computed in Step 2 followed by the tarball filename. If they don't match, **stop** — your download was corrupted or the upstream release was tampered with. Re-download and recompute.

- [ ] **Step 4: Clean up**

```sh
rm -f /tmp/nfpm.tar.gz
```

- [ ] **Step 5: Record the values**

Note `<NFPM_VERSION>` and `<NFPM_SHA256>` somewhere convenient (a sticky note, a scratch file). You'll inline them into the workflow YAML in Task 5. No commit yet.

---

## Task 5: Create the release workflow

**Files:**
- Create: `.github/workflows/release.yml`

**Why:** This is the automation that runs on every `v*` tag push and produces the published release. It calls existing project tooling (`make verify-bat`, `make build`) plus the pinned nfpm to produce artifacts, then `softprops/action-gh-release` attaches them.

- [ ] **Step 1: Confirm the workflow doesn't exist yet**

Run:
```sh
ls .github/workflows/
```

Expected: `go.yml` (and only `go.yml`).

- [ ] **Step 2: Create release.yml**

Substitute `<NFPM_VERSION>` and `<NFPM_SHA256>` from Task 4 in the two marked locations below.

Create `.github/workflows/release.yml` with this exact content:

```yaml
# Builds .deb and .rpm packages of volt and attaches them to a GitHub
# Release whenever a tag matching v* is pushed.
# See docs/superpowers/specs/2026-04-26-deb-rpm-packaging-design.md.

name: Release

on:
  push:
    tags:
      - 'v*'

permissions:
  contents: write   # required for softprops/action-gh-release to create the release

jobs:
  release:
    runs-on: ubuntu-latest
    env:
      NFPM_VERSION: <NFPM_VERSION>           # e.g. v2.43.0 — set in Task 4
      NFPM_SHA256: <NFPM_SHA256>             # e.g. abc123... — set in Task 4

    steps:
      - name: Checkout
        uses: actions/checkout@v4
        with:
          submodules: false
          fetch-depth: 0   # required so `git describe` in the Makefile sees tags

      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: "1.26.2"

      - name: Verify embedded bat binary
        run: make verify-bat

      - name: Build volt binary
        run: make build

      - name: Install pinned nfpm
        run: |
          set -euo pipefail
          NFPM_VERSION_BARE="${NFPM_VERSION#v}"
          URL="https://github.com/goreleaser/nfpm/releases/download/${NFPM_VERSION}/nfpm_${NFPM_VERSION_BARE}_Linux_x86_64.tar.gz"
          echo "→ Fetching nfpm ${NFPM_VERSION} from ${URL}"
          curl -fsSL -o /tmp/nfpm.tar.gz "${URL}"
          ACTUAL=$(sha256sum /tmp/nfpm.tar.gz | awk '{print $1}')
          if [ "${ACTUAL}" != "${NFPM_SHA256}" ]; then
            echo "✗ nfpm sha256 mismatch: expected=${NFPM_SHA256} actual=${ACTUAL}"
            exit 1
          fi
          tar -xzf /tmp/nfpm.tar.gz -C /tmp nfpm
          sudo mv /tmp/nfpm /usr/local/bin/nfpm
          sudo chmod 0755 /usr/local/bin/nfpm
          /usr/local/bin/nfpm --version

      - name: Derive package version
        run: |
          set -euo pipefail
          VERSION="${GITHUB_REF_NAME#v}"
          echo "Package version: ${VERSION}"
          echo "VERSION=${VERSION}" >> "${GITHUB_ENV}"

      - name: Build packages
        run: |
          set -euo pipefail
          mkdir -p dist
          nfpm pkg --packager deb --target dist/
          nfpm pkg --packager rpm --target dist/
          ls -lh dist/

      - name: Generate SHA256SUMS
        run: |
          set -euo pipefail
          cd dist
          sha256sum *.deb *.rpm > SHA256SUMS
          cat SHA256SUMS

      - name: Smoke-test package metadata
        run: |
          set -euo pipefail
          echo "→ .deb metadata:"
          dpkg-deb --info dist/*.deb
          echo "→ .deb contents:"
          dpkg-deb --contents dist/*.deb
          echo "→ .rpm metadata:"
          rpm -qpi dist/*.rpm
          echo "→ .rpm contents:"
          rpm -qpl dist/*.rpm
          echo "→ .rpm requirements:"
          rpm -qpR dist/*.rpm

      - name: Publish GitHub Release
        uses: softprops/action-gh-release@v2
        with:
          files: |
            dist/*.deb
            dist/*.rpm
            dist/SHA256SUMS
          fail_on_unmatched_files: true
          generate_release_notes: true
```

- [ ] **Step 3: Validate the YAML syntax**

If you have `actionlint` installed (`brew install actionlint` on macOS, or download from https://github.com/rhysd/actionlint/releases on Linux):

Run:
```sh
actionlint .github/workflows/release.yml
```

Expected: no output (clean exit code 0).

If you don't have actionlint, do a manual YAML parse check:
```sh
python3 -c "import yaml; yaml.safe_load(open('.github/workflows/release.yml'))"
```

Expected: no output, exit code 0. Any error here means the YAML is malformed — fix and re-run.

- [ ] **Step 4: Confirm no other workflows triggered by tags**

Run:
```sh
grep -l "tags:" .github/workflows/*.yml
```

Expected: only `.github/workflows/release.yml`. (We do not want `go.yml` running again on tag pushes — it's `master`/PR only by design.)

- [ ] **Step 5: Commit**

```sh
git add .github/workflows/release.yml
git commit -m "Add release workflow that builds and publishes deb/rpm on tag push

Triggers on v* tags. Verifies the embedded bat, builds bin/volt via the
existing make build, installs a pinned nfpm <NFPM_VERSION>, packages both
formats, generates SHA256SUMS, smoke-tests metadata, and uploads everything
to the GitHub Release for that tag."
```

(Replace `<NFPM_VERSION>` in the commit message with the actual pinned version.)

---

## Task 6: Document the install path in README

**Files:**
- Modify: `README.md`

**Why:** Users won't know the new packages exist unless the README points at them. Today the only install instructions are "git clone + make build". After this task there are three options: download a `.deb`, download a `.rpm`, or build from source.

- [ ] **Step 1: Read the existing Install section**

Run:
```sh
sed -n '37,48p' README.md
```

Expected output: the existing "Install & run" section with the git-clone instructions. Confirm the line numbers — if they've shifted, find the section by `grep -n "## Install" README.md`.

- [ ] **Step 2: Replace the Install section**

Use the Edit tool to replace the existing `## Install & run` section. The exact `old_string` to match (lines 37-48 of the current README) is:

````
## Install & run

```sh
git clone https://github.com/dejanvujkov/volt.git
cd volt
make build        # compiles volt with the bundled bat binary baked in
./bin/volt        # launches the TUI — bat is extracted on first run
```

That's it. The `bat` binary ships pre-built inside this repo
(`internal/batbin/batdata/bat`); the build never has to compile it.
On first run, volt extracts that binary to `~/.cache/volt/bat`.
````

Replace with the following exact `new_string`:

````markdown
## Install

### From a release (recommended)

Pre-built `.deb` and `.rpm` packages for x86_64 Linux are attached to every
[GitHub Release](https://github.com/dejanvujkov/volt/releases). Pick the one
that matches your distro:

**Debian / Ubuntu / Mint / Pop!_OS:**

```sh
VOLT_VERSION=0.1.0   # replace with the latest release tag (without the leading v)
curl -fsSLO "https://github.com/dejanvujkov/volt/releases/download/v${VOLT_VERSION}/volt_${VOLT_VERSION}_amd64.deb"
sudo dpkg -i "volt_${VOLT_VERSION}_amd64.deb"
```

**Fedora / RHEL / CentOS Stream / Rocky / openSUSE:**

```sh
VOLT_VERSION=0.1.0   # replace with the latest release tag (without the leading v)
curl -fsSLO "https://github.com/dejanvujkov/volt/releases/download/v${VOLT_VERSION}/volt-${VOLT_VERSION}-1.x86_64.rpm"
sudo rpm -i "volt-${VOLT_VERSION}-1.x86_64.rpm"
```

**Verifying the download:**

Each release also publishes a `SHA256SUMS` file:

```sh
curl -fsSLO "https://github.com/dejanvujkov/volt/releases/download/v${VOLT_VERSION}/SHA256SUMS"
sha256sum -c SHA256SUMS --ignore-missing
```

Both packages install `volt` to `/usr/bin/volt` and require `sudo` and
`systemd` on the host (used by `volt persist` / `volt reset`). On first
run, volt extracts its bundled `bat` binary to `~/.cache/volt/bat`.

### From source

```sh
git clone https://github.com/dejanvujkov/volt.git
cd volt
make build        # compiles volt with the bundled bat binary baked in
./bin/volt        # launches the TUI — bat is extracted on first run
```

The `bat` binary ships pre-built inside this repo
(`internal/batbin/batdata/bat`); the build never has to compile it.
On first run, volt extracts that binary to `~/.cache/volt/bat`.
````

- [ ] **Step 3: Verify the section reads correctly**

Run:
```sh
grep -n "^## " README.md
```

Expected: the section headers should now read in this order:
```
## Features
## Install
## Keybindings
## CLI mode
## How the bundling works
## Upstream project
## Disclaimer
```

The old `## Install & run` should be gone, replaced by `## Install`.

- [ ] **Step 4: Verify no broken links or stale references**

Run:
```sh
grep -n "Install & run\|make build" README.md
```

Expected:
- No matches for `Install & run` (the old heading is gone).
- One match for `make build` (inside the new "From source" subsection).

If you see anything else, you've left orphan text — fix it.

- [ ] **Step 5: Commit**

```sh
git add README.md
git commit -m "Document deb/rpm install path in README

Add 'From a release' subsection with curl + dpkg/rpm commands and a
SHA256SUMS verification step. Move the existing build-from-source
instructions to a 'From source' subsection."
```

---

## Task 7: End-to-end test with a throwaway tag

**Files:**
- (No files created or modified — this is a release-pipeline integration test.)

**Why:** Tasks 1-6 produced all the static artifacts. This task verifies the *workflow* runs correctly end-to-end against the real GitHub Actions environment by pushing a throwaway tag, watching the workflow, inspecting the produced release, and then deleting the test release. This is the only way to catch problems that only manifest on the actual runner (e.g. the runner's `rpm` CLI behaving differently from your local one).

If you'd rather skip this and learn about any breakage on the first real release, you can — but the cleanup of a broken first release is more annoying than a throwaway test now.

- [ ] **Step 1: Push the changes from Tasks 1-6**

If you haven't pushed yet:
```sh
git push origin master
```

- [ ] **Step 2: Create a throwaway pre-release tag**

```sh
git tag v0.0.1-test1
git push origin v0.0.1-test1
```

This triggers `release.yml`. The chosen tag (`v0.0.1-test1`) sorts below any real release you might cut later and clearly looks like a test.

- [ ] **Step 3: Watch the workflow run**

Open https://github.com/dejanvujkov/volt/actions and find the "Release" run for tag `v0.0.1-test1`. Wait for it to complete (typically 1-2 minutes).

If it fails, click into the failed step. Common first-time failures:
- `make verify-bat` failure → the embedded bat sha256 doesn't match the manifest. Run `make verify-bat` locally to reproduce.
- nfpm sha256 mismatch → the value committed in Task 5 is wrong. Re-run Task 4 to recompute.
- `softprops/action-gh-release` permission error → confirm `permissions: { contents: write }` is at the job level in `release.yml`.

Fix locally, push, delete the test tag (`git push --delete origin v0.0.1-test1` and `git tag -d v0.0.1-test1`), then re-tag and push again.

- [ ] **Step 4: Inspect the published release**

Open https://github.com/dejanvujkov/volt/releases/tag/v0.0.1-test1.

Verify the release page has exactly three attached assets:
- `volt_0.0.1-test1_amd64.deb`
- `volt-0.0.1-test1-1.x86_64.rpm`
- `SHA256SUMS`

Click `SHA256SUMS` to view it — it should contain two lines, one per package, each with a 64-char hex digest.

- [ ] **Step 5: Download and inspect one package**

```sh
mkdir -p /tmp/volt-release-check
cd /tmp/volt-release-check
curl -fsSLO https://github.com/dejanvujkov/volt/releases/download/v0.0.1-test1/volt_0.0.1-test1_amd64.deb
curl -fsSLO https://github.com/dejanvujkov/volt/releases/download/v0.0.1-test1/SHA256SUMS
sha256sum -c SHA256SUMS --ignore-missing
dpkg-deb --info volt_0.0.1-test1_amd64.deb
dpkg-deb --contents volt_0.0.1-test1_amd64.deb
```

Expected:
- `sha256sum -c` reports `volt_0.0.1-test1_amd64.deb: OK`.
- `dpkg-deb --info` shows `Version: 0.0.1-test1`, `Depends: sudo, systemd`, `Maintainer: Dejan Vujkov <vujkovdejan@gmail.com>`.
- `dpkg-deb --contents` shows `./usr/bin/volt`, `./usr/share/doc/volt/copyright`, `./usr/share/doc/volt/README.md`.

- [ ] **Step 6: Delete the test release and tag**

In the GitHub UI: open https://github.com/dejanvujkov/volt/releases, click the `v0.0.1-test1` release, click the trash icon to delete the release. (GitHub may also offer to delete the underlying tag — you can accept that, or leave it and run the explicit tag-deletion commands below.)

Then explicitly delete the tag locally and remotely (idempotent — safe to run even if the UI already deleted it):
```sh
git push --delete origin v0.0.1-test1 || true
git tag -d v0.0.1-test1 || true
```

Verify it's gone:
```sh
git ls-remote --tags origin | grep test
```
Expected: no output.

- [ ] **Step 7: Clean up local test artifacts**

```sh
rm -rf /tmp/volt-release-check
```

- [ ] **Step 8: No commit needed**

This task only verified the live workflow.

---

## Task 8: Manual smoke-test on real distros (first release only)

**Files:**
- (No files modified — this is a manual verification step before tagging the first official release.)

**Why:** The CI smoke-test (Task 5 step 9, run by `release.yml`) only inspects package *metadata* on the Ubuntu runner. It does not actually install the package, so it can't catch issues like "the binary segfaults on a real Fedora install" or "an obscure package format detail breaks `dpkg -i` on Debian stable". One manual round-trip on a real Debian or Ubuntu system and a real Fedora system before the first tagged release closes that gap. Subsequent releases don't need this — only the first time.

If you don't have access to two distro VMs / containers, the minimum viable version is one Debian-family check and one RHEL-family check. Distrobox, Multipass, lxc, or even Docker containers (with caveats — see notes) all work.

- [ ] **Step 1: Re-run Task 7 to produce a fresh test release**

Repeat Task 7 with tag `v0.0.1-test2` so you have a real `.deb` and `.rpm` available for download. Don't delete this release yet.

- [ ] **Step 2: Test on a Debian/Ubuntu environment**

Spin up a Debian/Ubuntu environment. Examples:

**Multipass (macOS/Linux):**
```sh
multipass launch --name volt-test-deb 22.04
multipass shell volt-test-deb
```

**Docker (quick sanity check, but `systemd`/`sudo` won't actually be functional):**
```sh
docker run --rm -it ubuntu:22.04 bash
apt-get update && apt-get install -y curl
```

Inside the environment:

```sh
VOLT_VERSION=0.0.1-test2
curl -fsSLO "https://github.com/dejanvujkov/volt/releases/download/v${VOLT_VERSION}/volt_${VOLT_VERSION}_amd64.deb"
sudo dpkg -i "volt_${VOLT_VERSION}_amd64.deb" || sudo apt-get install -f -y
volt version
volt status
volt capacity
```

Expected:
- `dpkg -i` succeeds (or `apt-get install -f -y` resolves any missing deps and finishes the install).
- `volt version` prints `volt v0.0.1-test2` and a `bat <tag> (bundled, ...)` line.
- `volt status` prints something — even on a VM with no battery, it should print a sensible error rather than crash.

If installed on a real laptop with a battery, `volt status` should print `Charging` / `Discharging` / `Not charging` / `Full`.

Exit and discard the VM.

- [ ] **Step 3: Test on a Fedora environment**

```sh
multipass launch --name volt-test-rpm fedora
multipass shell volt-test-rpm
```

Or with Docker (same caveat):
```sh
docker run --rm -it fedora:latest bash
dnf install -y curl
```

Inside:

```sh
VOLT_VERSION=0.0.1-test2
curl -fsSLO "https://github.com/dejanvujkov/volt/releases/download/v${VOLT_VERSION}/volt-${VOLT_VERSION}-1.x86_64.rpm"
sudo rpm -i "volt-${VOLT_VERSION}-1.x86_64.rpm"
volt version
volt status
```

Expected: same as Step 2.

If `rpm -i` complains about missing `sudo` or `systemd`, install them (`dnf install -y sudo systemd`) and retry — that's the dependency declaration working as intended.

Exit and discard the VM.

- [ ] **Step 4: Delete the v0.0.1-test2 release and tag**

In the GitHub UI: delete the release. Then:
```sh
git push --delete origin v0.0.1-test2
git tag -d v0.0.1-test2
```

- [ ] **Step 5: Record the smoke-test result**

If both distros worked: proceed to Task 9.

If anything failed: do not tag a real release. Diagnose, fix (may require a `nfpm.yaml` change → re-do Task 2 step 4 / Task 3 / Task 7), and repeat this task with a fresh `v0.0.1-testN` tag.

- [ ] **Step 6: No commit needed**

This task is purely operational.

---

## Task 9: Tag the first real release

**Files:**
- (No files modified — this is the cutover.)

**Why:** Tasks 1-8 produced and validated the entire pipeline. This task is the actual public release. Once this is done, anyone can install volt with `dpkg -i` or `rpm -i`.

- [ ] **Step 1: Confirm working tree is clean and pushed**

```sh
git status
git log origin/master..master
```

Expected: `nothing to commit, working tree clean`. The `git log` should show no commits ahead of origin/master (everything pushed).

- [ ] **Step 2: Decide on the version number**

For the first release, `v0.1.0` is conventional. Subsequent releases follow [semver](https://semver.org/) — patch bumps for fixes, minor bumps for new features, major bumps for breaking changes (e.g. dropping a CLI subcommand).

If `git tag` shows existing tags, pick a version that's strictly higher.

- [ ] **Step 3: Tag and push**

```sh
git tag v0.1.0
git push origin v0.1.0
```

- [ ] **Step 4: Watch the release workflow**

Open https://github.com/dejanvujkov/volt/actions. The "Release" workflow should start within seconds and complete in 1-2 minutes.

- [ ] **Step 5: Verify the release page**

Open https://github.com/dejanvujkov/volt/releases/tag/v0.1.0 and confirm:
- Three assets attached: `volt_0.1.0_amd64.deb`, `volt-0.1.0-1.x86_64.rpm`, `SHA256SUMS`.
- The release notes (auto-generated by `softprops/action-gh-release`) summarize commits since the last tag.
- The release is **not** marked as a pre-release. (If you want it marked as latest, edit the release in the UI.)

- [ ] **Step 6: Update the README VOLT_VERSION example**

The README install instructions reference `VOLT_VERSION=0.1.0` as a placeholder example. After your first tag, this is also accurate. For future releases, you can leave the example at `0.1.0` (it's clearly an example users substitute) or bump it. No change required for the first release.

- [ ] **Step 7: Announce (optional, out of scope of this plan)**

If you want to share the release with anyone, the URL is now `https://github.com/dejanvujkov/volt/releases/tag/v0.1.0`.

- [ ] **Step 8: No commit needed**

The release is the artifact.

---

## Spec Coverage Check

Cross-reference of spec sections → tasks that implement them:

| Spec section | Implemented by |
|---|---|
| Goal: publish .deb/.rpm to GitHub Releases on tag push | Tasks 5, 9 |
| Scope: nfpm.yaml producing both formats | Task 2 |
| Scope: tag-triggered workflow | Task 5 |
| Scope: amd64 only | Task 2 (`arch: amd64`) |
| Scope: LICENSE prerequisite | Task 1 |
| Architecture: three new files, no app-code changes | Tasks 1, 2, 5 |
| Versioning Flow steps 1-6 | Task 5 (workflow steps) |
| CI Workflow: trigger, permissions, all 10 steps | Task 5 |
| nfpm Configuration: identity, deps, contents | Task 2 |
| Verification: local nfpm dry-run | Task 3 |
| Verification: lintian/rpmlint | Documented in Task 3 as optional; spec says not in CI |
| Verification: real-system install smoke test | Task 8 |
| Verification: per-release CI smoke test | Task 5 (step 9 of release.yml) |
| Prerequisites: LICENSE file | Task 1 |
| Future Work | Not implemented — explicitly out of scope |

All spec requirements have a corresponding task. Documentation of the install path in README (Task 6) is added beyond the spec because users need to know the new install option exists; the spec's scope section was about the packaging mechanism, not user-facing communication.
