package claude

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kylesnowschwartz/gearshifter/internal/agent"
	"github.com/kylesnowschwartz/gearshifter/internal/testutil"
)

// TestMain sandboxes HOME (shared invariant; fixtures use t.TempDir()).
func TestMain(m *testing.M) {
	os.Exit(testutil.SandboxHome(m))
}

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

func TestReadSettings(t *testing.T) {
	home := writeSettings(t, `{"model":"claude-fable-5[1m]","effortLevel":"high","outputStyle":"butterfield","env":{"SECRET":"x"}}`)
	s := readSettings(home)
	if s.Model != "claude-fable-5[1m]" || s.Effort != "high" || s.Style != "butterfield" {
		t.Errorf("got %+v, want model claude-fable-5[1m] / effort high / style butterfield", s)
	}
}

func TestReadSettingsDegradesToZero(t *testing.T) {
	if s := readSettings(t.TempDir()); s != (agent.State{}) {
		t.Errorf("missing file: got %+v, want zero", s)
	}
	if s := readSettings(writeSettings(t, "{not json")); s != (agent.State{}) {
		t.Errorf("malformed file: got %+v, want zero", s)
	}
	if s := readSettings(writeSettings(t, `{"statusLine":"x"}`)); s != (agent.State{}) {
		t.Errorf("fields absent: got %+v, want zero", s)
	}
}
