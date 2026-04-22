# volt

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
make build        # compiles bat, embeds it, compiles volt
./bin/volt        # launches the TUI — bat is extracted on first run
```

That's it. There is no separate step to install or copy `bat` anywhere.

## Keybindings

| Key | Action                                     |
| --- | ------------------------------------------ |
| `r` | Force refresh                              |
| `s` | Enter a new threshold (1–100)              |
| `p` | `sudo bat persist` — keep threshold across reboots |
| `R` | `sudo bat reset`   — clear persistence     |
| `?` | Inline key help                            |
| `q` | Quit                                       |

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

1. `make build` runs `third_party/bat` through `go build` with the
   upstream `-ldflags "-X main.tag=$(git describe …)"` so the version
   string is real.
2. The resulting binary is copied to `internal/batbin/batdata/bat`.
3. `internal/batbin/embed.go` declares `//go:embed batdata`, so that
   binary becomes part of the `volt` executable.
4. At runtime, `batbin.EnsureInstalled` writes the embedded binary to
   `~/.cache/volt/bat` (atomically, via a temp file + rename) and chmods
   it `0755`. Subsequent runs detect the cached copy and reuse it. A
   size mismatch after a `volt` upgrade triggers a re-extract.
5. `batbin.Version` runs the resolved binary with `-v` and parses
   `bat <tag>` out of stdout, which is what the TUI banner renders.

## Vendored project

The canonical `bat` sources are checked in under
[`third_party/bat/`](third_party/bat). volt re-implements the sysfs reads
directly; writes that need systemd (persist/reset) defer to the bundled
binary so its well-tested behaviour is preserved.

## Disclaimer

The underlying kernel hook (`charge_control_end_threshold`) is only
exposed by some ASUS and Lenovo ThinkPad laptops. See the upstream
[`bat` README][bat] for specifics.

[bat]: https://github.com/tshakalekholoane/bat
