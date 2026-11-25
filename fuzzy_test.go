package jot

import "testing"

func TestFuzzyMatch(t *testing.T) {
	tests := []struct {
		query, text string
		want        bool
	}{
		{"", "anything", true},
		{"", "", true},
		{"abc", "aXbXc", true},
		{"abc", "acb", false},
		{"abc", "abc", true},
		{"GYM", "gym session", true},
		{"gym", "GYM SESSION", true},
		{"xyz", "no match here", false},
		{"你好", "a你好b", true},
	}
	for _, tt := range tests {
		if got := FuzzyMatch(tt.query, tt.text); got != tt.want {
			t.Errorf("FuzzyMatch(%q, %q) = %v, want %v", tt.query, tt.text, got, tt.want)
		}
	}
}

func TestFuzzyScore_NoMatch(t *testing.T) {
	if got := FuzzyScore("xyz", "gym session"); got != 0 {
		t.Errorf("expected 0 for a non-match, got %d", got)
	}
	if got := FuzzyScore("", ""); got != 0 {
		t.Errorf("expected 0 for an empty query and text, got %d", got)
	}
}

func TestFuzzyScore_ConsecutiveBeatsScattered(t *testing.T) {
	if FuzzyScore("gym", "gym session") <= FuzzyScore("gym", "axgxyxmz") {
		t.Error("expected consecutive matches to score higher than scattered ones")
	}
}

func TestFuzzyScore_WordStartBeatsMidWord(t *testing.T) {
	if FuzzyScore("g", "the gym") <= FuzzyScore("g", "agenda") {
		t.Error("expected a word-start match to score higher than a mid-word one")
	}
}

func TestFuzzyScore_StartBeatsLater(t *testing.T) {
	if FuzzyScore("g", "gym session") <= FuzzyScore("g", "the gym") {
		t.Error("expected a string-start match to score higher than a later one")
	}
}

func TestFuzzyScore_WordStartBoundaries(t *testing.T) {
	texts := []string{"a b-gym_x/yZ", "camelCaseGym"}
	for _, text := range texts {
		if FuzzyScore("g", text) <= FuzzyScore("g", "agenda extra") {
			t.Errorf("expected the separator or camelCase boundary in %q to score higher", text)
		}
	}
}

func TestFuzzyScore_ShorterTarget(t *testing.T) {
	if FuzzyScore("gym", "gym") <= FuzzyScore("gym", "gym session with a lot more text") {
		t.Error("expected the shorter target to score higher")
	}
}

func TestFuzzyScore_CaseInsensitive(t *testing.T) {
	a := FuzzyScore("GYM", "gym session")
	b := FuzzyScore("gym", "GYM SESSION")
	if a != b {
		t.Errorf("expected case-insensitive scoring: %d vs %d", a, b)
	}
	if a == 0 {
		t.Fatal("expected a match score above zero")
	}
}

func TestFuzzyScore_Deterministic(t *testing.T) {
	first := FuzzyScore("note", "another note about noting")
	for i := 0; i < 10; i++ {
		if got := FuzzyScore("note", "another note about noting"); got != first {
			t.Fatalf("score changed between calls: %d then %d", first, got)
		}
	}
}
