package match

import (
	"reflect"
	"testing"
)

func TestMatchTiers(t *testing.T) {
	cases := []struct {
		name string
		term string
		f    Fields
		tier Tier
		idx  []int
	}{
		{"exact name", "model", Fields{Name: "model", Key: "model"}, NameExact, []int{0, 1, 2, 3, 4}},
		{"exact name ignores case", "MODEL", Fields{Name: "model"}, NameExact, []int{0, 1, 2, 3, 4}},
		{"name prefix", "allow", Fields{Name: "allowedDomains", Key: "sandbox.network.allowedDomains"}, NamePrefix, []int{0, 1, 2, 3, 4}},
		{"name contains", "domain", Fields{Name: "allowedDomains"}, NameContains, []int{7, 8, 9, 10, 11, 12}},
		{"key contains", "sandbox", Fields{Name: "allowedDomains", Key: "sandbox.network.allowedDomains"}, KeyContains, nil},
		{"section word", "perm", Fields{Name: "defaultMode", Key: "defaultMode", Section: "Permissions"}, SectionWord, nil},
		{"section needs word start", "mission", Fields{Section: "Permissions"}, None, nil},
		{"desc word", "conf", Fields{Name: "theme", Desc: "Colour configuration."}, DescWord, nil},
		{"desc needs word start", "figur", Fields{Name: "theme", Desc: "Colour configuration."}, None, nil},
		{"short term skips desc", "on", Fields{Name: "theme", Desc: "Colour configuration on start."}, None, nil},
		{"short term still matches name", "ui", Fields{Name: "ui", Key: "ui"}, NameExact, []int{0, 1}},
		{"short term still matches key", "os", Fields{Name: "name", Key: "os.name"}, KeyContains, nil},
		{"three letters match desc", "own", Fields{Name: "existing", Desc: "Runs on its own row."}, DescWord, nil},
		{"underscore splits words", "effort", Fields{Name: "theme", Desc: "Reads CODE_EFFORT."}, DescWord, nil},
		{"multi word both required", "sandbox network", Fields{Name: "allowedDomains", Key: "sandbox.network.allowedDomains"}, KeyContains, nil},
		{"multi word one missing", "sandbox colour", Fields{Name: "allowedDomains", Key: "sandbox.network.allowedDomains"}, None, nil},
		{"multi word worst tier wins", "allowed sandbox", Fields{Name: "allowedDomains", Key: "sandbox.network.allowedDomains"}, KeyContains, []int{0, 1, 2, 3, 4, 5, 6}},
		{"multi word name indexes merge", "all dom", Fields{Name: "allowedDomains"}, NameContains, []int{0, 1, 2, 7, 8, 9}},
		{"unicode name indexes are runes", "sep", Fields{Name: "✿ Separator"}, NameContains, []int{2, 3, 4}},
		{"empty term", "", Fields{Name: "model"}, None, nil},
		{"blank term", "   ", Fields{Name: "model"}, None, nil},
		{"no match", "zzz", Fields{Name: "model", Key: "model", Section: "Core", Desc: "The model."}, None, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := Match(c.term, c.f)
			if h.Tier != c.tier {
				t.Fatalf("tier = %d, want %d", h.Tier, c.tier)
			}
			if !reflect.DeepEqual(h.Name, c.idx) {
				t.Fatalf("name indexes = %v, want %v", h.Name, c.idx)
			}
		})
	}
}

func TestRankOrdersByTierThenInput(t *testing.T) {
	rows := []Fields{
		{Name: "theme", Desc: "Colour configuration for the model picker."},
		{Name: "defaultModel", Key: "defaultModel"},
		{Name: "model", Key: "model"},
		{Name: "name", Key: "model.name"},
		{Name: "modelAlias", Key: "modelAlias"},
		{Name: "other", Key: "other"},
	}
	got := Rank("model", rows)
	want := []int{2, 4, 1, 3, 0}
	var idx []int
	for _, h := range got {
		idx = append(idx, h.Index)
	}
	if !reflect.DeepEqual(idx, want) {
		t.Fatalf("order = %v, want %v", idx, want)
	}
	if got[0].Tier != NameExact || got[1].Tier != NamePrefix || got[2].Tier != NameContains || got[3].Tier != KeyContains || got[4].Tier != DescWord {
		t.Fatalf("tiers = %+v", got)
	}
}

func TestRankEmptyTermKeepsEverything(t *testing.T) {
	rows := []Fields{{Name: "a"}, {Name: "b"}, {Name: "c"}}
	got := Rank("", rows)
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
	for i, h := range got {
		if h.Index != i || h.Tier != None {
			t.Fatalf("hit %d = %+v", i, h)
		}
	}
}

func TestJoinSplitRoundTrip(t *testing.T) {
	f := Fields{Name: "Separator", Key: "statusline.separator", Section: "Status line", Desc: "The text between fields.\nSpaces   and newlines stay."}
	if got := Split(Join(f)); got != f {
		t.Fatalf("round trip = %+v, want %+v", got, f)
	}
	if got := Split("plain"); got != (Fields{Name: "plain"}) {
		t.Fatalf("bare split = %+v", got)
	}
	if got := Split(""); got != (Fields{}) {
		t.Fatalf("empty split = %+v", got)
	}
}
