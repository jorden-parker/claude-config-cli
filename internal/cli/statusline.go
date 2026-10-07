package cli

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/jorden-parker/claude-config-cli/internal/statusline"
	"github.com/jorden-parker/claude-config-cli/internal/store"
)

func statuslineCmd() *cobra.Command {
	c := &cobra.Command{
		Use:     "statusline",
		Aliases: []string{"sl"},
		Short:   "Build a status line that keeps the one you already have",
		Long: `Build a status line from the fields Claude Code documents at
` + statusline.DocURL + `

Turning it on points statusLine at ` + name + `'s renderer. A status line command
that is already set is kept: the renderer runs it with the same input and
shows its output next to the fields you pick. Turning it off puts the
original statusLine value back.`,
	}
	c.PersistentFlags().StringVarP(&flagScope, "scope", "s", "", "user | project | local")
	c.AddCommand(statuslineOnCmd(), statuslineOffCmd(), statuslinePreviewCmd(), statuslineFieldsCmd(), statuslineRenderCmd())
	return c
}

// statuslineTarget loads the settings files and the config for the -s scope.
func statuslineTarget() (map[store.Scope]*store.File, store.Scope, string, *statusline.Config, error) {
	scope, err := store.ParseScope(flagScope)
	if err != nil {
		return nil, "", "", nil, err
	}
	if scope == store.ScopeGlobal || scope == store.ScopeManaged {
		return nil, "", "", nil, fmt.Errorf("a status line can go in user, project, or local settings, not %s", scope)
	}
	files, errs := store.OpenAll(cwd())
	if err := errs[scope]; err != nil {
		return nil, "", "", nil, err
	}
	path := statusline.ConfigPath(scope, cwd())
	cfg, err := statusline.Load(path)
	if err != nil {
		return nil, "", "", nil, fmt.Errorf("%s: %w", path, err)
	}
	return files, scope, path, cfg, nil
}

func statuslineOnCmd() *cobra.Command {
	var fields, existing, separator string
	var noColor bool
	c := &cobra.Command{
		Use:   "on",
		Short: "Use this status line, keeping the existing command's output",
		Example: "  " + name + " statusline on\n" +
			"  " + name + " statusline on --fields model,branch,ctx_used --existing end",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			files, scope, path, cfg, err := statuslineTarget()
			if err != nil {
				return err
			}
			if cmd.Flags().Changed("fields") {
				cfg.Fields = []string{}
				for _, id := range strings.Split(fields, ",") {
					if id = strings.TrimSpace(id); id == "" {
						continue
					}
					if statusline.Lookup(id) == nil {
						return fmt.Errorf("unknown field %q. Run `%s statusline fields` to see every field", id, name)
					}
					if !cfg.Has(id) {
						cfg.Fields = append(cfg.Fields, id)
					}
				}
			}
			if cmd.Flags().Changed("existing") {
				ok := false
				for _, p := range statusline.Positions {
					ok = ok || p == existing
				}
				if !ok {
					return fmt.Errorf("--existing must be one of: %s", strings.Join(statusline.Positions, ", "))
				}
				cfg.Existing = existing
			}
			if cmd.Flags().Changed("separator") {
				cfg.Separator = separator
			}
			if cmd.Flags().Changed("no-color") {
				cfg.Color = !noColor
			}
			f := files[scope]
			kept, keptFrom := statusline.Existing(files, scope, cwd())
			command := statusline.Command(scope, cwd())
			if err := statusline.Enable(f, cfg, command, kept, keptFrom); err != nil {
				return err
			}
			// Check the renderer before the config is published. This does not
			// run the kept command.
			if err := statusline.Check(command, cwd()); err != nil {
				return err
			}
			if err := statusline.Save(path, cfg); err != nil {
				return err
			}
			if err := f.Save(); err != nil {
				return err
			}
			fmt.Printf("status line on  →  %s\nfields: %s\n", f.Path, orNone(strings.Join(cfg.Fields, ", ")))
			if cfg.Inherited != "" {
				fmt.Printf("kept:   %s  (%s)\n", cfg.Inherited, cfg.Existing)
			}
			return nil
		},
	}
	c.Flags().StringVar(&fields, "fields", "", "comma-separated field IDs, in display order")
	c.Flags().StringVar(&existing, "existing", "", "where the existing status line goes: "+strings.Join(statusline.Positions, " | "))
	c.Flags().StringVar(&separator, "separator", "", "text between fields")
	c.Flags().BoolVar(&noColor, "no-color", false, "leave the fields uncoloured")
	return c
}

func statuslineOffCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "off",
		Short: "Put back the status line that was there before",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			files, scope, _, cfg, err := statuslineTarget()
			if err != nil {
				return err
			}
			f := files[scope]
			if !statusline.On(f) {
				fmt.Printf("status line was not on in %s\n", f.Path)
				return nil
			}
			if err := statusline.Disable(f, cfg); err != nil {
				return err
			}
			if err := f.Save(); err != nil {
				return err
			}
			if v, ok := f.Get("statusLine"); ok {
				fmt.Printf("status line off  →  %s\nrestored: %s\n", f.Path, store.Format(v))
			} else {
				fmt.Printf("status line off  →  %s\nremoved statusLine; there was none before\n", f.Path)
			}
			return nil
		},
	}
}

func statuslinePreviewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "preview",
		Short: "Print the status line for the example session in the docs",
		Long: `Print the status line for the example session in the docs. This runs the
existing status line command with that example as its input, the same way
Claude Code runs it.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			files, scope, _, cfg, err := statuslineTarget()
			if err != nil {
				return err
			}
			now := time.Now()
			input := statusline.Sample(cwd(), now)
			kept, _ := statusline.Existing(files, scope, cwd())
			out := ""
			if kept != "" && cfg.Existing != statusline.ExistingHidden {
				if out, err = statusline.RunInherited(kept, input); err != nil {
					fmt.Fprintf(os.Stderr, "existing status line left out (%s): %v\n", kept, err)
				}
			}
			fmt.Println(statusline.Compose(cfg, input, out, now))
			return nil
		},
	}
}

func statuslineFieldsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "fields",
		Short: "List every field, the data it reads, and whether it is shown",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, _, _, cfg, err := statuslineTarget()
			if err != nil {
				return err
			}
			group := ""
			for _, f := range statusline.Fields {
				if f.Group != group {
					group = f.Group
					fmt.Printf("\n%s\n", group)
				}
				mark := " "
				if cfg.Has(f.ID) {
					mark = "●"
				}
				fmt.Printf("  %s %-12s %s\n    %s  (%s)\n", mark, f.ID, f.Name, f.Desc, f.Source)
			}
			return nil
		},
	}
}

func statuslineRenderCmd() *cobra.Command {
	var config string
	var check bool
	c := &cobra.Command{
		Use:   "render",
		Short: "Render the status line from session JSON on stdin (run by Claude Code)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			input, _ := io.ReadAll(os.Stdin)
			if check {
				cmd.SilenceUsage = true
				return statusline.CheckRender(config, input)
			}
			// Always exit 0: a non-zero exit blanks the whole status line.
			fmt.Println(statusline.Render(config, input))
			return nil
		},
	}
	c.Flags().StringVar(&config, "config", "", "status line config file")
	c.Flags().BoolVar(&check, "check", false, "only check that the renderer and config work; never runs the kept command")
	return c
}

func orNone(s string) string {
	if s == "" {
		return "none"
	}
	return s
}
