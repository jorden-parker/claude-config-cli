package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/jorden-parker/claude-config-cli/internal/perm"
	"github.com/jorden-parker/claude-config-cli/internal/store"
)

func checkCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "check",
		Short: "Report permission rules that contradict each other",
		Long: `Read every settings file and list the permission rules that can never take
effect: a rule that is in two of allow, ask, and deny, or a rule for a tool
that another rule in the same file denies outright. Nothing is changed.

Exit status is 1 when any conflict is found.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			files, errs := store.OpenAll(cwd())
			w := cmd.OutOrStdout()
			n := 0
			for _, sc := range store.All {
				if err := errs[sc]; err != nil {
					fmt.Fprintf(w, "%-8s %s\n  can't read: %v\n", sc, files[sc].Path, err)
					n++
					continue
				}
				conflicts, err := perm.Check(files[sc])
				if err != nil {
					fmt.Fprintf(w, "%-8s %s\n  %v\n", sc, files[sc].Path, err)
					n++
					continue
				}
				if len(conflicts) == 0 {
					continue
				}
				fmt.Fprintf(w, "%-8s %s\n", sc, files[sc].Path)
				for _, c := range conflicts {
					fmt.Fprintf(w, "  %s\n", c)
					n++
				}
			}
			if n == 0 {
				fmt.Fprintln(w, "no permission conflicts")
				return nil
			}
			return fmt.Errorf("%d permission conflict(s)", n)
		},
	}
}
