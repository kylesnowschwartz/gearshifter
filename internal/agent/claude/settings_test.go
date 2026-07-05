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

// writeSettings builds a sandbox Claude root holding the given
// settings.json and returns it. Fixture paths derive from the same
// claudedir.Root seam production uses, so writes and reads can't diverge.
func writeSettings(t *testing.T, content string) claudedir.Root {
	t.Helper()
	root := claudedir.Root(filepath.Join(t.TempDir(), ".claude"))
	if err := os.MkdirAll(root.String(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(root.SettingsPath(), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestReadSettings(t *testing.T) {
	root := writeSettings(t, `{"model":"claude-fable-5[1m]","effortLevel":"high","outputStyle":"butterfield","env":{"SECRET":"x"}}`)
	s := readSettings(root)
	if s.Model != "claude-fable-5[1m]" || s.Effort != "high" || s.Style != "butterfield" {
		t.Errorf("got %+v, want model claude-fable-5[1m] / effort high / style butterfield", s)
	}
}

func TestReadSettingsDegradesToZero(t *testing.T) {
	if s := readSettings(claudedir.Root(filepath.Join(t.TempDir(), ".claude"))); s != (agent.State{}) {
		t.Errorf("missing file: got %+v, want zero", s)
	}
	if s := readSettings(writeSettings(t, "{not json")); s != (agent.State{}) {
		t.Errorf("malformed file: got %+v, want zero", s)
	}
	if s := readSettings(writeSettings(t, `{"statusLine":"x"}`)); s != (agent.State{}) {
		t.Errorf("fields absent: got %+v, want zero", s)
	}
}

func TestZeroRootProviderIsStateless(t *testing.T) {
	if s := (Claude{}).State(0, "/anywhere"); s != (agent.State{}) {
		t.Errorf("zero root: got %+v, want zero State (never CWD-relative reads)", s)
	}
}
