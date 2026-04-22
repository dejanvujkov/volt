//go:build linux

package battery

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	sysfsGlob        = "/sys/class/power_supply/BAT?"
	thresholdAttr    = "charge_control_end_threshold"
	capacityAttr     = "capacity"
	statusAttr       = "status"
	chargeFull       = "charge_full"
	chargeFullDesign = "charge_full_design"
	energyFull       = "energy_full"
	energyFullDesign = "energy_full_design"
)

// Read returns a snapshot of the first battery discovered under
// /sys/class/power_supply/BAT?.
func Read() (Info, error) {
	var info Info

	matches, err := filepath.Glob(sysfsGlob)
	if err != nil {
		return info, err
	}
	if len(matches) == 0 {
		return info, nil
	}

	root := matches[0]
	info.Present = true
	info.Root = root

	if v, err := readInt(filepath.Join(root, capacityAttr)); err == nil {
		info.Capacity = v
	}
	if v, err := readString(filepath.Join(root, statusAttr)); err == nil {
		info.Status = v
	}

	info.Health = computeHealth(root)

	if v, err := readInt(filepath.Join(root, thresholdAttr)); err == nil {
		info.Threshold = v
		info.ThresholdSupported = true
	} else if !errors.Is(err, fs.ErrNotExist) {
		return info, err
	}

	return info, nil
}

// SetThreshold writes a new charge-control end threshold to the kernel. The
// caller is expected to either run as root or to have delegated the call to
// the vendored `bat` binary (see SetThresholdWithBat).
func SetThreshold(pct int) error {
	if pct < 1 || pct > 100 {
		return fmt.Errorf("threshold must be between 1 and 100, got %d", pct)
	}
	root, err := primaryRoot()
	if err != nil {
		return err
	}
	path := filepath.Join(root, thresholdAttr)
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return ErrNotFound
		}
		return err
	}
	return os.WriteFile(path, []byte(strconv.Itoa(pct)), 0o644)
}

// SetThresholdWithBat shells out to the given bat binary to apply a new
// threshold. Callers should resolve the path via batbin.EnsureInstalled
// rather than trusting $PATH — there are several unrelated tools named
// "bat" in the wild (notably sharkdp/bat).
func SetThresholdWithBat(binPath string, pct int) ([]byte, error) {
	if pct < 1 || pct > 100 {
		return nil, fmt.Errorf("threshold must be between 1 and 100, got %d", pct)
	}
	if binPath == "" {
		return nil, errors.New("no bundled bat binary available")
	}
	return exec.Command(binPath, "threshold", strconv.Itoa(pct)).CombinedOutput()
}

// PersistWithBat invokes `sudo <binPath> persist`.
func PersistWithBat(binPath string) ([]byte, error) {
	return runSudoBat(binPath, "persist")
}

// ResetWithBat invokes `sudo <binPath> reset`.
func ResetWithBat(binPath string) ([]byte, error) {
	return runSudoBat(binPath, "reset")
}

func runSudoBat(binPath, subcmd string) ([]byte, error) {
	if binPath == "" {
		return nil, errors.New("no bundled bat binary available")
	}
	sudo, err := exec.LookPath("sudo")
	if err != nil {
		// Fall back to running bat directly; it will refuse if unprivileged.
		return exec.Command(binPath, subcmd).CombinedOutput()
	}
	return exec.Command(sudo, binPath, subcmd).CombinedOutput()
}

func primaryRoot() (string, error) {
	matches, err := filepath.Glob(sysfsGlob)
	if err != nil {
		return "", err
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("no battery found under %s", sysfsGlob)
	}
	return matches[0], nil
}

func computeHealth(root string) int {
	// Prefer charge_* counters (reported in µAh) and fall back to energy_*
	// (µWh). Mirrors the upstream `bat` implementation.
	pairs := [][2]string{
		{chargeFull, chargeFullDesign},
		{energyFull, energyFullDesign},
	}
	for _, p := range pairs {
		full, err1 := readInt(filepath.Join(root, p[0]))
		design, err2 := readInt(filepath.Join(root, p[1]))
		if err1 != nil || err2 != nil || design == 0 {
			continue
		}
		return full * 100 / design
	}
	return 0
}

func readString(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(bytes.TrimSpace(b)), nil
}

func readInt(path string) (int, error) {
	s, err := readString(path)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(s))
}
