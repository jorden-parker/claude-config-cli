package snapshot

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setup(t *testing.T) (string, string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	return t.TempDir(), filepath.Join(t.TempDir(), "snapshot.json")
}

func put(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestRoundTripAfterDeletingConfiguration(t *testing.T) {
	project, archive := setup(t)
	original := map[string][]byte{}
	for key, path := range paths(project) {
		data := []byte("{\n  \"unknown\": 9007199254740993, \"env\": {\"SECRET\": \"a b\"}, \"name\": \"" + key + "\"\n}\n")
		original[path] = data
		put(t, path, data)
	}
	n, err := Save(archive, project)
	if err != nil || n != 7 {
		t.Fatalf("save = %d, %v", n, err)
	}
	info, _ := os.Stat(archive)
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("permissions: %v", info.Mode())
	}
	for path := range original {
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
	}
	// Remove the directories too, as a configuration reset would.
	os.Remove(filepath.Join(project, ".claude"))
	os.Remove(filepath.Join(os.Getenv("HOME"), ".claude"))
	n, err = Restore(archive, "", false)
	if err != nil || n != 7 {
		t.Fatalf("restore = %d, %v", n, err)
	}
	for path, want := range original {
		got, err := os.ReadFile(path)
		if err != nil || string(got) != string(want) {
			t.Fatalf("%s: got %q, %v", path, got, err)
		}
	}
}

func TestOverwriteProtectionAndForce(t *testing.T) {
	project, archive := setup(t)
	targets := paths(project)
	put(t, targets["user"], []byte(`{"model":"saved"}`))
	put(t, targets["project"], []byte(`{}`))
	if _, err := Save(archive, project); err != nil {
		t.Fatal(err)
	}
	if _, err := Save(archive, project); err == nil {
		t.Fatal("overwrote snapshot")
	}
	put(t, targets["user"], []byte(`{"model":"new"}`))
	os.Remove(targets["project"])
	if _, err := Restore(archive, "", false); err == nil {
		t.Fatal("overwrote settings")
	}
	if _, err := os.Stat(targets["project"]); !os.IsNotExist(err) {
		t.Fatal("wrote before conflict check")
	}
	put(t, targets["local"], []byte(`{"unrelated":true}`))
	if _, err := Restore(archive, "", true); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(targets["user"])
	if string(data) != `{"model":"saved"}` {
		t.Fatalf("got %s", data)
	}
	data, _ = os.ReadFile(targets["local"])
	if string(data) != `{"unrelated":true}` {
		t.Fatal("changed unsaved file")
	}
}

func TestInvalidArchivesWriteNothing(t *testing.T) {
	for _, kind := range []string{"version", "unknown", "json", "empty", "project"} {
		t.Run(kind, func(t *testing.T) {
			project, archive := setup(t)
			a := Archive{Version: 1, Project: project, Files: map[string][]byte{"user": []byte(`{}`)}}
			switch kind {
			case "version":
				a.Version = 2
			case "unknown":
				a.Files["../../elsewhere"] = []byte(`{}`)
			case "json":
				a.Files["project"] = []byte(`null`)
			case "empty":
				a.Files = nil
			case "project":
				a.Project = "relative"
			}
			data, _ := json.Marshal(a)
			put(t, archive, data)
			if _, err := Restore(archive, "", true); err == nil {
				t.Fatal("accepted invalid archive")
			}
			if _, err := os.Stat(paths(project)["user"]); !os.IsNotExist(err) {
				t.Fatal("wrote invalid archive")
			}
		})
	}
}

func TestSnapshotLocationAndMissingSettings(t *testing.T) {
	project, archive := setup(t)
	if _, err := Save(archive, project); err == nil {
		t.Fatal("saved empty snapshot")
	}
	put(t, paths(project)["user"], []byte(`{}`))
	inside := filepath.Join(os.Getenv("HOME"), ".claude", "backup.json")
	if _, err := Save(inside, project); err == nil || !strings.Contains(err.Error(), "outside") {
		t.Fatalf("inside: %v", err)
	}
	link := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(filepath.Dir(inside), link); err != nil {
		t.Fatal(err)
	}
	if _, err := Save(filepath.Join(link, "backup.json"), project); err == nil {
		t.Fatal("accepted symlink into config")
	}
	put(t, paths(project)["user"], []byte(`not json`))
	if _, err := Save(archive, project); err == nil {
		t.Fatal("saved invalid settings")
	}
}

func TestProjectOverride(t *testing.T) {
	project, archive := setup(t)
	put(t, paths(project)["local"], []byte(`{"model":"saved"}`))
	if _, err := Save(archive, project); err != nil {
		t.Fatal(err)
	}
	destination := t.TempDir()
	if _, err := Restore(archive, destination, false); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(paths(destination)["local"])
	if err != nil || string(data) != `{"model":"saved"}` {
		t.Fatalf("got %q, %v", data, err)
	}
}

func TestHomeAsProject(t *testing.T) {
	_, archive := setup(t)
	project := os.Getenv("HOME")
	path := paths(project)["user"]
	put(t, path, []byte(`{"model":"saved"}`))
	if _, err := Save(archive, project); err != nil {
		t.Fatal(err)
	}
	os.Remove(path)
	if n, err := Restore(archive, "", false); err != nil || n != 1 {
		t.Fatalf("restore = %d, %v", n, err)
	}
}

func TestSymlinkedProjectAliasesUserScope(t *testing.T) {
	_, archive := setup(t)
	home := os.Getenv("HOME")
	link := filepath.Join(t.TempDir(), "project-link")
	if err := os.Symlink(home, link); err != nil {
		t.Fatal(err)
	}
	// Save with the alias: user and project keys hold identical bytes.
	put(t, paths(home)["user"], []byte(`{"model":"saved"}`))
	if _, err := Save(archive, link); err != nil {
		t.Fatal(err)
	}
	os.Remove(paths(home)["user"])
	os.Remove(filepath.Join(home, ".claude"))
	// Non-forced restore must succeed once, not fail on the second alias.
	if n, err := Restore(archive, "", false); err != nil || n != 1 {
		t.Fatalf("restore = %d, %v", n, err)
	}
}

func TestConflictingAliasContentsAreRejected(t *testing.T) {
	_, archive := setup(t)
	home := os.Getenv("HOME")
	link := filepath.Join(t.TempDir(), "project-link")
	if err := os.Symlink(home, link); err != nil {
		t.Fatal(err)
	}
	a := Archive{Version: 1, Project: link, Files: map[string][]byte{
		"user":    []byte(`{"model":"a"}`),
		"project": []byte(`{"model":"b"}`),
	}}
	data, _ := json.Marshal(a)
	put(t, archive, data)
	if _, err := Restore(archive, "", true); err == nil || !strings.Contains(err.Error(), "conflicting") {
		t.Fatalf("got %v", err)
	}
	if _, err := os.Stat(paths(home)["user"]); !os.IsNotExist(err) {
		t.Fatal("wrote before rejecting the conflict")
	}
}
