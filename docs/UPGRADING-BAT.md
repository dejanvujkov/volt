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
