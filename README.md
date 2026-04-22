# volt

A compact Bubble Tea TUI for managing laptop battery charging thresholds on
Linux. `volt` wraps — and bundles — [`tshakalekholoane/bat`][bat] so you get
the same battery-management capabilities through an interactive terminal
interface, plus a command-line mode that mirrors the original tool.

```
  ██╗  ██╗ ██████╗ ██╗  ████████╗
  ██║  ██║██╔═══██╗██║  ╚══██╔══╝
  ██║  ██║██║   ██║██║     ██║
  ╚██╗██╔╝██║   ██║██║     ██║
   ╚███╔╝ ╚██████╔╝███████╗██║
    ╚══╝   ╚═════╝ ╚══════╝╚═╝
```

## Features

- Live-updating battery dashboard (capacity, status, health, threshold).
- Set a new charging threshold interactively (`s`), validated 1–100.
- Persist (`p`) / reset (`R`) the threshold across reboots by delegating
  to the vendored `bat` binary.
- Standalone CLI subcommands (`volt status`, `volt capacity`, …) so it
  drops into scripts in place of `bat`.

## Install & run

```sh
# Build just volt
make build
./bin/volt

# Also build the bundled upstream bat binary
make bat
sudo cp third_party/bat/bat /usr/local/bin/bat

# volt's persist/reset actions now invoke the bundled bat.
./bin/volt
```

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
```

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
