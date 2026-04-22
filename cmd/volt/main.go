//go:build linux

// Binary volt is a Bubble Tea TUI for managing the battery charge-control
// threshold on Linux laptops. It is a thin, interactive front-end over the
// same sysfs attributes driven by the vendored tshakalekholoane/bat project
// (see third_party/bat in this repository). The upstream `bat` binary is
// bundled directly into the volt executable via //go:embed and extracted
// to the user cache directory on first run, so end users never have to
// build or install bat themselves.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"

	"github.com/dejanvujkov/volt/internal/batbin"
	"github.com/dejanvujkov/volt/internal/battery"
	"github.com/dejanvujkov/volt/internal/tui"
)

// voltVersion is stamped at build time via -ldflags "-X main.voltVersion=...".
// Defaults to "dev" for local builds.
var voltVersion = "dev"

const usage = `volt — battery management TUI

Usage:
  volt                      Launch the interactive TUI (default).
  volt status               Print current charging status.
  volt capacity             Print current battery capacity (%).
  volt health               Print battery health (%).
  volt threshold            Print the charge-control end threshold.
  volt threshold <1-100>    Set a new threshold (requires root).
  volt persist              Persist the threshold across reboots (uses bundled bat).
  volt reset                Undo threshold persistence (uses bundled bat).
  volt version              Print volt and bundled bat versions.
  volt --help               Show this message.

The bundled tshakalekholoane/bat binary is extracted to the user cache
directory on first run — no manual installation step is required.`

func main() {
	flag.Usage = func() { fmt.Fprintln(os.Stderr, usage) }
	help := flag.Bool("help", false, "show help")
	showVersion := flag.Bool("version", false, "show version information")
	flag.Parse()

	if *help {
		fmt.Println(usage)
		return
	}
	if *showVersion {
		printVersion()
		return
	}

	args := flag.Args()
	if len(args) == 0 {
		if err := tui.Run(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	if err := runCLI(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

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

// resolveBat returns the cached-or-freshly-extracted bat binary path,
// printing a one-line notice the very first time it is installed.
func resolveBat() (string, error) {
	path, installed, err := batbin.EnsureInstalled()
	if err != nil {
		return "", err
	}
	if installed {
		fmt.Fprintf(os.Stderr, "volt: installed bundled bat → %s\n", path)
	}
	return path, nil
}

func runCLI(args []string) error {
	// `volt version` mirrors the --version flag so it feels natural next to
	// the other bat-style subcommands.
	if args[0] == "version" {
		printVersion()
		return nil
	}

	info, err := battery.Read()
	if err != nil {
		return err
	}
	if !info.Present {
		return errors.New("no battery detected under /sys/class/power_supply/BAT?")
	}

	switch args[0] {
	case "status":
		fmt.Println(info.Status)
	case "capacity":
		fmt.Println(info.Capacity)
	case "health":
		if info.Health == 0 {
			return errors.New("battery health is not exposed by the kernel")
		}
		fmt.Println(info.Health)
	case "threshold":
		switch len(args) {
		case 1:
			if !info.ThresholdSupported {
				return errors.New("charging threshold not exposed by the kernel")
			}
			fmt.Println(info.Threshold)
		case 2:
			pct, perr := strconv.Atoi(args[1])
			if perr != nil {
				return fmt.Errorf("threshold must be an integer: %w", perr)
			}
			bin, berr := resolveBat()
			if berr != nil {
				// Fall back to a direct sysfs write when the bundled binary
				// is missing — the user will still see a useful error if
				// they lack permissions.
				if derr := battery.SetThreshold(pct); derr != nil {
					return derr
				}
				fmt.Printf("Charging threshold set to %d%%.\n", pct)
				return nil
			}
			out, err := battery.SetThresholdWithBat(bin, pct)
			if err != nil {
				if derr := battery.SetThreshold(pct); derr != nil {
					if len(out) > 0 {
						fmt.Fprint(os.Stderr, string(out))
					}
					return derr
				}
				fmt.Printf("Charging threshold set to %d%%.\n", pct)
				return nil
			}
			fmt.Print(string(out))
		default:
			return errors.New("usage: volt threshold [1-100]")
		}
	case "persist":
		bin, err := resolveBat()
		if err != nil {
			return err
		}
		out, err := battery.PersistWithBat(bin)
		if len(out) > 0 {
			fmt.Print(string(out))
		}
		return err
	case "reset":
		bin, err := resolveBat()
		if err != nil {
			return err
		}
		out, err := battery.ResetWithBat(bin)
		if len(out) > 0 {
			fmt.Print(string(out))
		}
		return err
	default:
		flag.Usage()
		return fmt.Errorf("unknown command %q", args[0])
	}
	return nil
}
