package catalog

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func userSkillsDir(home string) string   { return filepath.Join(home, ".claude", "skills") }
func userCommandsDir(home string) string { return filepath.Join(home, ".claude", "commands") }
func projectSkillsDir(dir string) string { return filepath.Join(dir, ".claude", "skills") }
func projectCommandsDir(dir string) string {
	return filepath.Join(dir, ".claude", "commands")
}

// scanSkills reads <dir>/<name>/SKILL.md entries. The directory name is the
// command name; frontmatter provides description and argument-hint.
func scanSkills(dir string, src Source) []Command {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var cmds []Command
	for _, e := range entries {
		// Opening SKILL.md directly (rather than checking e.IsDir) keeps
		// symlinked skill directories working — a common dotfiles setup.
		path := filepath.Join(dir, e.Name(), "SKILL.md")
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		fm := parseFrontmatter(f)
		f.Close()
		cmds = append(cmds, Command{
			Name:         e.Name(),
			ArgumentHint: fm["argument-hint"],
			Description:  fm["description"],
			Source:       src.String(),
			Path:         path,
		})
	}
	return cmds
}

// scanCommands walks <dir>/**/*.md. Subdirectories namespace the command
// name with ':' (foo/bar.md -> foo:bar), matching Claude Code's convention.
func scanCommands(dir string, src Source) []Command {
	var cmds []Command
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(d.Name(), ".md") {
			return nil //nolint:nilerr // unreadable entries are skipped, not fatal
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return nil
		}
		name := strings.TrimSuffix(rel, ".md")
		name = strings.ReplaceAll(name, string(filepath.Separator), ":")

		f, err := os.Open(path)
		if err != nil {
			return nil
		}
		fm := parseFrontmatter(f)
		f.Close()

		cmds = append(cmds, Command{
			Name:         name,
			ArgumentHint: fm["argument-hint"],
			Description:  fm["description"],
			Source:       src.String(),
			Path:         path,
		})
		return nil
	})
	return cmds
}
