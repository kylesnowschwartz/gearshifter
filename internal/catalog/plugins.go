package catalog

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// Plugin discovery intersects two records (verified against Claude Code's
// on-disk layout, 2026-07):
//
//   - ~/.claude/settings.json          "enabledPlugins": {"<plugin>@<mp>": true}
//   - ~/.claude/plugins/installed_plugins.json  (schema v2)
//     "plugins": {"<plugin>@<mp>": [{scope, installPath, projectPath?, ...}]}
//
// installed_plugins.json holds stale entries for plugins whose cache dirs
// are gone, so installPath is verified on disk before scanning.

type installedPluginEntry struct {
	Scope       string `json:"scope"`
	ProjectPath string `json:"projectPath"`
	InstallPath string `json:"installPath"`
}

// scanPlugins discovers commands and skills shipped by installed & enabled
// plugins, namespaced <plugin>:<name> as Claude Code surfaces them.
func scanPlugins(home, projectDir string) []Command {
	enabled := readEnabledPlugins(filepath.Join(home, ".claude", "settings.json"))
	if len(enabled) == 0 {
		return nil
	}
	installed := readInstalledPlugins(filepath.Join(home, ".claude", "plugins", "installed_plugins.json"))

	var cmds []Command
	for key, entries := range installed {
		if !enabled[key] {
			continue
		}
		pluginName, _, ok := strings.Cut(key, "@")
		if !ok || pluginName == "" {
			continue
		}
		for _, e := range entries {
			if e.Scope == "project" && e.ProjectPath != projectDir {
				continue
			}
			info, err := os.Stat(e.InstallPath)
			if err != nil || !info.IsDir() {
				continue // stale install record
			}
			found := scanCommands(filepath.Join(e.InstallPath, "commands"), SourcePlugin)
			found = append(found, scanSkillsDeep(filepath.Join(e.InstallPath, "skills"), SourcePlugin)...)
			for _, c := range found {
				c.Name = pluginName + ":" + c.Name
				cmds = append(cmds, c)
			}
		}
	}
	return cmds
}

func readEnabledPlugins(settingsPath string) map[string]bool {
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		return nil
	}
	var s struct {
		EnabledPlugins map[string]bool `json:"enabledPlugins"`
	}
	if err := json.Unmarshal(data, &s); err != nil {
		return nil
	}
	return s.EnabledPlugins
}

func readInstalledPlugins(path string) map[string][]installedPluginEntry {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var f struct {
		Plugins map[string][]installedPluginEntry `json:"plugins"`
	}
	if err := json.Unmarshal(data, &f); err != nil {
		return nil
	}
	return f.Plugins
}
