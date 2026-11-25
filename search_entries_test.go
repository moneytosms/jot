package jot

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// writeDay writes a day file by hand, including untimed preambles that Add
// cannot produce.
func writeDay(t *testing.T, j *Jot, journal, day, content string) {
	t.Helper()
	dir := filepath.Join(j.Root(), journal)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, day+".md"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestSearchEntries_Ranking(t *testing.T) {
	j := testJot(t)
	writeDay(t, j, "work", "2026-01-02",
		"## 10:00:00\n\nsome random note\n\n## 11:00:00\n\ngym plan session big\n")
	writeDay(t, j, "work", "2026-01-01",
		"## 09:30:00\n\nbrief gym note\n")
	writeDay(t, j, "work", "2026-01-03",
		"## 08:00:00\n\ngym\n")

	matches, err := j.SearchEntriesJournal("work", "gym")
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 3 {
		t.Fatalf("expected 3 matches, got %d: %+v", len(matches), matches)
	}
	// "gym" alone: consecutive and string-start wins, then days newest first.
	wantDays := []string{"2026-01-03", "2026-01-02", "2026-01-01"}
	for i, m := range matches {
		if m.Day != wantDays[i] {
			t.Errorf("match %d: got day %s, want %s", i, m.Day, wantDays[i])
		}
	}
	if matches[0].Score <= matches[1].Score || matches[1].Score <= matches[2].Score {
		t.Errorf("scores not strictly descending: %+v", matches)
	}
	if matches[0].Time != "08:00:00" || matches[0].Body != "gym" {
		t.Errorf("unexpected top match: %+v", matches[0])
	}
}

func TestSearchEntries_TieBreakByTime(t *testing.T) {
	j := testJot(t)
	writeDay(t, j, "work", "2026-01-02",
		"## 09:00:00\n\nmorning gym\n\n## 17:30:00\n\nevening gym\n")

	matches, err := j.SearchEntriesJournal("work", "gym")
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 2 {
		t.Fatalf("expected 2 matches, got %d: %+v", len(matches), matches)
	}
	if matches[0].Time != "17:30:00" || matches[1].Time != "09:00:00" {
		t.Errorf("expected later time first on a tie: %+v", matches)
	}
}

func TestSearchEntries_CaseInsensitive(t *testing.T) {
	j := testJot(t)
	if err := j.Add(Entry{Name: "work", Day: "2026-01-02", Body: "Gym Session At Noon"}); err != nil {
		t.Fatal(err)
	}
	matches, err := j.SearchEntriesJournal("work", "gYm")
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d: %+v", len(matches), matches)
	}
}

func TestSearchEntries_CrossJournal(t *testing.T) {
	j := testJot(t)
	if err := j.Add(Entry{Name: "work", Day: "2026-01-02", Body: "gym at lunch"}); err != nil {
		t.Fatal(err)
	}
	if err := j.Add(Entry{Name: "default", Day: "2026-01-01", Body: "walked to the gym"}); err != nil {
		t.Fatal(err)
	}
	if err := j.Add(Entry{Name: "default", Day: "2026-01-03", Body: "unrelated"}); err != nil {
		t.Fatal(err)
	}

	matches, err := j.SearchEntries("gym")
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 2 {
		t.Fatalf("expected 2 matches, got %d: %+v", len(matches), matches)
	}
	journals := map[string]bool{}
	for _, m := range matches {
		journals[m.Journal] = true
	}
	if !journals["work"] || !journals["default"] {
		t.Errorf("expected hits from both journals: %+v", matches)
	}
}

func TestSearchEntries_SkipsBackups(t *testing.T) {
	j := testJot(t)
	writeDay(t, j, "backups", "2026-01-02", "## 10:00:00\n\ngym in backups\n")
	matches, err := j.SearchEntries("gym")
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Errorf("expected the backups journal to be skipped, got %+v", matches)
	}
}

func TestSearchEntries_MissingJournal(t *testing.T) {
	j := testJot(t)
	if _, err := j.SearchEntriesJournal("nope", "gym"); !errors.Is(err, ErrNoJournal) {
		t.Fatalf("expected ErrNoJournal, got %v", err)
	}
}

func TestSearchEntries_EmptyQuery(t *testing.T) {
	j := testJot(t)
	if err := j.Init(); err != nil {
		t.Fatal(err)
	}
	if _, err := j.SearchEntries("   "); err == nil || err.Error() != "empty search query" {
		t.Fatalf("expected an empty search query error, got %v", err)
	}
	if _, err := j.SearchEntriesJournal("work", ""); err == nil || err.Error() != "empty search query" {
		t.Fatalf("expected an empty search query error, got %v", err)
	}
}

func TestSearchEntries_UntimedPreamble(t *testing.T) {
	j := testJot(t)
	writeDay(t, j, "work", "2026-01-02",
		"untimed preamble line about yoga\n\n## 10:00:00\n\ntimed gym note\n")

	matches, err := j.SearchEntriesJournal("work", "yoga")
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d: %+v", len(matches), matches)
	}
	if matches[0].Time != "" {
		t.Errorf("expected an empty Time for the preamble, got %q", matches[0].Time)
	}
	if matches[0].Body != "untimed preamble line about yoga" {
		t.Errorf("unexpected body: %q", matches[0].Body)
	}
}

func TestSearchEntries_EmptyRoot(t *testing.T) {
	j := testJot(t)
	matches, err := j.SearchEntries("gym")
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("expected no matches, got %+v", matches)
	}
}

func TestSearchEntries_PreambleBeforeFirstHeading(t *testing.T) {
	j := testJot(t)
	writeDay(t, j, "work", "2026-01-02", "loose notes\n")
	matches, err := j.SearchEntriesJournal("work", "loose")
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 || matches[0].Time != "" || matches[0].Body != "loose notes" {
		t.Fatalf("unexpected matches: %+v", matches)
	}
}
