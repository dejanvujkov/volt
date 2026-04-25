# volt

[![Pipeline](https://github.com/dejanvujkov/volt/actions/workflows/go.yml/badge.svg)](https://github.com/dejanvujkov/volt/actions/workflows/go.yml)
![Go Version](https://img.shields.io/badge/go-1.26.2-00ADD8?logo=go&logoColor=white)

A compact Bubble Tea TUI for managing laptop battery charging thresholds on
Linux. `volt` wraps — and **bundles** — [`tshakalekholoane/bat`][bat] so you
get the same battery-management capabilities through an interactive
terminal interface, plus a command-line mode that mirrors the original
tool. The upstream `bat` binary is baked directly into `volt` via
`//go:embed` and extracted to your user cache directory on first run. You
never build or install `bat` separately.

```
  ██╗  ██╗ ██████╗ ██╗  ████████╗
  ██║  ██║██╔═══██╗██║  ╚══██╔══╝
  ██║  ██║██║   ██║██║     ██║
  ╚██╗██╔╝██║   ██║██║     ██║
   ╚███╔╝ ╚██████╔╝███████╗██║
    ╚══╝   ╚═════╝ ╚══════╝╚═╝
    powered by bundled bat <version>
```

## Features

- Live-updating battery dashboard (capacity, status, health, threshold).
- Set a new charging threshold interactively (`s`), validated 1–100.
- Persist (`p`) / reset (`R`) the threshold across reboots, delegating
  to the bundled `bat` binary under `sudo`.
- First-run auto-install: on launch, `volt` extracts its embedded `bat`
  to `$XDG_CACHE_HOME/volt/bat` (typically `~/.cache/volt/bat`) and shows
  a one-line notice.
- Shows the bundled `bat` version in the banner and via `volt version`.
- Standalone CLI subcommands (`volt status`, `volt capacity`, …) so it
  drops into scripts in place of `bat`.

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

## Keybindings

| Key | Action                                             |
| --- | -------------------------------------------------- |
| `r` | Force refresh                                      |
| `s` | Enter a new threshold (1–100)                      |
| `p` | `sudo bat persist` — keep threshold across reboots |
| `R` | `sudo bat reset` — clear persistence               |
| `?` | Inline key help                                    |
| `q` | Quit                                               |

## CLI mode

```sh
volt status         # "Charging" / "Discharging" / …
volt capacity       # integer 0-100
volt health         # full / design as percent
volt threshold      # current charge_control_end_threshold
volt threshold 80   # set it (requires root)
volt persist        # invoke bundled bat
volt reset          # invoke bundled bat
volt version        # "volt <tag>" + "bat <tag> (bundled, <path>)"
```

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

```sh
make update-bat VERSION=v0.10.0
```

This downloads the upstream release binary, verifies its sha256, swaps
the embed slot, and rewrites `BAT_VERSION`. `make verify-bat` re-checks
that the committed binary still matches the manifest at any later
point.

## Upstream project

The upstream tool is [`tshakalekholoane/bat`][bat]. volt re-implements
the sysfs reads directly; writes that need systemd (persist/reset)
defer to the bundled binary so its well-tested behaviour is preserved.
The exact upstream version currently shipped is recorded in
[`internal/batbin/BAT_VERSION`](internal/batbin/BAT_VERSION).

## Disclaimer

The underlying kernel hook (`charge_control_end_threshold`) is only
exposed by some ASUS and Lenovo ThinkPad laptops. See the upstream
[`bat` README][bat] for specifics.

[bat]: https://github.com/tshakalekholoane/bat
