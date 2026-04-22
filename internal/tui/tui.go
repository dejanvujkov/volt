//go:build linux

// Package tui renders the volt Bubble Tea interface. volt is Linux-only;
// see the battery package for the sysfs backend this TUI drives.
package tui

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/dejanvujkov/volt/internal/batbin"
	"github.com/dejanvujkov/volt/internal/battery"
)

// Run launches the TUI; it blocks until the user quits.
func Run() error {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	_, err := p.Run()
	return err
}

// --- Messages -------------------------------------------------------------

type tickMsg time.Time
type infoMsg struct {
	info battery.Info
	err  error
}
type actionMsg struct {
	label  string
	output string
	err    error
}
type batReadyMsg struct {
	path        string
	version     string
	installed   bool // true when the binary was extracted during this run
	err         error
}

// --- Model ----------------------------------------------------------------

type mode int

const (
	modeNormal         mode = iota
	modeInput               // prompting for a new threshold
	modeConfirmPersist      // prompting to persist after threshold change
)

type model struct {
	info    battery.Info
	loadErr error

	batPath    string
	batVersion string
	batErr     error

	mode    mode
	input   string // buffer for threshold entry
	status  string // last status / action output shown in the footer
	width   int
	height  int
	startAt time.Time
}

func initialModel() model {
	return model{
		status:  "Preparing bundled bat binary…",
		startAt: time.Now(),
	}
}

// Init kicks off the bat extraction, the first battery fetch, and the
// refresh ticker in parallel.
func (m model) Init() tea.Cmd {
	return tea.Batch(ensureBatCmd(), fetchInfoCmd(), tickCmd())
}

// --- Commands -------------------------------------------------------------

func tickCmd() tea.Cmd {
	return tea.Tick(3*time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func fetchInfoCmd() tea.Cmd {
	return func() tea.Msg {
		info, err := battery.Read()
		return infoMsg{info: info, err: err}
	}
}

// ensureBatCmd extracts the bundled bat binary on first run and resolves
// its version for display. It is intentionally safe to call repeatedly;
// subsequent invocations observe the cached copy and return installed=false.
func ensureBatCmd() tea.Cmd {
	return func() tea.Msg {
		path, installed, err := batbin.EnsureInstalled()
		if err != nil {
			return batReadyMsg{err: err}
		}
		return batReadyMsg{
			path:      path,
			version:   batbin.Version(path),
			installed: installed,
		}
	}
}

func setThresholdCmd(binPath string, pct int) tea.Cmd {
	return func() tea.Msg {
		_, err := battery.SetThresholdWithBat(binPath, pct)
		if err != nil {
			if derr := battery.SetThreshold(pct); derr != nil {
				return actionMsg{label: "threshold", err: derr}
			}
		}
		return actionMsg{label: "threshold", output: fmt.Sprintf("Threshold set to %d%%.", pct)}
	}
}

func sudoBatCmd(binPath, subcmd, label string) tea.Cmd {
	c := exec.Command("sudo", binPath, subcmd)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		if err != nil {
			return actionMsg{label: label, err: err}
		}
		return actionMsg{label: label, output: fmt.Sprintf("%s: ok", label)}
	})
}

// --- Update ---------------------------------------------------------------

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil

	case tickMsg:
		return m, tea.Batch(fetchInfoCmd(), tickCmd())

	case infoMsg:
		m.info = msg.info
		m.loadErr = msg.err
		if m.status == "Refreshing…" {
			m.status = ""
		}
		return m, nil

	case batReadyMsg:
		m.batPath = msg.path
		m.batVersion = msg.version
		m.batErr = msg.err
		switch {
		case msg.err != nil:
			m.status = fmt.Sprintf("bat unavailable: %v (mutating actions disabled)", msg.err)
		case msg.installed:
			m.status = fmt.Sprintf("First-run setup: installed bundled bat → %s", msg.path)
		default:
			m.status = ""
		}
		return m, nil

	case actionMsg:
		switch {
		case msg.err != nil:
			m.status = fmt.Sprintf("%s failed: %v — %s", msg.label, msg.err, strings.TrimSpace(msg.output))
		default:
			text := strings.TrimSpace(msg.output)
			if text == "" {
				text = fmt.Sprintf("%s: ok", msg.label)
			}
			// After a successful threshold change, prompt to persist.
			if msg.label == "threshold" {
				m.mode = modeConfirmPersist
				m.status = text + "  Persist across reboots? Enter = yes, Esc = no"
				return m, fetchInfoCmd()
			}
			m.status = text
		}
		// Refresh after any mutation so the UI reflects the new state.
		return m, fetchInfoCmd()

	case tea.KeyMsg:
		switch m.mode {
		case modeInput:
			return m.updateInput(msg)
		case modeConfirmPersist:
			return m.updateConfirmPersist(msg)
		default:
			return m.updateNormal(msg)
		}
	}
	return m, nil
}

func (m model) updateNormal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c", "esc":
		return m, tea.Quit
	case "r":
		m.status = "Refreshing…"
		return m, fetchInfoCmd()
	case "s":
		if m.batPath == "" {
			m.status = "Bundled bat binary is unavailable; rebuild volt with `make build`."
			return m, nil
		}
		if !m.info.ThresholdSupported {
			m.status = "Kernel does not expose charge_control_end_threshold."
			return m, nil
		}
		m.mode = modeInput
		m.input = ""
		m.status = "Enter threshold (1-100), press Enter to apply, Esc to cancel."
		return m, nil
	case "p":
		if m.batPath == "" {
			m.status = "Bundled bat binary is unavailable; rebuild volt with `make build`."
			return m, nil
		}
		m.status = "Running `sudo bat persist`…"
		return m, sudoBatCmd(m.batPath, "persist", "persist")
	case "R":
		if m.batPath == "" {
			m.status = "Bundled bat binary is unavailable; rebuild volt with `make build`."
			return m, nil
		}
		m.status = "Running `sudo bat reset`…"
		return m, sudoBatCmd(m.batPath, "reset", "reset")
	case "?":
		m.status = "Keys: r refresh · s set threshold · p persist · R reset · q quit"
		return m, nil
	}
	return m, nil
}

func (m model) updateInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeNormal
		m.input = ""
		m.status = "Cancelled."
		return m, nil
	case "enter":
		var pct int
		if _, err := fmt.Sscanf(m.input, "%d", &pct); err != nil || pct < 1 || pct > 100 {
			m.status = fmt.Sprintf("Invalid threshold %q; expected an integer between 1 and 100.", m.input)
			return m, nil
		}
		m.mode = modeNormal
		m.input = ""
		m.status = fmt.Sprintf("Applying threshold %d%% via bundled bat…", pct)
		return m, setThresholdCmd(m.batPath, pct)
	case "backspace":
		if len(m.input) > 0 {
			m.input = m.input[:len(m.input)-1]
		}
		return m, nil
	default:
		if len(msg.Runes) == 1 && msg.Runes[0] >= '0' && msg.Runes[0] <= '9' && len(m.input) < 3 {
			m.input += string(msg.Runes)
		}
		return m, nil
	}
}

func (m model) updateConfirmPersist(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		m.mode = modeNormal
		m.status = "Running `sudo bat persist`…"
		return m, sudoBatCmd(m.batPath, "persist", "persist")
	case "esc":
		m.mode = modeNormal
		m.status = "Threshold set (not persisted)."
		return m, nil
	}
	return m, nil
}

// --- View -----------------------------------------------------------------

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#F5D547")).
			Padding(0, 1)
	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#5F5F87")).
			Padding(0, 1).
			MarginTop(1)
	labelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#8a8a8a")).
			Width(14)
	valueStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#EEEEEE"))
	statusOK     = lipgloss.NewStyle().Foreground(lipgloss.Color("#5fd787"))
	statusWarn   = lipgloss.NewStyle().Foreground(lipgloss.Color("#F28B82"))
	footerStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#8a8a8a")).MarginTop(1)
	helpStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#5F5F87")).MarginTop(1)
	bannerChrome = lipgloss.NewStyle().Foreground(lipgloss.Color("#F5D547"))
	subtitleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#8a8a8a")).Italic(true)
)

func (m model) View() string {
	var b strings.Builder

	b.WriteString(banner())
	b.WriteString("\n")
	b.WriteString(subtitleStyle.Render("    " + m.batLine()))
	b.WriteString("\n")

	if m.loadErr != nil {
		b.WriteString(statusWarn.Render(fmt.Sprintf("error reading battery: %v", m.loadErr)))
		b.WriteString("\n")
	}

	b.WriteString(boxStyle.Render(m.renderHost()))
	b.WriteString("\n")
	b.WriteString(boxStyle.Render(m.renderCharge()))

	if m.mode == modeInput {
		b.WriteString("\n")
		prompt := fmt.Sprintf("new threshold ▸ %s▍", m.input)
		b.WriteString(boxStyle.BorderForeground(lipgloss.Color("#F5D547")).Render(prompt))
	}

	b.WriteString("\n")
	b.WriteString(footerStyle.Render(m.status))
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("keys  r refresh · s set threshold · p persist · R reset · q quit"))

	return b.String()
}

// batLine returns the one-line "powered by bat x.y" string shown under the
// banner. It distinguishes three states: fully ready, ready-without-version,
// and unavailable.
func (m model) batLine() string {
	switch {
	case m.batErr != nil:
		return fmt.Sprintf("bat: unavailable (%v)", m.batErr)
	case m.batPath == "":
		return "bat: resolving…"
	case m.batVersion == "":
		return "powered by bundled bat (unknown version)"
	default:
		return fmt.Sprintf("powered by bundled bat %s", m.batVersion)
	}
}

func banner() string {
	art := []string{
		"  ██╗  ██╗ ██████╗ ██╗  ████████╗",
		"  ██║  ██║██╔═══██╗██║  ╚══██╔══╝",
		"  ██║  ██║██║   ██║██║     ██║   ",
		"  ╚██╗██╔╝██║   ██║██║     ██║   ",
		"   ╚███╔╝ ╚██████╔╝███████╗██║   ",
		"    ╚══╝   ╚═════╝ ╚══════╝╚═╝   ",
	}
	return bannerChrome.Render(strings.Join(art, "\n"))
}

func (m model) renderHost() string {
	info := m.info

	state := "battery detected"
	if !info.Present {
		state = "no battery detected"
	}
	root := info.Root
	if root == "" {
		root = "—"
	}
	batPath := m.batPath
	if batPath == "" {
		batPath = "—"
	}

	rows := [][2]string{
		{"state", state},
		{"sysfs root", root},
		{"bat binary", batPath},
		{"uptime", time.Since(m.startAt).Round(time.Second).String()},
	}
	return titleStyle.Render("host") + "\n" + renderRows(rows)
}

func (m model) renderCharge() string {
	info := m.info
	if !info.Present {
		return titleStyle.Render("battery") + "\n" +
			statusWarn.Render("No battery found under /sys/class/power_supply/BAT?.")
	}

	capStr := fmt.Sprintf("%d%%", info.Capacity)
	status := info.Status
	if status == "" {
		status = "unknown"
	}
	health := "—"
	if info.Health > 0 {
		health = fmt.Sprintf("%d%%", info.Health)
	}
	threshold := "not exposed by kernel"
	if info.ThresholdSupported {
		threshold = fmt.Sprintf("%d%%", info.Threshold)
	}

	bar := renderBar(info.Capacity, 32)

	rows := [][2]string{
		{"capacity", capStr + "  " + bar},
		{"status", colourStatus(status)},
		{"health", health},
		{"threshold", threshold},
	}
	return titleStyle.Render("battery") + "\n" + renderRows(rows)
}

func renderRows(rows [][2]string) string {
	var b strings.Builder
	for i, r := range rows {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(labelStyle.Render(r[0]))
		b.WriteString("  ")
		b.WriteString(valueStyle.Render(r[1]))
	}
	return b.String()
}

func colourStatus(s string) string {
	switch strings.ToLower(s) {
	case "charging", "charged", "full":
		return statusOK.Render(s)
	case "discharging", "not charging":
		return statusWarn.Render(s)
	default:
		return s
	}
}

func renderBar(pct, width int) string {
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	filled := pct * width / 100
	empty := width - filled

	fillColor := lipgloss.Color("#5fd787")
	switch {
	case pct <= 15:
		fillColor = lipgloss.Color("#F28B82")
	case pct <= 35:
		fillColor = lipgloss.Color("#F5D547")
	}

	fill := lipgloss.NewStyle().Foreground(fillColor).Render(strings.Repeat("█", filled))
	gap := lipgloss.NewStyle().Foreground(lipgloss.Color("#3a3a3a")).Render(strings.Repeat("░", empty))
	return "[" + fill + gap + "]"
}
