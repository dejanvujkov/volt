//go:build linux

package battery

import (
	"os"
	"path/filepath"
	"testing"
)

// fakeBattery creates a temporary sysfs-like directory tree and returns its
// path. The caller can write attribute files into it before calling the
// functions under test.
func fakeBattery(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, capacityAttr), "72")
	writeFile(t, filepath.Join(dir, statusAttr), "Discharging")
	return dir
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// --- readInt / readString ---------------------------------------------------

func TestReadInt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "val")

	writeFile(t, path, "42")
	v, err := readInt(path)
	if err != nil {
		t.Fatalf("readInt: %v", err)
	}
	if v != 42 {
		t.Fatalf("readInt = %d, want 42", v)
	}
}

func TestReadInt_Invalid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "val")

	writeFile(t, path, "abc")
	_, err := readInt(path)
	if err == nil {
		t.Fatal("expected error for non-numeric content")
	}
}

func TestReadString(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "val")

	writeFile(t, path, "  hello  ")
	v, err := readString(path)
	if err != nil {
		t.Fatalf("readString: %v", err)
	}
	if v != "hello" {
		t.Fatalf("readString = %q, want %q", v, "hello")
	}
}

// --- computeHealth ----------------------------------------------------------

func TestComputeHealth_ChargeCounters(t *testing.T) {
	dir := fakeBattery(t)
	writeFile(t, filepath.Join(dir, chargeFull), "4000000")
	writeFile(t, filepath.Join(dir, chargeFullDesign), "5000000")

	h := computeHealth(dir)
	if h != 80 {
		t.Fatalf("computeHealth = %d, want 80", h)
	}
}

func TestComputeHealth_EnergyFallback(t *testing.T) {
	dir := fakeBattery(t)
	// No charge_full/charge_full_design — should fall back to energy counters.
	writeFile(t, filepath.Join(dir, energyFull), "45000000")
	writeFile(t, filepath.Join(dir, energyFullDesign), "50000000")

	h := computeHealth(dir)
	if h != 90 {
		t.Fatalf("computeHealth = %d, want 90", h)
	}
}

func TestComputeHealth_NoCounters(t *testing.T) {
	dir := fakeBattery(t)
	h := computeHealth(dir)
	if h != 0 {
		t.Fatalf("computeHealth = %d, want 0 when no counters present", h)
	}
}

// --- SetThreshold -----------------------------------------------------------

func TestSetThreshold_Bounds(t *testing.T) {
	for _, pct := range []int{0, -1, 101, 200} {
		if err := SetThreshold(pct); err == nil {
			t.Errorf("SetThreshold(%d) should fail", pct)
		}
	}
}
