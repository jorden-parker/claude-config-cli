package schema

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "ccfg-schema-test")
	if err != nil {
		panic(err)
	}
	os.Setenv(EnvCacheDir, dir)
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

func writeCacheFile(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "schema.json")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

const cacheBody = `{"format":1,"generatedOn":"2026-10-15","settings":[{"key":"model","section":"Model","kind":"string","desc":"from cache"}],"envVars":[]}`

func TestLoadPrefersValidCache(t *testing.T) {
	p := writeCacheFile(t, cacheBody)
	s, o := load(p, raw, time.Now())
	if !o.Cached || o.Warning != "" {
		t.Fatalf("origin = %+v", o)
	}
	if st := s.Get("model"); st == nil || st.Desc != "from cache" {
		t.Fatalf("cache not used: %+v", st)
	}
}

func TestLoadIgnoresCorruptCache(t *testing.T) {
	p := writeCacheFile(t, "{not json")
	s, o := load(p, raw, time.Now())
	if o.Cached || !strings.Contains(o.Warning, "ignoring cache") {
		t.Fatalf("origin = %+v", o)
	}
	if len(s.Settings) < 200 {
		t.Fatal("embedded catalogue not used")
	}
}

func TestLoadIgnoresOldFormatCache(t *testing.T) {
	p := writeCacheFile(t, strings.Replace(cacheBody, `"format":1`, `"format":0`, 1))
	_, o := load(p, raw, time.Now())
	if o.Cached || !strings.Contains(o.Warning, "format 0") {
		t.Fatalf("origin = %+v", o)
	}
}

func TestLoadMissingCacheUsesEmbedded(t *testing.T) {
	s, o := load(filepath.Join(t.TempDir(), "none.json"), raw, time.Now())
	if o.Cached || o.Warning != "" || len(s.Settings) < 200 {
		t.Fatalf("origin = %+v, %d settings", o, len(s.Settings))
	}
}

func TestLoadEmptyPathSkipsCache(t *testing.T) {
	if _, o := load("", raw, time.Now()); o.Cached || o.Path != "" {
		t.Fatalf("origin = %+v", o)
	}
}

func TestCachePathHonoursEnv(t *testing.T) {
	t.Setenv(EnvCacheDir, "/tmp/x")
	if p, _ := CachePath(); p != filepath.Join("/tmp/x", "schema.json") {
		t.Fatal(p)
	}
	t.Setenv(EnvCacheDir, "")
	p, err := CachePath()
	if err != nil || !strings.HasSuffix(p, filepath.Join("ccfg", "schema.json")) {
		t.Fatal(p, err)
	}
}

func TestWriteCacheRejectsInvalid(t *testing.T) {
	t.Setenv(EnvCacheDir, t.TempDir())
	for _, body := range []string{"nope", `{"format":1,"settings":[]}`, strings.Replace(cacheBody, `"format":1`, `"format":9`, 1)} {
		if _, err := WriteCache([]byte(body)); err == nil {
			t.Fatalf("accepted %q", body)
		}
	}
	p, err := WriteCache([]byte(cacheBody))
	if err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(p); string(b) != cacheBody {
		t.Fatal("cache contents differ")
	}
}

func TestStale(t *testing.T) {
	now := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	for date, want := range map[string]bool{"2026-10-09": false, "2026-10-07": true, "garbage": false, "": false} {
		if got := (&Schema{GeneratedOn: date}).Stale(now); got != want {
			t.Errorf("Stale(%q) = %v", date, got)
		}
	}
}

func TestParseRejectsEmpty(t *testing.T) {
	if _, err := Parse([]byte(`{"settings":[]}`)); err == nil {
		t.Fatal("expected error")
	}
	if _, err := Parse([]byte(`[]`)); err == nil {
		t.Fatal("expected error")
	}
}
