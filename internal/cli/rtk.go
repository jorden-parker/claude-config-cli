package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/jorden-parker/claude-config-cli/internal/rtk"
	"github.com/jorden-parker/claude-config-cli/internal/statusline"
	"github.com/jorden-parker/claude-config-cli/internal/store"
)

func rtkCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "rtk",
		Short: "Route command output through rtk to save tokens",
		Long: `Route command output through rtk (https://github.com/rtk-ai/rtk), which
condenses what Claude reads. Two PreToolUse hooks go into the chosen
settings file:

  Bash  ` + rtk.BashHook + `   rtk's own hook; rewrites cat, grep, pnpm, bun, ...
  Read  ` + name + ` rtk read-hook   refuses the built-in Read tool and names the
                            shell command to run instead

Edits keep working: Claude Code counts a plain cat, head, tail, sed -n, or
grep on one file as the read before an Edit. The Read hook suggests sed -n,
which rtk leaves alone.

Install rtk first (brew install rtk). For rtk's awareness notes in CLAUDE.md,
also run: rtk init -g --no-patch`,
	}
	c.PersistentFlags().StringVarP(&flagScope, "scope", "s", "", "user | project | local")
	c.AddCommand(rtkOnCmd(), rtkOffCmd(), rtkStatusCmd(), rtkReadHookCmd())
	return c
}

func rtkTarget() (*store.File, store.Scope, error) {
	scope, err := store.ParseScope(flagScope)
	if err != nil {
		return nil, "", err
	}
	if scope == store.ScopeGlobal || scope == store.ScopeManaged {
		return nil, "", fmt.Errorf("hooks go in user, project, or local settings, not %s", scope)
	}
	f, err := store.Open(scope, cwd())
	return f, scope, err
}

func rtkOnCmd() *cobra.Command {
	var keepRead bool
	c := &cobra.Command{
		Use:   "on",
		Short: "Add the rtk hooks to the target settings file",
		Example: "  " + name + " rtk on\n" +
			"  " + name + " rtk on --keep-read -s project",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := rtk.Binary(); err != nil {
				return err
			}
			f, scope, err := rtkTarget()
			if err != nil {
				return err
			}
			readCommand := ""
			if !keepRead {
				readCommand = rtk.ReadCommand(scope, statusline.ProgramPath())
			}
			if err := rtk.Enable(f, readCommand); err != nil {
				return err
			}
			if err := f.Save(); err != nil {
				return err
			}
			fmt.Printf("rtk on  →  %s\nBash: %s\n", f.Path, rtk.BashHook)
			if readCommand != "" {
				fmt.Printf("Read: %s\n", readCommand)
			} else {
				fmt.Println("Read: left as it is")
			}
			fmt.Println("Restart Claude Code; hooks are read once at session start.")
			return nil
		},
	}
	c.Flags().BoolVar(&keepRead, "keep-read", false, "leave the built-in Read tool on; only rewrite Bash")
	return c
}

func rtkOffCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "off",
		Short: "Remove the rtk hooks from the target settings file",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			f, _, err := rtkTarget()
			if err != nil {
				return err
			}
			st := rtk.On(f)
			if !st.Bash && !st.Read {
				fmt.Printf("rtk was not on in %s\n", f.Path)
				return nil
			}
			rtk.Disable(f)
			if err := f.Save(); err != nil {
				return err
			}
			fmt.Printf("rtk off  →  %s\nother hooks were left alone\n", f.Path)
			return nil
		},
	}
}

func rtkStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show which settings files carry the rtk hooks",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if p, err := rtk.Binary(); err != nil {
				fmt.Println("rtk binary: not found")
			} else {
				fmt.Printf("rtk binary: %s\n", p)
			}
			files, _ := store.OpenAll(cwd())
			for _, sc := range []store.Scope{store.ScopeUser, store.ScopeProject, store.ScopeLocal, store.ScopeManaged} {
				f := files[sc]
				if f == nil {
					continue
				}
				st := rtk.On(f)
				fmt.Printf("%-8s Bash %s  Read %s  %s\n", sc, onOff(st.Bash), onOff(st.Read), f.Path)
			}
			return nil
		},
	}
}

func onOff(b bool) string {
	if b {
		return "on "
	}
	return "off"
}

func rtkReadHookCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "read-hook",
		Short:  "PreToolUse hook for the Read tool (run by Claude Code)",
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return rtk.ReadHook(os.Stdin, os.Stdout)
		},
	}
}
