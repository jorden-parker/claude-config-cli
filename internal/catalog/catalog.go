package catalog

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/jorden-parker/claude-config-cli/internal/schema"
)

// GeneratedFrom is recorded in every catalogue.
const GeneratedFrom = "https://code.claude.com/docs/en/settings-reference.md"

// Inputs are the three doc pages as downloaded.
type Inputs struct {
	Ref, Env, Tools []byte
}

// Options tune a build.
type Options struct {
	// Today stamps GeneratedOn; zero means time.Now.
	Today time.Time
	// ListedTools are the names the Tools pane shows. When set, Build fails
	// if the tools reference lists different deniable tools.
	ListedTools []string
}

// Build parses the inputs into a catalogue. Removed settings stay in
// Settings; schema.Parse separates them later.
func Build(in Inputs, opt Options) (*schema.Schema, error) {
	entries, err := extract(string(in.Ref))
	if err != nil {
		return nil, err
	}
	settings, err := buildSettings(entries)
	if err != nil {
		return nil, err
	}
	env, dropped, err := parseEnv(string(in.Env))
	if err != nil {
		return nil, err
	}
	if _, err := parseToolsTable(string(in.Tools)); err != nil {
		return nil, err
	}
	if opt.ListedTools != nil {
		if err := CheckTools(in.Tools, opt.ListedTools); err != nil {
			return nil, err
		}
	}
	today := opt.Today
	if today.IsZero() {
		today = time.Now()
	}
	return &schema.Schema{
		Format:         schema.CurrentFormat,
		GeneratedFrom:  GeneratedFrom,
		GeneratedOn:    today.Format("2006-01-02"),
		Settings:       settings,
		EnvVars:        env,
		RemovedEnvVars: dropped,
		HookEvents:     hookEvents,
		ModelAliases:   modelAliases,
	}, nil
}

// Encode renders a catalogue as indented JSON with a trailing newline.
func Encode(s *schema.Schema) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(s); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Decode reads a catalogue without building indexes or separating removed
// settings, so it can be compared field by field.
func Decode(b []byte) (*schema.Schema, error) {
	s := &schema.Schema{}
	if err := json.Unmarshal(b, s); err != nil {
		return nil, err
	}
	return s, nil
}

// Pages are the doc pages Build needs, in Inputs order.
var Pages = [3]string{"settings-reference.md", "env-vars.md", "tools-reference.md"}

// ReadDir loads the pages from a directory holding ref.md, env.md, tools.md
// or the files named in Pages.
func ReadDir(dir string) (Inputs, error) {
	var in Inputs
	for i, dst := range []*[]byte{&in.Ref, &in.Env, &in.Tools} {
		short := []string{"ref.md", "env.md", "tools.md"}[i]
		b, err := os.ReadFile(filepath.Join(dir, short))
		if err != nil {
			var err2 error
			if b, err2 = os.ReadFile(filepath.Join(dir, Pages[i])); err2 != nil {
				return in, fmt.Errorf("%s: need %s or %s: %w", dir, short, Pages[i], err)
			}
		}
		*dst = b
	}
	return in, nil
}
