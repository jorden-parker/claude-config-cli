package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/jorden-parker/claude-config-cli/internal/atomicfile"
	"github.com/jorden-parker/claude-config-cli/internal/catalog"
	"github.com/jorden-parker/claude-config-cli/internal/schema"
	"github.com/jorden-parker/claude-config-cli/internal/tui"
)

// errDrift makes "sync --check" exit non-zero without a usage dump.
var errDrift = errors.New("the catalogue differs from the docs")

func syncCmd() *cobra.Command {
	var (
		check, embed bool
		out, from    string
		base         string
		timeout      time.Duration
	)
	c := &cobra.Command{
		Use:   "sync",
		Short: "Refresh the settings catalogue from the official docs",
		Long: `Download the settings reference, the environment variables page, and the
tools reference, rebuild the catalogue, and store it in the cache. Every
command then lists the settings the docs list today, without a new build.

  ` + name + ` sync            fetch and update the cache, printing what changed
  ` + name + ` sync --check    fetch and report differences only (exit 1 on drift)
  ` + name + ` sync --embed    maintainers: rewrite internal/schema/schema.json

The cache lives at the path "` + name + ` paths" shows under catalogue; set
CCFG_CACHE_DIR to move it, or pass --no-cache to ignore it for one run.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			var in catalog.Inputs
			var err error
			if from != "" {
				in, err = catalog.ReadDir(from)
			} else {
				ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
				defer cancel()
				in, err = catalog.Fetch(ctx, &http.Client{Timeout: timeout}, base)
			}
			if err != nil {
				return err
			}
			built, err := catalog.Build(in, catalog.Options{Today: now()})
			if err != nil {
				return fmt.Errorf("the docs changed shape; this build can't read them: %w", err)
			}
			if err := catalog.CheckTools(in.Tools, tui.ToolNames()); err != nil {
				if embed {
					return err
				}
				fmt.Fprintf(cmd.ErrOrStderr(), "note: %v (the Tools pane is part of the binary; everything else is up to date)\n", err)
			}
			data, err := catalog.Encode(built)
			if err != nil {
				return err
			}
			// The catalogue this run compares against: the embed target, else
			// whatever Load would use now.
			prevPath := out
			if !embed {
				prevPath, _ = schema.CachePath()
			}
			prev, err := os.ReadFile(prevPath)
			if errors.Is(err, os.ErrNotExist) && !embed {
				prev = schema.Embedded()
			} else if err != nil {
				return err
			}
			var diff []string
			if old, err := catalog.Decode(prev); err != nil {
				diff = []string{fmt.Sprintf("current catalogue unreadable (%v)", err)}
			} else {
				diff = catalog.Diff(old, built)
			}
			w := cmd.OutOrStdout()
			current := 0
			for _, st := range built.Settings {
				if !st.Removed {
					current++
				}
			}
			fmt.Fprintf(w, "catalogue from the docs: %d settings, %d environment variables (%s)\n", current, len(built.EnvVars), built.GeneratedOn)
			if len(diff) == 0 {
				fmt.Fprintln(w, "no changes")
			}
			for _, d := range diff {
				fmt.Fprintln(w, "  "+d)
			}
			switch {
			case check:
				if len(diff) > 0 {
					return errDrift
				}
				return nil
			case embed:
				if _, err := os.Stat(filepath.Dir(out)); err != nil {
					return fmt.Errorf("--embed writes into a source checkout: %w", err)
				}
				if bytes.Equal(prev, data) {
					return nil
				}
				if err := atomicfile.Write(out, data); err != nil {
					return err
				}
				fmt.Fprintf(w, "wrote %s\n", out)
				return nil
			default:
				path, err := schema.WriteCache(data)
				if err != nil {
					return err
				}
				fmt.Fprintf(w, "saved to %s\n", path)
				return nil
			}
		},
	}
	c.Flags().BoolVar(&check, "check", false, "report differences and exit 1 on drift; write nothing")
	c.Flags().BoolVar(&embed, "embed", false, "write the embedded catalogue for a new build instead of the cache")
	c.Flags().StringVar(&out, "out", filepath.Join("internal", "schema", "schema.json"), "file --embed writes")
	c.Flags().StringVar(&from, "from", "", "read the pages from this directory instead of downloading")
	c.Flags().DurationVar(&timeout, "timeout", 30*time.Second, "download timeout")
	c.Flags().StringVar(&base, "base", catalog.DefaultBase, "docs base URL")
	c.Flags().MarkHidden("base")
	return c
}

// catalogueNotes prints one line about the catalogue after a command when
// something deserves attention: the cache was ignored, or it is old.
func catalogueNotes(cmd *cobra.Command) {
	switch cmd.Name() {
	case "sync", "help", "completion", "man":
		return
	}
	if !schema.Loaded() {
		return
	}
	w := cmd.ErrOrStderr()
	if warn := schema.Source().Warning; warn != "" {
		fmt.Fprintf(w, "%s: %s\n", name, warn)
	}
	s := schema.Load()
	if s.Stale(now()) {
		if t, ok := s.Generated(); ok {
			days := int(now().Sub(t).Hours() / 24)
			fmt.Fprintf(w, "%s: the settings catalogue is %d days old; run \"%s sync\" to refresh it from the docs\n", name, days, name)
		}
	}
}
