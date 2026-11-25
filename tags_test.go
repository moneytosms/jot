package jot

import (
	"slices"
	"testing"
)

func TestTags(t *testing.T) {
	got := Tags("# Heading\nshipped #Work and #work/infra, not#this, (#idea) #1 #a-")
	want := []string{"work", "work/infra", "idea", "a"}
	if !slices.Equal(got, want) {
		t.Fatalf("Tags = %v, want %v", got, want)
	}
}

func TestParseQuery(t *testing.T) {
	q := ParseQuery("deploy #ops tag:Infra journal:work after:2026-01-01 before:today has:todo is:open after:nope")
	if q.Text != "deploy after:nope" || !slices.Equal(q.Tags, []string{"ops", "infra"}) ||
		q.Journal != "work" || q.After != "2026-01-01" || q.Before == "" || !q.HasTodo || !q.Open {
		t.Fatalf("ParseQuery = %+v", q)
	}
}

func TestSearchEntriesFilters(t *testing.T) {
	j := opsTestJot(t)
	addEntry(t, j, "work", "fixed the #deploy script\n- [ ] follow up")
	addEntry(t, j, "work", "lunch")
	addEntry(t, j, "home", "#deploy the garden")
	hits, err := j.SearchEntries("#deploy journal:work")
	if err != nil || len(hits) != 1 || hits[0].Journal != "work" {
		t.Fatalf("tag+journal = %+v, %v", hits, err)
	}
	if hits, _ := j.SearchEntries("is:open"); len(hits) != 1 {
		t.Fatalf("is:open = %+v", hits)
	}
	if hits, _ := j.SearchEntries("garden #deploy"); len(hits) != 1 || hits[0].Journal != "home" {
		t.Fatalf("text+tag = %+v", hits)
	}
	if hits, _ := j.SearchEntries("before:2000-01-01"); len(hits) != 0 {
		t.Fatalf("before = %+v", hits)
	}
	counts, err := j.TagCounts("")
	if err != nil || len(counts) != 1 || counts[0] != (TagCount{"deploy", 2}) {
		t.Fatalf("TagCounts = %+v, %v", counts, err)
	}
}

func TestRenderTags(t *testing.T) {
	got := RenderStyled("ship #it now", 0, MDStyles{Tag: "<"})
	if got != "ship <#it"+ansiReset+" now" {
		t.Fatalf("RenderStyled = %q", got)
	}
}
