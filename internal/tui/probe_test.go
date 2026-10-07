package tui

import (
	"fmt"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestProbe(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	m := newModel(t.TempDir())
	var longest string
	for i := range m.sch.Settings {
		st := &m.sch.Settings[i]
		if len(st.Desc) > len(m.sch.Get(orKey(longest, st.Key)).Desc) || longest == "" {
			longest = st.Key
		}
	}
	fmt.Println("longest", longest, m.sch.Get(longest).Kind, len(m.sch.Get(longest).Desc))
	for _, sz := range [][2]int{{80, 24}, {120, 40}} {
		m.Update(tea.WindowSizeMsg{Width: sz[0], Height: sz[1]})
		selectKey(t, m, longest)
		fmt.Println(ansi.Strip(m.View().Content))
		m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
		fmt.Println("mode", m.mode)
		fmt.Println(ansi.Strip(m.View().Content))
		m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	}
}

func orKey(a, b string) string {
	if a == "" {
		return b
	}
	return a
}
