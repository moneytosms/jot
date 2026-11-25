package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/moneytosms/jot"
)

// feedHeat gives the overlay a loaded result for one year.
func feedHeat(m model, year int, counts map[string]int) model {
	return send(m, heatResultMsg{seq: m.heat.seq, year: year, counts: counts})
}

// TestHeatmapOverlayOpenAndYearNav drives the heatmap overlay headlessly:
// open with y, feed a result, navigate years, close with esc.
func TestHeatmapOverlayOpenAndYearNav(t *testing.T) {
	_, base := newTestModel(t)

	m := send(base, keyMsg("y"))
	if m.mode != modeHeatmap {
		t.Fatalf("mode = %v, want modeHeatmap", m.mode)
	}
	if !m.heat.loading || m.heat.year != time.Now().Year() {
		t.Fatalf("state = %+v, want loading current year", m.heat)
	}

	m = feedHeat(m, 2026, map[string]int{
		"2026-01-01": 1, "2026-01-02": 3, "2026-01-03": 9,
	})
	if m.heat.loading || m.heat.err != nil {
		t.Fatalf("state after result = %+v, want loaded", m.heat)
	}

	// The view renders the header, painted cells and legend.
	view := m.heatView(m.layout())
	if !strings.Contains(view, "Mo Tu We Th Fr Sa Su") {
		t.Error("view lacks the weekday header")
	}
	if !strings.Contains(view, jot.HeatGlyphs[0]) {
		t.Error("view lacks painted cells")
	}
	if !strings.Contains(view, "HEATMAP") {
		t.Error("view lacks the panel title")
	}

	// h goes to the previous year and reloads.
	m2, cmd := m.shiftHeatYear(-1)
	m = m2.(model)
	if m.heat.year != 2025 || !m.heat.loading {
		t.Fatalf("state after h = %+v, want loading 2025", m.heat)
	}
	if cmd == nil {
		t.Fatal("h produced no reload command")
	}

	// A stale result is dropped: the state keeps loading the 2025 query.
	m = send(m, heatResultMsg{seq: m.heat.seq - 1, year: 1234, counts: map[string]int{"2025-01-01": 5}})
	if m.heat.year != 2025 || len(m.heat.counts) != 0 {
		t.Fatalf("stale result applied: %+v", m.heat)
	}

	// esc closes.
	m = press(m, "esc")
	if m.mode != modeBrowse {
		t.Fatalf("esc mode = %v, want browse", m.mode)
	}
}

// TestHeatmapOverlayScroll checks the row scroll keys and clamping.
func TestHeatmapOverlayScroll(t *testing.T) {
	_, base := newTestModel(t)
	m := send(base, keyMsg("y"))
	m = feedHeat(m, 2026, nil)

	rows := m.heatRows()
	total := heatWeekRows(2026)
	if rows >= total {
		t.Skipf("terminal tall enough (%d rows for %d weeks)", rows, total)
	}

	// j moves the window down by one.
	before := m.heat.offset
	m = send(m, keyMsg("j"))
	if m.heat.offset != before+1 {
		t.Fatalf("offset after j = %d, want %d", m.heat.offset, before+1)
	}
	// G jumps to the last window.
	m = send(m, keyMsg("G"))
	if m.heat.offset != total-rows {
		t.Fatalf("offset after G = %d, want %d", m.heat.offset, total-rows)
	}
	// Scrolling past the end clamps.
	m = send(m, keyMsg("j"))
	if m.heat.offset != total-rows {
		t.Fatalf("offset after clamped j = %d, want %d", m.heat.offset, total-rows)
	}
	// g returns to the top.
	m = send(m, keyMsg("g"))
	if m.heat.offset != 0 {
		t.Fatalf("offset after g = %d, want 0", m.heat.offset)
	}
}

// TestHeatmapPaletteAction opens the overlay from the command palette.
func TestHeatmapPaletteAction(t *testing.T) {
	_, base := newTestModel(t)
	m := send(base, ctrlKey(tea.KeyCtrlP))
	m = send(m, keyMsg("heatmap"))
	m = send(m, keyMsg("enter"))
	if m.mode != modeHeatmap {
		t.Fatalf("mode = %v, want modeHeatmap", m.mode)
	}
}
