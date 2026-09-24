package tui

import (
	"fmt"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/huh/v2"

	"github.com/jorden-parker/claude-config-cli/internal/schema"
	"github.com/jorden-parker/claude-config-cli/internal/value"
)

const customChoice = "custom…"
const unsetChoice = "(unset)"

// editState holds the huh form and the bound variables for one edit.
type editState struct {
	st      *schema.Setting
	form    *huh.Form
	choice  string   // select-based kinds
	text    string   // text-based kinds
	picks   []string // multiselect
	lockAll bool     // multiselect: true value
}

// newEdit builds a form for a setting, pre-filled from the current value.
func newEdit(st *schema.Setting, current any, sch *schema.Schema, width, height int, isDark bool) *editState {
	e := &editState{st: st}
	desc := st.Desc
	if len(desc) > 300 {
		desc = desc[:299] + "…"
	}
	title := fmt.Sprintf("%s  (%s)", st.Key, st.Kind)
	var groups []*huh.Group

	switch st.Kind {
	case schema.KindBool:
		e.choice = fmt.Sprint(current)
		if current == nil {
			e.choice = "true"
		}
		opts := []huh.Option[string]{
			huh.NewOption(withHelp("true", st), "true"),
			huh.NewOption(withHelp("false", st), "false"),
		}
		groups = append(groups, huh.NewGroup(huh.NewSelect[string]().Title(title).Description(desc).Options(opts...).Value(&e.choice)))

	case schema.KindEnum, schema.KindEnumOrString:
		e.choice = fmt.Sprint(current)
		var opts []huh.Option[string]
		found := false
		for _, o := range st.Options {
			opts = append(opts, huh.NewOption(withHelp(o, st), o))
			if o == e.choice {
				found = true
			}
		}
		if st.Kind == schema.KindEnumOrString {
			opts = append(opts, huh.NewOption(customChoice+" "+dimHint(st.Hint), customChoice))
			if current != nil && !found {
				e.text = e.choice
				e.choice = customChoice
			}
		}
		if current == nil {
			e.choice = st.Options[0]
		}
		groups = append(groups, huh.NewGroup(huh.NewSelect[string]().Title(title).Description(desc).Options(opts...).Value(&e.choice).Filtering(false)))
		if st.Kind == schema.KindEnumOrString {
			in := huh.NewInput().Title(st.Key + " (custom value)").Description(st.Hint).Value(&e.text).
				Validate(func(s string) error {
					if strings.TrimSpace(s) == "" {
						return fmt.Errorf("enter a value")
					}
					return nil
				})
			if len(st.Suggestions) > 0 {
				in.Suggestions(st.Suggestions)
			}
			groups = append(groups, huh.NewGroup(in).WithHideFunc(func() bool { return e.choice != customChoice }))
		}

	case schema.KindString:
		e.text = value.Text(st, current)
		in := huh.NewInput().Title(title).Description(desc).Placeholder(st.Type).Value(&e.text).
			Validate(func(s string) error { _, err := value.Parse(st, s); return err })
		if st.Key == "model" || st.Key == "agent" {
			in.Suggestions(sch.ModelAliases)
		}
		groups = append(groups, huh.NewGroup(in))

	case schema.KindNumber:
		e.text = value.Text(st, current)
		groups = append(groups, huh.NewGroup(huh.NewInput().Title(title).Description(desc).Placeholder(st.Type).Value(&e.text).
			Validate(func(s string) error {
				if _, err := strconv.ParseFloat(strings.TrimSpace(s), 64); err != nil {
					return fmt.Errorf("enter a number")
				}
				return nil
			})))

	case schema.KindMultiSelect:
		if current == true {
			e.lockAll = true
		} else if arr, ok := current.([]any); ok {
			for _, a := range arr {
				e.picks = append(e.picks, fmt.Sprint(a))
			}
		}
		var opts []huh.Option[string]
		for _, o := range st.Options {
			opts = append(opts, huh.NewOption(o, o).Selected(contains(e.picks, o)))
		}
		groups = append(groups,
			huh.NewGroup(huh.NewConfirm().Title(title).Description(desc+"\n\nSet to true (all kinds)?").Affirmative("true (all)").Negative("choose…").Value(&e.lockAll)),
			huh.NewGroup(huh.NewMultiSelect[string]().Title(st.Key).Options(opts...).Value(&e.picks)).WithHideFunc(func() bool { return e.lockAll }),
		)

	case schema.KindArray, schema.KindMap, schema.KindMapBool:
		e.text = value.Text(st, current)
		ph := "one item per line"
		if st.Kind == schema.KindMap {
			ph = "KEY=VALUE, one per line"
		}
		if st.Kind == schema.KindMapBool {
			ph = "name=true, one per line"
		}
		if len(st.Suggestions) > 0 {
			ph += "  •  values: " + strings.Join(st.Suggestions, ", ")
		}
		groups = append(groups, huh.NewGroup(huh.NewText().Title(title).Description(desc+"\n"+ph).Lines(10).Value(&e.text).
			ExternalEditor(true).
			Validate(func(s string) error { _, err := value.Parse(st, s); return err })))

	default: // json, group
		e.text = value.Text(st, current)
		if e.text == "" && st.Example != "" {
			e.text = exampleValue(st)
		}
		extra := "JSON. Press ctrl+e to open in $EDITOR."
		if st.Key == "hooks" {
			extra += "\nEvents: " + strings.Join(sch.HookEvents, ", ")
		}
		groups = append(groups, huh.NewGroup(huh.NewText().Title(title).Description(desc+"\n"+extra).Lines(14).Value(&e.text).
			ExternalEditor(true).EditorExtension("json").
			Validate(func(s string) error { _, err := value.Parse(st, s); return err })))
	}

	km := huh.NewDefaultKeyMap()
	km.Quit = key.NewBinding(key.WithKeys("esc", "ctrl+c"), key.WithHelp("esc", "cancel"))
	e.form = huh.NewForm(groups...).WithTheme(formTheme(isDark)).WithKeyMap(km).WithWidth(width).WithHeight(height).WithShowHelp(true)
	return e
}

// result converts the completed form into the JSON value to store.
func (e *editState) result() (any, error) {
	st := e.st
	switch st.Kind {
	case schema.KindBool, schema.KindEnum:
		return value.Parse(st, e.choice)
	case schema.KindEnumOrString:
		if e.choice == customChoice {
			return value.Parse(st, e.text)
		}
		return e.choice, nil
	case schema.KindMultiSelect:
		if e.lockAll {
			return true, nil
		}
		if e.picks == nil {
			e.picks = []string{}
		}
		return e.picks, nil
	default:
		return value.Parse(st, e.text)
	}
}

func withHelp(o string, st *schema.Setting) string {
	if h := st.OptionHelp[o]; h != "" {
		if len(h) > 90 {
			h = h[:89] + "…"
		}
		return o + "  — " + h
	}
	return o
}

func dimHint(h string) string {
	if h == "" {
		return ""
	}
	return "(" + h + ")"
}

func contains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

// exampleValue pulls this key's value out of the doc example JSON, so a JSON
// editor starts from a valid shape instead of a blank box.
func exampleValue(st *schema.Setting) string {
	var doc map[string]any
	if err := jsonUnmarshal(st.Example, &doc); err != nil {
		return ""
	}
	var cur any = doc
	for _, p := range strings.Split(st.Key, ".") {
		m, ok := cur.(map[string]any)
		if !ok {
			return ""
		}
		cur, ok = m[p]
		if !ok {
			return ""
		}
	}
	return value.Text(st, cur)
}
