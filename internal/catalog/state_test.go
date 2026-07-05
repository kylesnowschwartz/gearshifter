package catalog

import (
	"os"
	"path/filepath"
	"testing"
)

func writeSettings(t *testing.T, content string) string {
	t.Helper()
	home := t.TempDir()
	dir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return home
}

func TestReadGearState(t *testing.T) {
	home := writeSettings(t, `{"model":"claude-fable-5[1m]","effortLevel":"high","env":{"SECRET":"x"}}`)
	s := ReadGearState(home)
	if s.Model != "claude-fable-5[1m]" || s.Effort != "high" {
		t.Errorf("got %+v, want model claude-fable-5[1m] effort high", s)
	}
}

func TestReadGearStateDegradesToZero(t *testing.T) {
	if s := ReadGearState(t.TempDir()); s != (GearState{}) {
		t.Errorf("missing file: got %+v, want zero", s)
	}
	if s := ReadGearState(writeSettings(t, "{not json")); s != (GearState{}) {
		t.Errorf("malformed file: got %+v, want zero", s)
	}
	if s := ReadGearState(writeSettings(t, `{"outputStyle":"x"}`)); s != (GearState{}) {
		t.Errorf("fields absent: got %+v, want zero", s)
	}
}
