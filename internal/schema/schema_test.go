package schema

import (
	"strings"
	"testing"
)

func TestSearchRanksKeyHitsFirst(t *testing.T) {
	s := Load()
	got := s.Search("sandbox")
	if len(got) == 0 {
		t.Fatal("no results for sandbox")
	}
	seenOther := false
	for _, st := range got {
		inKey := strings.Contains(strings.ToLower(st.Key), "sandbox")
		if !inKey {
			seenOther = true
		} else if seenOther {
			t.Fatalf("%s ranked after a description-only hit", st.Key)
		}
	}
	if got[0].Key != "sandbox" {
		t.Fatalf("first result = %s, want the exact key sandbox", got[0].Key)
	}
}

func TestSearchShortTermSkipsDescriptions(t *testing.T) {
	for _, st := range Load().Search("on") {
		if !strings.Contains(strings.ToLower(st.Key), "on") {
			t.Fatalf("%s matched \"on\" only by description", st.Key)
		}
	}
}

func TestSearchEmptyReturnsAll(t *testing.T) {
	s := Load()
	if got := s.Search(""); len(got) != len(s.Settings) {
		t.Fatalf("empty search returned %d of %d", len(got), len(s.Settings))
	}
}
