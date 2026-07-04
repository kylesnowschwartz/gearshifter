package catalog

import (
	"fmt"
	"path/filepath"
	"testing"
)

// fixturePluginHome builds a home dir with two enabled plugins (one user
// scope, one project scope), one disabled plugin, and one stale install
// record whose cache dir is missing.
func fixturePluginHome(t *testing.T, projectDir string) string {
	t.Helper()
	home := t.TempDir()
	cache := filepath.Join(home, ".claude", "plugins", "cache")

	writeFile(t, filepath.Join(home, ".claude", "settings.json"), `{
		"enabledPlugins": {
			"alpha@mp": true,
			"beta@mp": true,
			"stale@mp": true,
			"disabled@mp": false
		}
	}`)

	alphaDir := filepath.Join(cache, "mp", "alpha", "1.2.0")
	betaDir := filepath.Join(cache, "mp", "beta", "0.4.0")
	disabledDir := filepath.Join(cache, "mp", "disabled", "1.0.0")

	writeFile(t, filepath.Join(home, ".claude", "plugins", "installed_plugins.json"), fmt.Sprintf(`{
		"version": 2,
		"plugins": {
			"alpha@mp":    [{"scope": "user", "installPath": %q, "version": "1.2.0"}],
			"beta@mp":     [{"scope": "project", "projectPath": %q, "installPath": %q, "version": "0.4.0"}],
			"stale@mp":    [{"scope": "user", "installPath": %q, "version": "9.9.9"}],
			"disabled@mp": [{"scope": "user", "installPath": %q, "version": "1.0.0"}]
		}
	}`, alphaDir, projectDir, betaDir, filepath.Join(cache, "mp", "stale", "9.9.9"), disabledDir))

	// alpha: one flat command, one nested skill
	writeFile(t, filepath.Join(alphaDir, "commands", "review.md"),
		"---\ndescription: Adversarial review\nargument-hint: '[--wait]'\n---\n")
	writeFile(t, filepath.Join(alphaDir, "skills", "engineering", "code-review", "SKILL.md"),
		"---\ndescription: Deep code review skill\n---\n")
	// beta (project-scoped): one skill
	writeFile(t, filepath.Join(betaDir, "skills", "critique", "SKILL.md"),
		"---\ndescription: Critique stage\n---\n")
	// disabled plugin has content that must never appear
	writeFile(t, filepath.Join(disabledDir, "commands", "ghost.md"),
		"---\ndescription: Should not appear\n---\n")
	return home
}

func TestScanPlugins(t *testing.T) {
	project := t.TempDir()
	home := fixturePluginHome(t, project)

	m := buildMap(t, Options{
		Home:       home,
		ProjectDir: project,
		Sources:    map[string]bool{"plugin": true},
	})

	alpha, ok := m["alpha:review"]
	if !ok {
		t.Fatal("alpha:review missing — flat plugin commands not scanned")
	}
	if alpha.Source != "plugin" || alpha.ArgumentHint != "[--wait]" {
		t.Errorf("alpha:review = %+v", alpha)
	}
	if _, ok := m["alpha:code-review"]; !ok {
		t.Error("alpha:code-review missing — nested plugin skills not walked")
	}
	if _, ok := m["beta:critique"]; !ok {
		t.Error("beta:critique missing — project-scoped plugin should match ProjectDir")
	}
	if _, ok := m["disabled:ghost"]; ok {
		t.Error("disabled plugin leaked into catalog")
	}
	for name := range m {
		if name == "stale:anything" {
			t.Error("stale install record leaked")
		}
	}
}

func TestScanPluginsProjectScopeMismatch(t *testing.T) {
	home := fixturePluginHome(t, "/some/other/project")
	m := buildMap(t, Options{
		Home:       home,
		ProjectDir: t.TempDir(), // different from the recorded projectPath
		Sources:    map[string]bool{"plugin": true},
	})
	if _, ok := m["beta:critique"]; ok {
		t.Error("project-scoped plugin appeared outside its project")
	}
	if _, ok := m["alpha:review"]; !ok {
		t.Error("user-scoped plugin should be unaffected by project mismatch")
	}
}

func TestPluginsExcludedByDefault(t *testing.T) {
	project := t.TempDir()
	home := fixturePluginHome(t, project)
	m := buildMap(t, Options{Home: home, ProjectDir: project}) // default sources
	if _, ok := m["alpha:review"]; ok {
		t.Error("plugin source should be opt-in, not part of the default set")
	}
}
