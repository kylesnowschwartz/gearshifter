package catalog

import (
	_ "embed"
	"strings"
)

// builtins.tsv is generated from the official commands reference by
// tools/genbuiltins (SPEC §5.2, decision D1). Regenerate on release:
//
//	go run ./tools/genbuiltins -in commands.md -out internal/catalog/builtins.tsv
//
// Columns: name, min_version, argument_hint, description.
//
//go:embed builtins.tsv
var builtinsTSV string

// Builtins returns the vendored built-in commands available at the given
// Claude Code version. An empty version disables gating and returns all rows.
func Builtins(claudeVersion string) []Command {
	var cmds []Command
	for _, line := range strings.Split(builtinsTSV, "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		f := strings.SplitN(line, "\t", 4)
		if len(f) != 4 {
			continue
		}
		c := Command{
			Name:         f[0],
			MinVersion:   f[1],
			ArgumentHint: f[2],
			Description:  f[3],
			Source:       SourceBuiltin.String(),
		}
		if claudeVersion != "" && c.MinVersion != "" && compareVersions(claudeVersion, c.MinVersion) < 0 {
			continue
		}
		cmds = append(cmds, c)
	}
	return cmds
}
