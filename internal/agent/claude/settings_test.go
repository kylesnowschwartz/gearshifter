package claude

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kylesnowschwartz/agent-ouija/claude/claudedir"

	"github.com/kylesnowschwartz/gearshifter/internal/agent"
	"github.com/kylesnowschwartz/gearshifter/internal/testutil"
)

// TestMain sandboxes HOME (shared invariant; fixtures use t.TempDir()).
func TestMain(m *testing.M) {
	os.Exit(testutil.SandboxHome(m))
}

// testRoot mirrors New's home→Root derivation so fixtures and the code
// under test address the same directory.
func testRoot(home string) claudedir.Root {
	return claudedir.Root(filepath.Join(home, ".claude"))
}

func writeSettings(t *testing.T, content string) string {
	t.Helper()
	home := t.TempDir()
	root := testRoot(home)
	if err := os.MkdirAll(root.String(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(root.SettingsPath(), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return home
}

func TestReadSettings(t *testing.T) {
	home := writeSettings(t, `{"model":"claude-fable-5[1m]","effortLevel":"high","outputStyle":"butterfield","env":{"SECRET":"x"}}`)
	s := readSettings(testRoot(home))
	if s.Model != "claude-fable-5[1m]" || s.Effort != "high" || s.Style != "butterfield" {
		t.Errorf("got %+v, want model claude-fable-5[1m] / effort high / style butterfield", s)
	}
}

func TestReadSettingsDegradesToZero(t *testing.T) {
	if s := readSettings(testRoot(t.TempDir())); s != (agent.State{}) {
		t.Errorf("missing file: got %+v, want zero", s)
	}
	if s := readSettings(testRoot(writeSettings(t, "{not json"))); s != (agent.State{}) {
		t.Errorf("malformed file: got %+v, want zero", s)
	}
	if s := readSettings(testRoot(writeSettings(t, `{"statusLine":"x"}`))); s != (agent.State{}) {
		t.Errorf("fields absent: got %+v, want zero", s)
	}
}
