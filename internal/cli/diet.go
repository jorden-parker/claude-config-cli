package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/jorden-parker/claude-config-cli/internal/diet"
	"github.com/jorden-parker/claude-config-cli/internal/store"
)

// dietCmd prints the context diet checklist. It is the text twin of the
// editor's Context diet pane and reads the same checks.
func dietCmd() *cobra.Command {
	var asJSON bool
	c := &cobra.Command{
		Use:   "diet",
		Short: "Show the context diet checklist: what is applied, and what is a trap",
		Long: `Check each row of the context diet checklist against your settings files and
print its state: applied (with the file it is set in), not applied, warning (a
trap is set), or info (ccfg cannot check it from files). Nothing is changed.

Exit status is 0 whatever the result. The checklist is informational.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			files, errs := store.OpenAll(cwd())
			in := diet.Input{Files: map[store.Scope]*store.File{}, Cwd: cwd(), Home: homeDir()}
			for sc, f := range files {
				if err := errs[sc]; err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "%s: %s file not read, so it is left out: %v\n", name, sc, err)
					continue
				}
				in.Files[sc] = f
			}
			rows := diet.Evaluate(in)
			if asJSON {
				return writeDietJSON(cmd.OutOrStdout(), rows)
			}
			writeDietText(cmd.OutOrStdout(), rows)
			return nil
		},
	}
	c.Flags().BoolVar(&asJSON, "json", false, "print the checklist as JSON for scripts")
	return c
}

func homeDir() string {
	h, _ := os.UserHomeDir()
	return h
}

func dietGlyph(st diet.State) string {
	switch st {
	case diet.Applied:
		return "✓"
	case diet.Warning:
		return "▲"
	case diet.Info:
		return "◇"
	}
	return "○"
}

func writeDietText(w io.Writer, rows []diet.Row) {
	fmt.Fprintf(w, "Context diet: %s\n", diet.Summarize(rows).Line())
	group := ""
	for _, r := range rows {
		if r.Rec.Group != group {
			group = r.Rec.Group
			fmt.Fprintf(w, "\n%s\n", diet.GroupTitle(group))
		}
		res := r.Result
		fmt.Fprintf(w, "  %s %-11s %-8s %s\n", dietGlyph(res.State), res.State, res.Where(), r.Rec.Title)
		fmt.Fprintf(w, "      %s\n", r.Rec.Why)
	}
}

// dietRowJSON is one checklist row for scripts. Every key is always present.
type dietRowJSON struct {
	ID         string   `json:"id"`
	Title      string   `json:"title"`
	Group      string   `json:"group"`
	State      string   `json:"state"`
	Where      string   `json:"where"`
	Note       string   `json:"note"`
	Why        string   `json:"why"`
	Caveat     string   `json:"caveat"`
	Apply      string   `json:"apply"`
	URL        string   `json:"url"`
	Confidence string   `json:"confidence"`
	Offenders  []string `json:"offenders"`
}

func writeDietJSON(w io.Writer, rows []diet.Row) error {
	out := make([]dietRowJSON, 0, len(rows))
	for _, r := range rows {
		offenders := []string{}
		for _, o := range r.Result.Offenders {
			offenders = append(offenders, o.Key)
		}
		out = append(out, dietRowJSON{
			ID: r.Rec.ID, Title: r.Rec.Title, Group: r.Rec.Group,
			State: r.Result.State.String(), Where: r.Result.Where(), Note: r.Result.Note,
			Why: r.Rec.Why, Caveat: r.Rec.Caveat, Apply: r.Rec.Apply.Text,
			URL: r.Rec.URL, Confidence: r.Rec.Confidence, Offenders: offenders,
		})
	}
	b, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(w, string(b))
	return err
}
