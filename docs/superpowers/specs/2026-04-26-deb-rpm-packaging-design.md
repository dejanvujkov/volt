# Volt deb/rpm Packaging — Design

**Date:** 2026-04-26
**Status:** Draft, pending approval
**Owner:** Dejan Vujkov

## Goal

Publish `.deb` and `.rpm` packages of `volt` as GitHub Release assets on every
tag push, so users on Debian/Ubuntu and Fedora/RHEL-family systems can install
volt with their native package manager instead of cloning the repo.

## Scope

**In scope:**

- A single `nfpm.yaml` that produces both `.deb` and `.rpm` from the same
  Go binary built by the existing `make build`.
- A new GitHub Actions workflow (`.github/workflows/release.yml`) triggered
  on tags matching `v*`, which builds the binary, packages it, generates a
  `SHA256SUMS` file, and attaches everything to a GitHub Release.
- amd64 only.
- A `LICENSE` file added to the repo (prerequisite — the project currently
  has no license file).

**Explicitly out of scope (deliberate decisions, see "Decisions" section):**

- No Flatpak. Sandboxing breaks volt's privileged operations
  (persist/reset write a host systemd unit via `sudo bat`).
- No arm64. The embedded `bat` binary is x86-64 only; arm64 would
  double pipeline complexity for ~zero current users.
- No PPA, COPR, OBS, or any third-party hosted repo. GitHub Releases only.
- No GoReleaser. Custom workflow + nfpm directly, so `make build` stays the
  source of truth for compiling the binary.
- No GPG signing. Users verify via HTTPS download + `SHA256SUMS`.
- No man page, no shell completions, no auto-update mechanism, no APT/DNF
  repository.
- No packages for non-tag pushes; master pushes still only run the
  existing `go.yml` build+test workflow.

## Decisions

These are the brainstorming choices that shaped the design. Recording them
so future-us doesn't relitigate.

| # | Question | Decision | Why |
|---|----------|----------|-----|
| 1 | Distribution target | GitHub Releases only | Lowest setup cost; no third-party accounts; matches the project's "single repo, single binary" ethos. |
| 2 | Flatpak fit | Drop Flatpak | The sandbox blocks `/sys/class/power_supply` reads and prevents `sudo bat persist` from writing host systemd units. A Flatpak would be a degraded version that can't actually persist a threshold. |
| 3 | Architectures | amd64 only | The embedded `bat` is x86-64 only. ThinkPad/ASUS laptops with `charge_control_end_threshold` are overwhelmingly x86-64 today. |
| 4 | Tooling | nfpm + custom GitHub Actions workflow (not GoReleaser) | volt has unusual build inputs (embedded third-party binary, `make update-bat`/`verify-bat`, `git describe` versioning). Keeping the existing Makefile as the source of truth and layering a thin packaging step on top is more transparent than adopting GoReleaser's conventions. |
| 5 | Runtime dependencies | Hard-require `sudo` and `systemd` | Mainstream desktop Linux has both by default. Declaring them is honest about what `volt persist`/`reset` need; install fails cleanly on systems where the main feature wouldn't work anyway. |

## Architecture

Three new top-level files; everything else in the repo is unchanged.

```
volt/
├── LICENSE                            # NEW — MIT, matches upstream `bat`
├── nfpm.yaml                          # NEW — single config for both packagers
├── .github/workflows/
│   ├── go.yml                         # unchanged (build+test on master/PR)
│   └── release.yml                    # NEW — packaging on tag push
└── (everything else unchanged)
```

`make build` is the source of truth for compiling `bin/volt`. The new
workflow calls `make build` and then wraps `bin/volt` in `.deb` and `.rpm`
packages — no parallel build path, no duplicated ldflags logic, no
duplicated version derivation.

## Versioning Flow

1. Maintainer pushes a tag matching `v*`:
   ```sh
   git tag v0.1.0
   git push --tags
   ```
2. Workflow derives the package version by stripping the leading `v`:
   ```sh
   VERSION="${GITHUB_REF_NAME#v}"   # "v0.1.0" → "0.1.0"
   ```
3. `make build` runs unchanged. `git describe --always --dirty --tags`
   inside the Makefile sees the tag and produces `v0.1.0`, which is
   injected as `voltVersion` via ldflags. The banner and `volt version`
   continue to display `v0.1.0`.
4. `nfpm` is invoked with `VERSION=0.1.0` (env-interpolated in
   `nfpm.yaml`), producing two files in `dist/`:
   - `volt_0.1.0_amd64.deb` (Debian convention: underscore separators,
     lowercase arch)
   - `volt-0.1.0-1.x86_64.rpm` (RPM convention: hyphen separators, RPM
     arch name, release `-1`)
5. A `SHA256SUMS` file is generated from `dist/*.deb` and `dist/*.rpm`.
6. All three files (`*.deb`, `*.rpm`, `SHA256SUMS`) are attached to the
   GitHub Release for tag `v0.1.0`.

**Pre-release tags** (`v0.1.0-rc1`) are handled identically by the
workflow. The maintainer manually marks the release as "pre-release" in
the GitHub UI if desired; the automation makes no distinction.

## CI Workflow

**File:** `.github/workflows/release.yml` (~50 lines, single job).

**Trigger:**
```yaml
on:
  push:
    tags: ['v*']
```

**Permissions:** least-privilege.
```yaml
permissions:
  contents: write   # required for softprops/action-gh-release
```

**Job steps (sequential, single `ubuntu-latest` runner):**

1. **Checkout** — `actions/checkout@v4`, `submodules: false` (matches
   existing convention; embedded `bat` is committed in-tree, not a
   submodule). Tags must be fetched so `git describe` works:
   `fetch-depth: 0`.
2. **Setup Go** — `actions/setup-go@v4`, `go-version: 1.26.2` (pinned to
   match `go.yml`).
3. **Verify embedded bat** — `make verify-bat`. Catches the case where
   someone tagged a release without re-running verify after a
   `make update-bat`.
4. **Build the binary** — `make build`. Produces `bin/volt`.
5. **Install nfpm** — pinned version (initial: `v2.41.3`) downloaded from
   the official GitHub release tarball, sha256-verified against a
   committed checksum, extracted to `/usr/local/bin/nfpm`. Pinning is
   important: an unpinned `latest` could change package-format defaults
   between releases. Uses the same defensive download pattern as
   `make update-bat`.
6. **Derive version:**
   ```sh
   echo "VERSION=${GITHUB_REF_NAME#v}" >> "$GITHUB_ENV"
   ```
7. **Build packages:**
   ```sh
   mkdir -p dist
   nfpm pkg --packager deb --target dist/
   nfpm pkg --packager rpm --target dist/
   ```
8. **Generate checksums:**
   ```sh
   cd dist && sha256sum *.deb *.rpm > SHA256SUMS
   ```
9. **Smoke-test packages** (metadata only, no install):
   ```sh
   dpkg-deb --info dist/*.deb
   dpkg-deb --contents dist/*.deb
   rpm -qpi dist/*.rpm
   rpm -qpl dist/*.rpm
   ```
   Catches malformed packages before publish. (`rpm` CLI is available on
   `ubuntu-latest`; no install step needed.)
10. **Publish** — `softprops/action-gh-release@v2`:
    ```yaml
    with:
      files: |
        dist/*.deb
        dist/*.rpm
        dist/SHA256SUMS
      fail_on_unmatched_files: true
      generate_release_notes: true
    ```

**Failure modes handled:**
- Stale embedded `bat` → step 3 fails before wasting time on the build.
- Build failure → step 4 fails with the standard Go compiler error.
- Malformed `nfpm.yaml` → step 7 fails with a readable nfpm error.
- Network flake fetching nfpm → step 5 fails with a curl error; rerun
  the workflow.

**Failure modes intentionally not handled:**
- No matrix build (amd64-only, single runner).
- No Go-module caching (release builds are infrequent; cache hit-rate
  would be low).
- No staging/draft release step (ship straight to published; manually
  flag pre-releases in the UI if needed).

## nfpm Configuration

**File:** `nfpm.yaml` (~30 lines).

**Identity:**
```yaml
name: volt
arch: amd64                     # nfpm normalizes to x86_64 for rpm automatically
platform: linux
version: ${VERSION}             # env-interpolated by nfpm
version_schema: semver
release: 1                      # rpm release; bump only for repackages of the same upstream
maintainer: "Dejan Vujkov <vujkovdejan@gmail.com>"
description: |
  A compact Bubble Tea TUI for managing laptop battery charging thresholds
  on Linux. Wraps and bundles tshakalekholoane/bat for an interactive
  terminal interface plus a CLI mode that mirrors the original tool.
vendor: "Dejan Vujkov"          # required by rpm
homepage: https://github.com/dejanvujkov/volt
license: MIT
section: utils                  # deb
rpm:
  group: Applications/System
```

**Dependencies:**
```yaml
depends:
  - sudo
  - systemd
```

nfpm's top-level `depends:` applies to both `.deb` (as `Depends:`) and
`.rpm` (as `Requires:`). No format-specific override needed because both
packages have the same runtime requirements.

**Contents — exactly 3 file mappings:**

| Source       | Destination                       | Mode | nfpm `type` |
|--------------|-----------------------------------|------|-------------|
| `bin/volt`   | `/usr/bin/volt`                   | 0755 | (default)   |
| `LICENSE`    | `/usr/share/doc/volt/copyright`   | 0644 | `doc`       |
| `README.md`  | `/usr/share/doc/volt/README.md`   | 0644 | `doc`       |

**Installed footprint:**
```
/usr/bin/volt                              ~8 MB (volt + embedded bat)
/usr/share/doc/volt/copyright              ~1 KB
/usr/share/doc/volt/README.md              ~5 KB
```

**Runtime behaviour:** unchanged from today. On first run, volt extracts
its embedded `bat` binary to `~/.cache/volt/bat`. The package format is
invisible to the runtime code path. Uninstalling removes `/usr/bin/volt`
but leaves `~/.cache/volt/`; that's standard for user-cache directories
(XDG convention).

**No postinst, postrm, preinst, or prerm scripts.** Nothing to migrate,
nothing to clean up beyond what `dpkg`/`rpm` does for owned files.

## Verification

**Pre-merge / before tagging the first release:**

1. **Local nfpm dry-run:**
   ```sh
   VERSION=0.1.0-test nfpm pkg --packager deb --target /tmp/
   VERSION=0.1.0-test nfpm pkg --packager rpm --target /tmp/
   dpkg-deb --info /tmp/volt_*.deb
   dpkg-deb --contents /tmp/volt_*.deb
   rpm -qpi /tmp/volt-*.rpm
   rpm -qpl /tmp/volt-*.rpm
   ```
2. **Lintian on the .deb:**
   ```sh
   lintian /tmp/volt_*.deb
   ```
   Expected to be clean except for two known cosmetic warnings we
   accept:
   - `binary-without-manpage` — we deliberately don't ship a man page
     yet.
   - `statically-linked-binary` — Go binaries are always statically
     linked; the warning is informational only.
3. **rpmlint on the .rpm:**
   ```sh
   rpmlint /tmp/volt-*.rpm
   ```
   Same accepted warnings (`no-manual-page-for-binary`,
   `statically-linked-binary`).
4. **Real-system install smoke test** (manual, before tagging the first
   release; not required for subsequent tags):
   - On a Debian/Ubuntu VM:
     ```sh
     sudo dpkg -i volt_0.1.0_amd64.deb
     volt version
     volt status
     volt capacity
     ```
   - On a Fedora VM:
     ```sh
     sudo rpm -i volt-0.1.0-1.x86_64.rpm
     volt version
     volt status
     ```

**Per-release (in CI, every tag):**

- `dpkg-deb --info` and `rpm -qpi` run inside the workflow (step 9).
  This catches structurally broken packages before they're published. We
  rely on the manual smoke test only for the *first* release; for
  subsequent releases the metadata smoke test is sufficient because the
  packaging config doesn't change.

**Not in CI:**
- `lintian`/`rpmlint` are not run in CI for the first iteration. They're
  prone to noisy false positives across Ubuntu runner versions and the
  signal-to-noise during normal releases is low. Worth revisiting if we
  ever consider submitting to a real distro repo.
- No actual install test in CI. The runner is Ubuntu, so we can't
  validate the rpm install path; running `dpkg -i` on the .deb in CI
  adds little signal beyond the metadata check.

## Prerequisites

These must be done before the first packaging release can ship:

1. **Add a `LICENSE` file** to the repo root. The design assumes **MIT**
   (matches upstream `bat`'s license, and is what's referenced in
   `nfpm.yaml`'s `license:` field above). If a different license is
   chosen, update both the file and the `nfpm.yaml` `license:` value to
   match. Without this file, `nfpm` will fail to find the `copyright`
   source mapping, and shipping unlicensed software is a real
   legal/distribution gap regardless of packaging.

## Future Work (Out of Scope, Recorded For Later)

These were considered and deferred. Listing them so we don't accidentally
re-discuss them next time someone opens this design.

- **arm64 packages.** Requires per-arch embedded `bat` binaries and
  build-tag selection in `internal/batbin/`. Revisit when there's
  demonstrated user demand or when upstream `bat` publishes arm64
  releases.
- **Man page** generated from a Markdown source via `pandoc` in CI.
  One file + one workflow step.
- **Shell completions** for bash/zsh/fish. Volt's CLI surface is small
  enough that completions don't pay for themselves yet; revisit if the
  CLI grows.
- **APT/DNF repository** for auto-updates. Big jump in operational
  complexity (key management, repo metadata signing, hosting). Only
  justified if there's a real user base asking for it.
- **GPG-signed packages.** Useful only once we're publishing through a
  repo where signature verification is automatic. With direct download
  from GitHub Releases, `SHA256SUMS` over HTTPS is sufficient.
- **Flathub submission.** Was ruled out at design time due to sandbox
  incompatibility. Revisit only if upstream `bat` (or a successor)
  exposes a sandbox-friendly D-Bus interface for the persist/reset
  operations.
