package jot

import "unicode"

// FuzzyMatch reports whether every rune of query appears in text, in order,
// ignoring case. An empty query matches anything.
func FuzzyMatch(query, text string) bool {
	qr := []rune(query)
	tr := []rune(text)
	ti := 0
	for _, q := range qr {
		q = unicode.ToLower(q)
		for ti < len(tr) && unicode.ToLower(tr[ti]) != q {
			ti++
		}
		if ti == len(tr) {
			return false
		}
		ti++
	}
	return true
}

// FuzzyScore scores a fuzzy match of query against text. It returns 0 when the
// query does not match; otherwise higher means a better match. Runs of
// consecutive query runes score more than scattered ones, matches at word
// starts (after space, dash, underscore or slash, or across camelCase
// boundaries) score more than matches mid-word, and shorter text scores more
// than longer text. All comparisons ignore case.
func FuzzyScore(query, text string) int {
	qr := []rune(query)
	tr := []rune(text)
	if len(qr) == 0 {
		return 0
	}

	// wordStart marks runes that begin a new word: the first rune, a rune
	// following a separator, or a lower-to-upper camelCase boundary.
	wordStart := make([]bool, len(tr))
	for i, r := range tr {
		if i == 0 {
			wordStart[i] = true
			continue
		}
		prev := tr[i-1]
		switch prev {
		case ' ', '-', '_', '/':
			wordStart[i] = true
		default:
			wordStart[i] = unicode.IsLower(prev) && unicode.IsUpper(r)
		}
	}

	score := 0
	prevMatch := -1
	started := false
	ti := 0
	for _, q := range qr {
		q = unicode.ToLower(q)
		for ti < len(tr) && unicode.ToLower(tr[ti]) != q {
			ti++
		}
		if ti == len(tr) {
			return 0
		}
		score += 10
		if started {
			if ti == prevMatch+1 {
				score += 15 // consecutive match
			}
			if wordStart[ti] {
				score += 20 // word-start match
			}
		} else if ti == 0 {
			score += 30 // match at string start
		} else if wordStart[ti] {
			score += 20 // word-start match
		}
		prevMatch = ti
		started = true
		ti++
	}

	// Prefer shorter targets: the ratio decays the score as text grows.
	score -= len(qr) * len(tr) / 8
	if score < 1 {
		score = 1
	}
	return score
}
