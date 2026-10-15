package schema

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/jorden-parker/claude-config-cli/internal/atomicfile"
)

// StaleAfter is how old a catalogue may be before the app suggests a sync.
const StaleAfter = 7 * 24 * time.Hour

// EnvCacheDir overrides where the synced catalogue lives. Tests set it so
// they never read or write the real cache.
const EnvCacheDir = "CCFG_CACHE_DIR"

// Origin says where Load got the catalogue from.
type Origin struct {
	// Cached is true when the synced copy was used instead of the embedded one.
	Cached bool
	// Path is the cache file that was used, or would have been.
	Path string
	// Warning explains why a present cache file was ignored, if it was.
	Warning string
}

var cacheEnabled = true

// SetCacheEnabled turns the synced catalogue off for this process. It must
// run before the first Load; later calls have no effect.
func SetCacheEnabled(on bool) { cacheEnabled = on }

// CacheEnabled reports whether Load may use the synced catalogue.
func CacheEnabled() bool { return cacheEnabled }

// CachePath is where "sync" stores the downloaded catalogue:
// $CCFG_CACHE_DIR/schema.json, else the user cache directory under ccfg/.
func CachePath() (string, error) {
	if dir := os.Getenv(EnvCacheDir); dir != "" {
		return filepath.Join(dir, "schema.json"), nil
	}
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "ccfg", "schema.json"), nil
}

// Source reports where Load took the catalogue from. Before Load it is empty.
func Source() Origin { return origin }

// Embedded returns the catalogue compiled into this build.
func Embedded() []byte { return raw }

// WriteCache validates b as a catalogue of the current format and stores it
// at CachePath. It returns the path written.
func WriteCache(b []byte) (string, error) {
	s, err := Parse(b)
	if err != nil {
		return "", fmt.Errorf("refusing to cache an invalid catalogue: %w", err)
	}
	if s.Format != CurrentFormat {
		return "", fmt.Errorf("refusing to cache a format %d catalogue; this build reads format %d", s.Format, CurrentFormat)
	}
	path, err := CachePath()
	if err != nil {
		return "", err
	}
	return path, atomicfile.Write(path, b)
}

// load picks the cache at path when it parses and has the current format,
// else the embedded catalogue. An empty path skips the cache.
func load(path string, embedded []byte, now time.Time) (*Schema, Origin) {
	o := Origin{Path: path}
	if path != "" {
		b, err := os.ReadFile(path)
		switch {
		case errors.Is(err, os.ErrNotExist):
		case err != nil:
			o.Warning = fmt.Sprintf("ignoring cache %s: %v", path, err)
		default:
			s, err := Parse(b)
			switch {
			case err != nil:
				o.Warning = fmt.Sprintf("ignoring cache %s: %v", path, err)
			case s.Format != CurrentFormat:
				o.Warning = fmt.Sprintf("ignoring cache %s: format %d, this build reads format %d", path, s.Format, CurrentFormat)
			default:
				o.Cached = true
				return s, o
			}
		}
	}
	s, err := Parse(embedded)
	if err != nil {
		panic("schema: embedded schema.json is invalid: " + err.Error())
	}
	return s, o
}

// Generated is the day the catalogue was built, if GeneratedOn parses.
func (s *Schema) Generated() (time.Time, bool) {
	t, err := time.Parse("2006-01-02", s.GeneratedOn)
	return t, err == nil
}

// Stale reports whether the catalogue is older than StaleAfter. A catalogue
// without a readable date is never stale.
func (s *Schema) Stale(now time.Time) bool {
	t, ok := s.Generated()
	return ok && now.Sub(t) > StaleAfter
}
