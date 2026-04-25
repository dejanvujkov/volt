// Package battery exposes a view of the host's primary battery together
// with helpers for mutating the charge-control threshold.
//
// The implementation reads the sysfs entries used by upstream
// tshakalekholoane/bat and is able to drive persist/reset operations
// through that project's binary when it is present on $PATH. volt
// targets Linux laptops only; other platforms will fail to build by
// design (see the build tags on battery_linux.go).
package battery

import "fmt"

// Info is the snapshot returned by Read.
type Info struct {
	// Present indicates whether a battery was found on the host.
	Present bool
	// Capacity is the current charge level in percent (0-100).
	Capacity int
	// Status is the raw charging status, e.g. "Charging", "Discharging",
	// "Full", "Not charging".
	Status string
	// Health is the ratio between full/full-design capacity, in percent.
	// Zero when the kernel does not expose the relevant counters.
	Health int
	// Threshold is the charge-control end threshold in percent. Zero when
	// the kernel/firmware does not expose it.
	Threshold int
	// ThresholdSupported reports whether the kernel exposes the
	// charge_control_end_threshold attribute.
	ThresholdSupported bool
	// Root is the backing sysfs directory.
	Root string
}

// ErrNotFound is returned when the kernel does not expose a charging
// threshold attribute for the detected battery.
var ErrNotFound = fmt.Errorf("charging threshold attribute not found")
