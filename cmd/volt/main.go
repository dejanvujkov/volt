//go:build linux

// Binary volt is a Bubble Tea TUI for managing the battery charge-control
// threshold on Linux laptops. It is a thin, interactive front-end over the
// same sysfs attributes driven by the vendored tshakalekholoane/bat project
// (see third_party/bat in this repository).
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"

	"github.com/dejanvujkov/volt/internal/battery"
	"github.com/dejanvujkov/volt/internal/tui"
)

const usage = `volt — battery management TUI

Usage:
  volt                      Launch the interactive TUI (default).
  volt status               Print current charging status.
  volt capacity             Print current battery capacity (%).
  volt health               Print battery health (%).
  volt threshold            Print the charge-control end threshold.
  volt threshold <1-100>    Set a new threshold (Linux, requires root).
  volt persist              Persist the threshold across reboots (uses bat).
  volt reset                Undo threshold persistence (uses bat).
  volt --help               Show this message.

The command-line mode exists so volt is a drop-in replacement for the
vendored bat tool when a TTY is not available (e.g. in scripts).`

func main() {
	flag.Usage = func() { fmt.Fprintln(os.Stderr, usage) }
	help := flag.Bool("help", false, "show help")
	flag.Parse()

	if *help {
		fmt.Println(usage)
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

func runCLI(args []string) error {
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
			out, err := battery.SetThresholdWithBat(pct)
			if err != nil {
				// Fallback: direct sysfs write (e.g. `bat` not installed).
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
		out, err := battery.PersistWithBat()
		if len(out) > 0 {
			fmt.Print(string(out))
		}
		return err
	case "reset":
		out, err := battery.ResetWithBat()
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
