package catalog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestMain sandboxes HOME so no test — present or future — can reach the
// user's real ~/.claude directory, even through an accidental env-based
// lookup. Fixtures themselves use t.TempDir(), which tears down
// automatically. (M1 review directive.)
func TestMain(m *testing.M) {
	sandbox, err := os.MkdirTemp("", "gearshifter-test-home-")
	if err != nil {
		panic(err)
	}
	os.Setenv("HOME", sandbox)
	code := m.Run()
	os.RemoveAll(sandbox)
	os.Exit(code)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func fixtureHome(t *testing.T) string {
	home := t.TempDir()
	writeFile(t, filepath.Join(home, ".claude", "skills", "review", "SKILL.md"),
		"---\nname: review\ndescription: Review code for issues\n---\nbody\n")
	writeFile(t, filepath.Join(home, ".claude", "commands", "deploy.md"),
		"---\ndescription: Deploy the app\nargument-hint: <env>\n---\nbody\n")
	writeFile(t, filepath.Join(home, ".claude", "commands", "gws", "sync.md"),
		"---\ndescription: Sync worktrees\n---\nbody\n")
	return home
}

func TestScanSkillsFollowsSymlinks(t *testing.T) {
	home := t.TempDir()
	real := filepath.Join(home, "dotfiles", "linked-skill")
	writeFile(t, filepath.Join(real, "SKILL.md"), "---\ndescription: Symlinked skill\n---\n")
	skills := filepath.Join(home, ".claude", "skills")
	if err := os.MkdirAll(skills, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, filepath.Join(skills, "linked-skill")); err != nil {
		t.Fatal(err)
	}

	m := buildMap(t, Options{Home: home, Sources: map[string]bool{"user": true}})
	if _, ok := m["linked-skill"]; !ok {
		t.Error("symlinked skill directory not discovered")
	}
}

func fixtureProject(t *testing.T) string {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".claude", "skills", "review", "SKILL.md"),
		"---\ndescription: Project-local review override\n---\nbody\n")
	writeFile(t, filepath.Join(dir, ".claude", "commands", "release.md"),
		"---\ndescription: Cut a release\n---\nbody\n")
	return dir
}

func buildMap(t *testing.T, opts Options) map[string]Command {
	t.Helper()
	cmds, err := Build(opts)
	if err != nil {
		t.Fatal(err)
	}
	m := map[string]Command{}
	for _, c := range cmds {
		m[c.Name] = c
	}
	return m
}

func TestBuildCollectsAllSources(t *testing.T) {
	m := buildMap(t, Options{Home: fixtureHome(t), ProjectDir: fixtureProject(t)})

	for name, wantSource := range map[string]string{
		"review":   "user", // user shadows project (SPEC §5.3)
		"deploy":   "user",
		"gws:sync": "user",
		"release":  "project",
		"context":  "builtin",
	} {
		got, ok := m[name]
		if !ok {
			t.Fatalf("command %q missing from catalog", name)
		}
		if got.Source != wantSource {
			t.Errorf("command %q: source = %q, want %q", name, got.Source, wantSource)
		}
	}
	if m["deploy"].ArgumentHint != "<env>" {
		t.Errorf("deploy argument hint = %q, want <env>", m["deploy"].ArgumentHint)
	}
}

func TestBuildUserShadowsProject(t *testing.T) {
	m := buildMap(t, Options{Home: fixtureHome(t), ProjectDir: fixtureProject(t)})
	if got := m["review"].Description; got != "Review code for issues" {
		t.Errorf("shadowing failed: review description = %q (project won over user)", got)
	}
}

func TestBuildSourceFilter(t *testing.T) {
	m := buildMap(t, Options{
		Home:       fixtureHome(t),
		ProjectDir: fixtureProject(t),
		Sources:    map[string]bool{"project": true},
	})
	if _, ok := m["deploy"]; ok {
		t.Error("user command present despite project-only filter")
	}
	if _, ok := m["context"]; ok {
		t.Error("builtin present despite project-only filter")
	}
	if got := m["review"].Source; got != "project" {
		t.Errorf("review source = %q, want project", got)
	}
}

func TestBuiltinsVersionGating(t *testing.T) {
	all := Builtins("")
	if len(all) < 50 {
		t.Fatalf("vendored builtins table suspiciously small: %d rows", len(all))
	}

	names := func(cmds []Command) map[string]bool {
		m := map[string]bool{}
		for _, c := range cmds {
			m[c.Name] = true
		}
		return m
	}

	old := names(Builtins("2.1.0"))
	if old["advisor"] {
		t.Error("advisor (min 2.1.98) should be gated out at 2.1.0")
	}
	if !old["context"] {
		t.Error("context (no min version) should always be present")
	}
	current := names(Builtins("2.1.200"))
	if !current["advisor"] {
		t.Error("advisor should be present at 2.1.200")
	}
}

func TestBuiltinsDescriptionsAreClean(t *testing.T) {
	for _, c := range Builtins("") {
		if strings.Contains(c.Description, "{/*") {
			t.Errorf("%s: unstripped annotation in description", c.Name)
		}
		if strings.Contains(c.Description, "](") {
			t.Errorf("%s: unstripped markdown link in description", c.Name)
		}
	}
}

func TestRequiresArgument(t *testing.T) {
	cases := []struct {
		hint string
		want bool
	}{
		{"<question>", true},
		{" <instruction>", true},
		{"[model|off]", false},
		{"[prompt]", false},
		{"", false},
	}
	for _, tc := range cases {
		got := Command{ArgumentHint: tc.hint}.RequiresArgument()
		if got != tc.want {
			t.Errorf("RequiresArgument(%q) = %v, want %v", tc.hint, got, tc.want)
		}
	}
}
