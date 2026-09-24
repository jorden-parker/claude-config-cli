// Package cli defines the claude-config-cli commands.
package cli

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/charmbracelet/fang"
	"github.com/spf13/cobra"

	"github.com/jorden-parker/claude-config-cli/internal/schema"
	"github.com/jorden-parker/claude-config-cli/internal/store"
	"github.com/jorden-parker/claude-config-cli/internal/tui"
	"github.com/jorden-parker/claude-config-cli/internal/value"
)

var (
	flagScope string
	flagDir   string
	name      = "claude-config-cli"
)

// Main runs the command under the given program name and exits non-zero on
// error. The name shows in help and error text, so the short ccfg binary
// tells users to type ccfg.
func Main(program, version string) {
	name = program
	if err := fang.Execute(context.Background(), Root(), fang.WithVersion(version)); err != nil {
		os.Exit(1)
	}
}

func cwd() string {
	if flagDir != "" {
		return flagDir
	}
	d, _ := os.Getwd()
	return d
}

// Root returns the root command.
func Root() *cobra.Command {
	root := &cobra.Command{
		Use:   name,
		Short: "Browse and edit every Claude Code setting",
		Long: name + ` is a terminal UI and CLI for Claude Code's settings files.
ccfg is its short name; both run the same commands.

Every key, its allowed values, and its default come from the official
settings reference: https://code.claude.com/docs/en/settings-reference

Run with no arguments to open the interactive editor.`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return tui.Run(cwd())
		},
	}
	root.PersistentFlags().StringVarP(&flagDir, "dir", "C", "", "project directory (default: current directory)")

	root.AddCommand(listCmd(), getCmd(), setCmd(), unsetCmd(), docCmd(), showCmd(), envCmd(), pathsCmd())
	return root
}

func listCmd() *cobra.Command {
	var section, search string
	var showAll bool
	c := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List settings keys, optionally filtered by section or text",
		RunE: func(cmd *cobra.Command, args []string) error {
			s := schema.Load()
			var items []*schema.Setting
			switch {
			case section != "":
				for _, sec := range s.Sections() {
					if strings.EqualFold(sec, section) || strings.Contains(strings.ToLower(sec), strings.ToLower(section)) {
						items = append(items, s.BySection(sec)...)
					}
				}
			default:
				items = s.Search(search)
			}
			if len(items) == 0 {
				return fmt.Errorf("no settings match")
			}
			cur := ""
			for _, st := range items {
				if !showAll && st.Deprecated != "" {
					continue
				}
				if st.Section != cur {
					cur = st.Section
					fmt.Printf("\n%s\n", cur)
				}
				fmt.Printf("  %-46s %-15s %s\n", st.Key, st.Kind, truncate(st.Desc, 70))
			}
			return nil
		},
	}
	c.Flags().StringVarP(&section, "section", "s", "", "only this section (substring match)")
	c.Flags().StringVarP(&search, "search", "q", "", "filter by key or description text")
	c.Flags().BoolVar(&showAll, "all", false, "include removed or deprecated keys")
	c.AddCommand(&cobra.Command{
		Use:   "sections",
		Short: "List the section names",
		Run: func(cmd *cobra.Command, args []string) {
			for _, sec := range schema.Load().Sections() {
				fmt.Println(sec)
			}
		},
	})
	return c
}

func getCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get KEY",
		Short: "Show a setting's value in every settings file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := args[0]
			s := schema.Load()
			if s.Get(key) == nil {
				return unknownKey(key)
			}
			files, errs := store.OpenAll(cwd())
			found := false
			for _, sc := range store.All {
				if err := errs[sc]; err != nil {
					fmt.Printf("%-8s error: %v\n", sc, err)
					continue
				}
				if v, ok := files[sc].Get(key); ok {
					found = true
					fmt.Printf("%-8s %s   (%s)\n", sc, store.Format(v), files[sc].Path)
				}
			}
			if !found {
				fmt.Printf("%s is unset in every file. Default: %s\n", key, s.Get(key).Default)
			}
			return nil
		},
	}
}

func setCmd() *cobra.Command {
	var force bool
	c := &cobra.Command{
		Use:   "set KEY VALUE",
		Short: "Set a setting in one file (default: user settings)",
		Long: `Set a setting. VALUE forms per kind:
  bool          true | false
  enum          one of the documented options (see: ` + name + ` doc KEY)
  number        123 or 0.5
  array         JSON array, or comma-separated items
  map           JSON object, or KEY=VALUE (use --stdin for several lines)
  json / group  a JSON document`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := args[0]
			s := schema.Load()
			st := s.Get(key)
			if st == nil {
				return unknownKey(key)
			}
			text := ""
			if len(args) == 2 {
				text = args[1]
			} else {
				b, err := readStdin()
				if err != nil {
					return err
				}
				text = b
			}
			scope, err := pickScope(st)
			if err != nil {
				return err
			}
			if !store.Allowed(st, scope) && !force {
				return fmt.Errorf("setting %s can't go in %s settings (docs: %s). Use --force to write it anyway", key, scope, st.Scope)
			}
			v, err := value.Parse(st, text)
			if err != nil {
				return err
			}
			f, err := store.Open(scope, cwd())
			if err != nil {
				return err
			}
			if err := f.Set(key, v); err != nil {
				return err
			}
			if err := f.Save(); err != nil {
				return err
			}
			fmt.Printf("%s = %s  →  %s\n", key, store.Format(v), f.Path)
			if st.Deprecated != "" {
				fmt.Printf("note: %s\n", st.Deprecated)
			}
			return nil
		},
	}
	c.Flags().StringVarP(&flagScope, "scope", "s", "", "user | project | local | global")
	c.Flags().BoolVar(&force, "force", false, "write even where the docs say the key isn't read")
	return c
}

func unsetCmd() *cobra.Command {
	c := &cobra.Command{
		Use:     "unset KEY",
		Aliases: []string{"rm"},
		Short:   "Remove a setting from one file (default: user settings)",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := args[0]
			st := schema.Load().Get(key)
			if st == nil {
				return unknownKey(key)
			}
			scope, err := pickScope(st)
			if err != nil {
				return err
			}
			f, err := store.Open(scope, cwd())
			if err != nil {
				return err
			}
			if !f.Unset(key) {
				fmt.Printf("%s was not set in %s\n", key, f.Path)
				return nil
			}
			if err := f.Save(); err != nil {
				return err
			}
			fmt.Printf("removed %s from %s\n", key, f.Path)
			return nil
		},
	}
	c.Flags().StringVarP(&flagScope, "scope", "s", "", "user | project | local | global")
	return c
}

func docCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doc KEY",
		Short: "Show the documentation for a setting",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			st := schema.Load().Get(args[0])
			if st == nil {
				return unknownKey(args[0])
			}
			fmt.Print(tui.RenderDoc(st, 100))
			return nil
		},
	}
}

func showCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "show",
		Short: "Print a settings file (default: user settings)",
		RunE: func(cmd *cobra.Command, args []string) error {
			scope, err := store.ParseScope(flagScope)
			if err != nil {
				return err
			}
			f, err := store.Open(scope, cwd())
			if err != nil {
				return err
			}
			if !f.Exists {
				fmt.Printf("%s does not exist\n", f.Path)
				return nil
			}
			fmt.Printf("// %s\n%s\n", f.Path, store.FormatPretty(f.Data))
			return nil
		},
	}
	c.Flags().StringVarP(&flagScope, "scope", "s", "", "user | project | local | global | managed")
	return c
}

func envCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "env [NAME|SEARCH]",
		Short: "List the environment variables Claude Code reads (for the env setting)",
		RunE: func(cmd *cobra.Command, args []string) error {
			s := schema.Load()
			q := ""
			if len(args) > 0 {
				q = strings.ToLower(args[0])
			}
			for _, e := range s.EnvVars {
				if q != "" && !strings.Contains(strings.ToLower(e.Name), q) && !strings.Contains(strings.ToLower(e.Purpose), q) {
					continue
				}
				fmt.Printf("%-50s %s\n", e.Name, truncate(e.Purpose, 110))
			}
			return nil
		},
	}
}

func pathsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "paths",
		Short: "Print where each settings file lives and whether it exists",
		Run: func(cmd *cobra.Command, args []string) {
			files, errs := store.OpenAll(cwd())
			for _, sc := range store.All {
				f := files[sc]
				state := "missing"
				if f.Exists {
					state = fmt.Sprintf("%d keys", len(f.Data))
				}
				if err := errs[sc]; err != nil {
					state = "error: " + err.Error()
				}
				fmt.Printf("%-8s %-10s %s\n", sc, state, f.Path)
			}
		},
	}
}

func pickScope(st *schema.Setting) (store.Scope, error) {
	if flagScope == "" {
		if st.Global {
			return store.ScopeGlobal, nil
		}
		return store.ScopeUser, nil
	}
	return store.ParseScope(flagScope)
}

func unknownKey(key string) error {
	s := schema.Load()
	var near []string
	lk := strings.ToLower(key)
	for _, k := range s.Keys() {
		if strings.Contains(strings.ToLower(k), lk) {
			near = append(near, k)
		}
	}
	sort.Strings(near)
	if len(near) > 0 {
		return fmt.Errorf("unknown setting %q. Did you mean: %s", key, strings.Join(near, ", "))
	}
	return fmt.Errorf("unknown setting %q. Run `%s ls` to see every key", key, name)
}

func readStdin() (string, error) {
	fi, _ := os.Stdin.Stat()
	if fi.Mode()&os.ModeCharDevice != 0 {
		return "", fmt.Errorf("no VALUE given and stdin is a terminal; pass VALUE or pipe it in")
	}
	b, err := os.ReadFile("/dev/stdin")
	return string(b), err
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
