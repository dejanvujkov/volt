//go:build linux

package tui

import (
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/dejanvujkov/volt/internal/battery"
)

// newTestModel returns a model pre-populated with a fake bat path and battery
// info so that key handlers don't bail out with "bat unavailable" guards.
func newTestModel() model {
	m := initialModel()
	m.batPath = "/fake/bat"
	m.info = battery.Info{
		Present:            true,
		Capacity:           72,
		Status:             "Discharging",
		Threshold:          80,
		ThresholdSupported: true,
	}
	return m
}

func key(k string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
}

func specialKey(t tea.KeyType) tea.KeyMsg {
	return tea.KeyMsg{Type: t}
}

// --- Normal mode ------------------------------------------------------------

func TestNormalMode_SEntersInputMode(t *testing.T) {
	m := newTestModel()
	result, _ := m.Update(key("s"))
	rm := result.(model)
	if rm.mode != modeInput {
		t.Fatalf("mode = %d, want modeInput (%d)", rm.mode, modeInput)
	}
}

func TestNormalMode_SBlockedWithoutBat(t *testing.T) {
	m := newTestModel()
	m.batPath = ""
	result, _ := m.Update(key("s"))
	rm := result.(model)
	if rm.mode != modeNormal {
		t.Fatal("should stay in modeNormal when bat is unavailable")
	}
}

func TestNormalMode_SBlockedWithoutThreshold(t *testing.T) {
	m := newTestModel()
	m.info.ThresholdSupported = false
	result, _ := m.Update(key("s"))
	rm := result.(model)
	if rm.mode != modeNormal {
		t.Fatal("should stay in modeNormal when threshold not supported")
	}
}

func TestNormalMode_QuestionMarkShowsHelp(t *testing.T) {
	m := newTestModel()
	result, _ := m.Update(key("?"))
	rm := result.(model)
	if rm.status == "" {
		t.Fatal("expected help text in status")
	}
}

func TestNormalMode_RefreshSetsStatus(t *testing.T) {
	m := newTestModel()
	result, _ := m.Update(key("r"))
	rm := result.(model)
	if rm.status != "Refreshing…" {
		t.Fatalf("status = %q, want %q", rm.status, "Refreshing…")
	}
}

// --- Input mode -------------------------------------------------------------

func TestInputMode_DigitsAccepted(t *testing.T) {
	m := newTestModel()
	m.mode = modeInput

	result, _ := m.Update(key("8"))
	rm := result.(model)
	result, _ = rm.Update(key("0"))
	rm = result.(model)

	if rm.input != "80" {
		t.Fatalf("input = %q, want %q", rm.input, "80")
	}
}

func TestInputMode_MaxThreeDigits(t *testing.T) {
	m := newTestModel()
	m.mode = modeInput
	m.input = "100"

	result, _ := m.Update(key("5"))
	rm := result.(model)
	if rm.input != "100" {
		t.Fatalf("input = %q, should not accept 4th digit", rm.input)
	}
}

func TestInputMode_Backspace(t *testing.T) {
	m := newTestModel()
	m.mode = modeInput
	m.input = "80"

	result, _ := m.Update(specialKey(tea.KeyBackspace))
	rm := result.(model)
	if rm.input != "8" {
		t.Fatalf("input = %q, want %q after backspace", rm.input, "8")
	}
}

func TestInputMode_EscCancels(t *testing.T) {
	m := newTestModel()
	m.mode = modeInput
	m.input = "80"

	result, _ := m.Update(specialKey(tea.KeyEscape))
	rm := result.(model)
	if rm.mode != modeNormal {
		t.Fatal("esc should return to modeNormal")
	}
	if rm.input != "" {
		t.Fatalf("input = %q, should be cleared", rm.input)
	}
	if rm.status != "Cancelled." {
		t.Fatalf("status = %q, want %q", rm.status, "Cancelled.")
	}
}

func TestInputMode_InvalidThreshold(t *testing.T) {
	m := newTestModel()
	m.mode = modeInput
	m.input = "0"

	result, _ := m.Update(specialKey(tea.KeyEnter))
	rm := result.(model)
	// Should stay in input mode on invalid value.
	if rm.mode != modeInput {
		t.Fatalf("mode = %d, should stay in modeInput on invalid threshold", rm.mode)
	}
}

func TestInputMode_ValidThresholdExitsInput(t *testing.T) {
	m := newTestModel()
	m.mode = modeInput
	m.input = "80"

	result, cmd := m.Update(specialKey(tea.KeyEnter))
	rm := result.(model)
	if rm.mode != modeNormal {
		t.Fatalf("mode = %d, want modeNormal after valid threshold", rm.mode)
	}
	if cmd == nil {
		t.Fatal("expected a command to set the threshold")
	}
}

// --- Confirm persist mode ---------------------------------------------------

func TestConfirmPersist_EscDeclines(t *testing.T) {
	m := newTestModel()
	m.mode = modeConfirmPersist

	result, cmd := m.Update(specialKey(tea.KeyEscape))
	rm := result.(model)
	if rm.mode != modeNormal {
		t.Fatal("esc should return to modeNormal")
	}
	if rm.status != "Threshold set (not persisted)." {
		t.Fatalf("status = %q", rm.status)
	}
	if cmd != nil {
		t.Fatal("esc should not trigger a command")
	}
}

func TestConfirmPersist_EnterPersists(t *testing.T) {
	m := newTestModel()
	m.mode = modeConfirmPersist

	result, cmd := m.Update(specialKey(tea.KeyEnter))
	rm := result.(model)
	if rm.mode != modeNormal {
		t.Fatal("enter should return to modeNormal")
	}
	if cmd == nil {
		t.Fatal("enter should trigger persist command")
	}
}

func TestConfirmPersist_IgnoresOtherKeys(t *testing.T) {
	m := newTestModel()
	m.mode = modeConfirmPersist

	result, _ := m.Update(key("x"))
	rm := result.(model)
	if rm.mode != modeConfirmPersist {
		t.Fatal("other keys should be ignored in confirm mode")
	}
}

// --- actionMsg transitions --------------------------------------------------

func TestActionMsg_ThresholdSuccessEntersConfirmPersist(t *testing.T) {
	m := newTestModel()
	msg := actionMsg{label: "threshold", output: "Threshold set to 80%."}

	result, _ := m.Update(msg)
	rm := result.(model)
	if rm.mode != modeConfirmPersist {
		t.Fatalf("mode = %d, want modeConfirmPersist", rm.mode)
	}
}

func TestActionMsg_ThresholdErrorStaysNormal(t *testing.T) {
	m := newTestModel()
	msg := actionMsg{label: "threshold", err: errFake}

	result, _ := m.Update(msg)
	rm := result.(model)
	if rm.mode != modeNormal {
		t.Fatalf("mode = %d, want modeNormal on error", rm.mode)
	}
}

func TestActionMsg_PersistDoesNotEnterConfirm(t *testing.T) {
	m := newTestModel()
	msg := actionMsg{label: "persist", output: "persist: ok"}

	result, _ := m.Update(msg)
	rm := result.(model)
	if rm.mode != modeNormal {
		t.Fatalf("mode = %d, persist should stay in modeNormal", rm.mode)
	}
}

// --- infoMsg clears refresh status ------------------------------------------

func TestInfoMsg_ClearsRefreshingStatus(t *testing.T) {
	m := newTestModel()
	m.status = "Refreshing…"

	result, _ := m.Update(infoMsg{info: m.info})
	rm := result.(model)
	if rm.status == "Refreshing…" {
		t.Fatal("infoMsg should clear the Refreshing status")
	}
}

func TestInfoMsg_PreservesOtherStatus(t *testing.T) {
	m := newTestModel()
	m.status = "Threshold set (not persisted)."

	result, _ := m.Update(infoMsg{info: m.info})
	rm := result.(model)
	if rm.status != "Threshold set (not persisted)." {
		t.Fatalf("infoMsg should not overwrite non-refresh status, got %q", rm.status)
	}
}

var errFake = fmt.Errorf("fake error")
