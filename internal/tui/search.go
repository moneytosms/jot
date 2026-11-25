package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/moneytosms/jot"
)

// searchDebounce delays the library search so a burst of typing runs one
// query. It is a variable so tests can shorten it.
var searchDebounce = 150 * time.Millisecond

// searchResultRow is the content row where the result list starts: input,
// scope line and column header come first.
const searchResultRow = 3

// searchState is the state of the '/' fuzzy search overlay.
type searchState struct {
	input    lineInput
	query    string
	scopeAll bool
	seq      int
	loading  bool
	err      error
	results  []jot.EntryMatch
	cursor   int
	offset   int
	// listFocus marks the result list as the movement target. Arrows and
	// page keys set it; typing a non-movement key returns focus to the input.
	listFocus bool
}

// searchTickMsg fires after the debounce delay and starts the actual search
// when its sequence number is still current.
type searchTickMsg struct{ seq int }

// searchResultMsg carries one search response. seq drops responses for an
// older query, scope or journal.
type searchResultMsg struct {
	seq      int
	scopeAll bool
	journal  string
	query    string
	matches  []jot.EntryMatch
	err      error
}

// searchRect is the full-width search panel.
func (m model) searchRect(lay layout) boxRect {
	return boxRect{x: 0, y: 0, w: max(m.width, 1), h: max(lay.paneH, 1)}
}

// searchVisibleRows is how many result rows fit, clamped to at least one.
func (m model) searchVisibleRows(lay layout) int {
	rect := m.searchRect(lay)
	innerH := max(rect.h-2, 0)
	return max(innerH-searchResultRow, 1)
}

// openSearch switches to the search overlay with a fresh empty query.
func (m model) openSearch() (tea.Model, tea.Cmd) {
	seq := m.search.seq + 1
	input := newLineInput()
	input.placeholder = "type to search every entry"
	input.focus()
	m.search = searchState{input: input, scopeAll: true, seq: seq}
	m.mode = modeSearch
	return m, nil
}

// searchChanged resets the result state for a new query and schedules the
// debounced search.
func (m *model) searchChanged() tea.Cmd {
	m.search.query = m.search.input.value()
	m.search.seq++
	m.search.results = nil
	m.search.cursor = 0
	m.search.offset = 0
	m.search.err = nil
	if strings.TrimSpace(m.search.query) == "" {
		m.search.loading = false
		return nil
	}
	m.search.loading = true
	seq := m.search.seq
	return tea.Tick(searchDebounce, func(time.Time) tea.Msg { return searchTickMsg{seq: seq} })
}

// searchLoadCmd runs the library search for the current query and scope.
// Disk work stays inside the command.
func (m model) searchLoadCmd() tea.Cmd {
	seq := m.search.seq
	scopeAll := m.search.scopeAll
	journal := m.journal
	query := m.search.query
	j := m.jot
	return func() tea.Msg {
		var (
			matches []jot.EntryMatch
			err     error
		)
		if scopeAll {
			matches, err = j.SearchEntries(query)
		} else {
			matches, err = j.SearchEntriesJournal(journal, query)
		}
		return searchResultMsg{
			seq: seq, scopeAll: scopeAll, journal: journal,
			query: query, matches: matches, err: err,
		}
	}
}

// applySearchResult merges a search response, dropping stale ones.
func (m model) applySearchResult(msg searchResultMsg) (tea.Model, tea.Cmd) {
	if m.mode != modeSearch || msg.seq != m.search.seq ||
		msg.scopeAll != m.search.scopeAll || msg.journal != m.journal {
		return m, nil
	}
	m.search.loading = false
	m.search.err = msg.err
	m.search.cursor = 0
	m.search.offset = 0
	if msg.err != nil {
		m.search.results = nil
		m.search.listFocus = false
		return m, nil
	}
	m.search.results = msg.matches
	// Results arriving never steal focus: j and k keep typing until an arrow
	// or ctrl+j/k moves into the list.
	return m, nil
}

// keySearch feeds keys to the search overlay.
func (m model) keySearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	switch key {
	case "esc":
		m.closeOverlay()
		return m, nil
	case "tab":
		m.search.scopeAll = !m.search.scopeAll
		m.search.seq++
		m.search.results = nil
		m.search.cursor = 0
		m.search.offset = 0
		m.search.err = nil
		if strings.TrimSpace(m.search.input.value()) == "" {
			m.search.loading = false
			return m, nil
		}
		m.search.loading = true
		m.search.query = m.search.input.value()
		return m, m.searchLoadCmd()
	case "enter":
		return m.activateSearch()
	case "up", "ctrl+k":
		m.search.listFocus = true
		m.moveSearch(-1)
		return m, nil
	case "down", "ctrl+j":
		m.search.listFocus = true
		m.moveSearch(1)
		return m, nil
	case "k":
		if m.search.listFocus {
			m.moveSearch(-1)
			return m, nil
		}
	case "j":
		if m.search.listFocus {
			m.moveSearch(1)
			return m, nil
		}
	case "pgup":
		m.search.listFocus = true
		m.moveSearch(-m.searchPage())
		return m, nil
	case "pgdown":
		m.search.listFocus = true
		m.moveSearch(m.searchPage())
		return m, nil
	case "ctrl+u":
		m.search.listFocus = true
		m.moveSearch(-m.searchPage())
		return m, nil
	case "ctrl+d":
		m.search.listFocus = true
		m.moveSearch(m.searchPage())
		return m, nil
	case "home", "g":
		if key == "g" && !m.search.listFocus {
			break
		}
		m.search.listFocus = true
		m.search.cursor = 0
		m.ensureSearchVisible()
		return m, nil
	case "end", "G":
		if key == "G" && !m.search.listFocus {
			break
		}
		m.search.listFocus = true
		if len(m.search.results) > 0 {
			m.search.cursor = len(m.search.results) - 1
			m.ensureSearchVisible()
		}
		return m, nil
	case "/":
		if m.search.listFocus {
			m.search.listFocus = false
			return m, nil
		}
	}
	if m.search.input.update(msg) {
		m.search.listFocus = false
		return m, m.searchChanged()
	}
	return m, nil
}

// searchPage is the number of rows a page key jumps.
func (m model) searchPage() int {
	return max(m.searchVisibleRows(m.layout())-1, 1)
}

// moveSearch moves the highlighted result and keeps it on screen.
func (m *model) moveSearch(delta int) {
	if len(m.search.results) == 0 {
		return
	}
	m.search.cursor = clamp(m.search.cursor+delta, 0, len(m.search.results)-1)
	m.ensureSearchVisible()
}

// ensureSearchVisible scrolls the result window to include the cursor.
func (m *model) ensureSearchVisible() {
	rows := m.searchVisibleRows(m.layout())
	if m.search.cursor < m.search.offset {
		m.search.offset = m.search.cursor
	}
	if m.search.cursor >= m.search.offset+rows {
		m.search.offset = m.search.cursor - rows + 1
	}
	last := max(len(m.search.results)-rows, 0)
	m.search.offset = clamp(m.search.offset, 0, last)
}

// scrollSearch moves the result window by delta rows without losing the
// cursor.
func (m *model) scrollSearch(delta int) {
	if len(m.search.results) == 0 {
		return
	}
	rows := m.searchVisibleRows(m.layout())
	last := max(len(m.search.results)-rows, 0)
	m.search.offset = clamp(m.search.offset+delta, 0, last)
	m.search.cursor = clamp(m.search.cursor, m.search.offset, m.search.offset+rows-1)
	m.search.cursor = clamp(m.search.cursor, 0, len(m.search.results)-1)
}

// activateSearch closes the overlay and jumps to the selected hit, opening its
// journal when it belongs to another one.
func (m model) activateSearch() (tea.Model, tea.Cmd) {
	if len(m.search.results) == 0 {
		return m, nil
	}
	hit := m.search.results[m.search.cursor]
	day, err := time.ParseInLocation("2006-01-02", hit.Day, time.Local)
	if err != nil {
		m.search.err = fmt.Errorf("bad day in search result %q", hit.Day)
		return m, nil
	}
	m.closeOverlay()
	m.selected = day
	m.month = firstOfMonth(day)
	m.scroll = 0
	m.scrollToTime = hit.Time
	m.scrollToBody = firstLine(hit.Body)
	if hit.Journal != "" && hit.Journal != m.journal {
		m.journal = hit.Journal
		return m, tea.Batch(m.fullLoad(), savePrefsCmd(m.themeName, m.journal, "journal"))
	}
	return m, m.fullLoad()
}

// applyScrollTarget scrolls the day pane toward the entry a search hit named,
// matching its time first and its first body line second. Best effort only.
func (m *model) applyScrollTarget() {
	if m.scrollToTime == "" && m.scrollToBody == "" {
		return
	}
	width := max(m.layout().innerW, 1)
	rendered := strings.Split(jot.RenderStyled(m.dayContent, width, m.theme.mdStyles()), "\n")
	idx := -1
	if m.scrollToTime != "" {
		idx = findRenderedLine(rendered, m.scrollToTime)
	}
	if idx < 0 && m.scrollToBody != "" {
		idx = findRenderedLine(rendered, m.scrollToBody)
	}
	if idx >= 0 {
		m.scroll = max(idx-1, 0)
	}
	m.scrollToTime = ""
	m.scrollToBody = ""
}

// findRenderedLine returns the first rendered line containing needle.
func findRenderedLine(lines []string, needle string) int {
	needle = strings.TrimSpace(needle)
	if needle == "" {
		return -1
	}
	for i, line := range lines {
		if strings.Contains(line, needle) {
			return i
		}
	}
	return -1
}

// searchView renders the search overlay: input, scope, result list and a
// preview of the highlighted entry.
func (m model) searchView(lay layout) string {
	rect := m.searchRect(lay)
	innerW := max(rect.w-2, 1)
	width := max(innerW-1, 1)

	scope := "all journals"
	if !m.search.scopeAll {
		scope = "this journal (" + m.journal + ")"
	}
	inputLine := m.search.input.view()
	if !m.search.input.isFocused() {
		inputLine = m.theme.muted().Render("press / to search")
	}
	count := len(m.search.results)
	scopeLine := m.theme.muted().Render(fmt.Sprintf("scope: %s · tab switches · %d %s", scope, count, plural(count, "match", "matches")))
	header := m.theme.dim().Render(fmt.Sprintf("%-10s %-10s %-8s %s", "JOURNAL", "DAY", "TIME", "ENTRY"))

	content := []string{inputLine, scopeLine, header}
	rows := m.searchVisibleRows(lay)
	avail := max(width-1, 1)
	listW := avail
	previewW := 0
	if avail >= 28 {
		listW = clamp(avail*3/5, 16, avail-12)
		previewW = max(avail-listW-3, 1)
	}
	if len(m.search.results) == 0 {
		msg := "No matches. Try a shorter query."
		if strings.TrimSpace(m.search.query) == "" {
			msg = "Type to search across every journal. Tab switches scope."
		} else if m.search.loading {
			msg = "Searching…"
		}
		if m.search.err != nil {
			msg = "Search failed: " + m.search.err.Error()
		}
		content = append(content, m.theme.muted().Render(msg))
		for len(content) < rect.h-2 {
			content = append(content, "")
		}
		return m.overlayCanvas(lay, rect, m.boxLines("SEARCH", rect, content))
	}

	// Build preview lines for the highlighted hit once.
	var preview []string
	if previewW > 0 {
		hit := m.search.results[m.search.cursor]
		rendered := jot.RenderStyled(hit.Body, previewW, m.theme.mdStyles())
		preview = strings.Split(rendered, "\n")
	}
	end := min(m.search.offset+rows, len(m.search.results))
	for i := m.search.offset; i < end; i++ {
		hit := m.search.results[i]
		line := m.searchResultLine(hit, listW)
		if i == m.search.cursor {
			line = m.theme.selStyle().Render(padRight(line, listW))
		} else {
			line = padRight(line, listW)
		}
		if previewW > 0 {
			p := ""
			if idx := i - m.search.cursor; idx >= 0 && idx < len(preview) {
				p = ansiTruncate(preview[idx], previewW)
			}
			line += m.theme.dim().Render(" │ ") + p
		}
		content = append(content, line)
	}
	for len(content) < rect.h-2 {
		content = append(content, "")
	}
	return m.overlayCanvas(lay, rect, m.boxLines("SEARCH", rect, content))
}

// searchResultLine renders one plain result row: journal, day, time and a
// one-line body preview, truncated to width.
func (m model) searchResultLine(hit jot.EntryMatch, width int) string {
	at := hit.Time
	if at == "" {
		at = "        "
	}
	body := firstLine(hit.Body)
	line := fmt.Sprintf("%-10s %-10s %-8s %s", hit.Journal, hit.Day, at, body)
	return ansiTruncate(line, max(width, 1))
}

// plural picks the singular or plural word for n.
func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// searchRowAt maps a mouse coordinate to a result index in the window.
func (m model) searchRowAt(lay layout, x, y int) (int, bool) {
	rect := m.searchRect(lay)
	if x < rect.x || x >= rect.x+rect.w {
		return 0, false
	}
	row := contentRow(y, rect)
	if row < searchResultRow || row >= rect.h-2 {
		return 0, false
	}
	idx := m.search.offset + row - searchResultRow
	if idx < 0 || idx >= len(m.search.results) {
		return 0, false
	}
	return idx, true
}

// handleSearchMouse scrolls the result list and activates clicked hits.
func (m model) handleSearchMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	lay := m.layout()
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		m.scrollSearch(-3)
		return m, nil
	case tea.MouseButtonWheelDown:
		m.scrollSearch(3)
		return m, nil
	case tea.MouseButtonLeft:
		if msg.Y == 0 || bodyRow(msg.Y) >= lay.paneH {
			m.closeOverlay()
			return m, nil
		}
		if idx, ok := m.searchRowAt(lay, msg.X, msg.Y); ok {
			m.search.cursor = idx
			m.ensureSearchVisible()
			return m.activateSearch()
		}
	}
	return m, nil
}
