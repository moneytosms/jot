package tui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/moneytosms/jot"
)

// heatState is the state of the year heatmap overlay.
type heatState struct {
	// year is the year currently charted.
	year int
	// seq tags requests so stale results are dropped.
	seq int
	// loading is true while the counts are being read.
	loading bool
	// err is the load error, if any.
	err error
	// counts maps "YYYY-MM-DD" labels to entry counts.
	counts map[string]int
	// offset is the first visible week row inside the scroll window.
	offset int
}

// heatResultMsg carries one year heatmap response.
type heatResultMsg struct {
	seq    int
	year   int
	counts map[string]int
	err    error
}

// openHeatmap opens the year heatmap of the open journal, charting the
// current year.
func (m model) openHeatmap() (tea.Model, tea.Cmd) {
	seq := m.heat.seq + 1
	m.heat = heatState{year: time.Now().Year(), seq: seq, loading: true, counts: map[string]int{}}
	m.mode = modeHeatmap
	return m, m.heatLoadCmd()
}

// heatLoadCmd reads one year of day counts in the background.
func (m model) heatLoadCmd() tea.Cmd {
	seq := m.heat.seq
	year := m.heat.year
	journal := m.journal
	j := m.jot
	return func() tea.Msg {
		days, err := j.YearDays(journal, year)
		counts := make(map[string]int, len(days))
		for _, d := range days {
			counts[d.Day] = d.Entries
		}
		return heatResultMsg{seq: seq, year: year, counts: counts, err: err}
	}
}

// applyHeatResult merges a heatmap response, dropping stale ones.
func (m model) applyHeatResult(msg heatResultMsg) (tea.Model, tea.Cmd) {
	if m.mode != modeHeatmap || msg.seq != m.heat.seq {
		return m, nil
	}
	m.heat.loading = false
	m.heat.err = msg.err
	m.heat.counts = msg.counts
	m.heat.offset = 0
	return m, nil
}

// shiftHeatYear moves the chart to another year and reloads.
func (m model) shiftHeatYear(delta int) (tea.Model, tea.Cmd) {
	year := m.heat.year + delta
	if year < 1 || year > 9999 {
		return m, nil
	}
	m.heat.year = year
	m.heat.seq++
	m.heat.loading = true
	m.heat.err = nil
	m.heat.counts = map[string]int{}
	m.heat.offset = 0
	return m, m.heatLoadCmd()
}

// keyHeat feeds keys to the heatmap overlay.
func (m model) keyHeat(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.closeOverlay()
		return m, nil
	case "left", "h", "H", "<":
		return m.shiftHeatYear(-1)
	case "right", "l", "L", ">":
		return m.shiftHeatYear(1)
	case "t":
		return m.shiftHeatYear(time.Now().Year() - m.heat.year)
	case "up", "k":
		m.scrollHeat(-1)
		return m, nil
	case "down", "j":
		m.scrollHeat(1)
		return m, nil
	case "pgup", "ctrl+u":
		m.scrollHeat(-m.heatPage())
		return m, nil
	case "pgdown", "ctrl+d":
		m.scrollHeat(m.heatPage())
		return m, nil
	case "home", "g":
		m.heat.offset = 0
		return m, nil
	case "end", "G":
		m.heat.offset = max(heatWeekRows(m.heat.year)-m.heatRows(), 0)
		return m, nil
	}
	return m, nil
}

// heatRect is the full-width heatmap panel, like the todo panel.
func (m model) heatRect(lay layout) boxRect {
	return boxRect{x: 0, y: 0, w: max(m.width, 1), h: max(lay.paneH, 1)}
}

// heatOverhead counts the content lines around the week rows: the status
// line, the weekday header and the legend.
const heatOverhead = 3

// heatRows is how many week rows fit in the panel, at least one.
func (m model) heatRows() int {
	rect := m.heatRect(m.layout())
	innerH := max(rect.h-2, 0)
	return max(innerH-heatOverhead, 1)
}

// heatPage is the number of rows a page key jumps.
func (m model) heatPage() int { return max(m.heatRows()-1, 1) }

// heatWeekRows is the number of week rows the year renders: the Mondays
// from the week of January 1st to the week of December 31st.
func heatWeekRows(year int) int {
	first := time.Date(year, time.January, 1, 0, 0, 0, 0, time.Local)
	last := time.Date(year, time.December, 31, 0, 0, 0, 0, time.Local)
	rows := 0
	for d := first.AddDate(0, 0, -((int(first.Weekday()) + 6) % 7)); !d.After(last); d = d.AddDate(0, 0, 7) {
		rows++
	}
	return rows
}

// scrollHeat moves the scroll window by delta rows, clamped to the grid.
func (m *model) scrollHeat(delta int) {
	last := max(heatWeekRows(m.heat.year)-m.heatRows(), 0)
	m.heat.offset = clamp(m.heat.offset+delta, 0, last)
}

// heatPainter paints heatmap cells with the theme ramp: dim dots for empty
// days, then green, bright green and yellow blocks as the counts rise. The
// plain theme renders the same glyphs uncolored.
type heatPainter struct{ theme Theme }

// Cell implements jot.HeatPainter.
func (p heatPainter) Cell(level int) string {
	glyph := jot.HeatGlyphs[level]
	switch level {
	case 1:
		return p.theme.good().Render(glyph)
	case 2:
		return p.theme.good().Bold(true).Render(glyph)
	case 3:
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color(p.theme.Yellow)).
			Bold(true).
			Render(glyph)
	default:
		return p.theme.dim().Render(glyph)
	}
}

// heatView renders the year heatmap overlay.
func (m model) heatView(lay layout) string {
	rect := m.heatRect(lay)
	title := fmt.Sprintf("HEATMAP · %s · %d", m.journal, m.heat.year)
	legend := m.theme.dim().Render(jot.HeatLegend(nil))

	if m.heat.loading {
		return m.heatFrame(lay, rect, title, []string{
			m.theme.muted().Render("loading heatmap…"), legend,
		})
	}
	if m.heat.err != nil {
		return m.heatFrame(lay, rect, title, []string{
			m.theme.bad().Render("✗ " + m.heat.err.Error()), legend,
		})
	}

	days, entries := 0, 0
	for _, n := range m.heat.counts {
		if n > 0 {
			days++
		}
		entries += n
	}
	status := m.theme.muted().Render(fmt.Sprintf(
		"%dd %de · h l change year · j k scroll · t today · esc close", days, entries))

	grid := jot.HeatmapLines(m.heat.year, m.heat.counts, time.Now(), heatPainter{m.theme})
	rows := m.heatRows()
	m.heat.offset = clamp(m.heat.offset, 0, max(len(grid)-1-rows, 0))
	// grid[0] is the weekday header; the week rows follow.
	content := []string{status, grid[0]}
	for i := m.heat.offset; i < min(m.heat.offset+rows, len(grid)-1); i++ {
		content = append(content, grid[i+1])
	}
	content = append(content, legend)
	return m.heatFrame(lay, rect, title, content)
}

// heatFrame pads the content and wraps it in the bordered overlay panel.
func (m model) heatFrame(lay layout, rect boxRect, title string, content []string) string {
	for len(content) < rect.h-2 {
		content = append(content, "")
	}
	return m.overlayCanvas(lay, rect, m.boxLines(title, rect, content))
}
