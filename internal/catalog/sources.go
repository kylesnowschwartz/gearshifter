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

// scanSkills reads <dir>/<name>/SKILL.md entries — the flat layout of user
// and project skills. It deliberately does NOT use WalkDir: user skill dirs
// are frequently symlinks into dotfiles repos, which WalkDir won't descend
// into; opening SKILL.md directly follows them. Plugins use the deep walker
// below instead.
func scanSkills(dir string, src Source) []Command {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var cmds []Command
	for _, e := range entries {
		path := filepath.Join(dir, e.Name(), "SKILL.md")
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		fm := parseFrontmatter(f)
		f.Close()
		cmds = append(cmds, Command{
			Name:         e.Name(),
			ArgumentHint: fm.ArgumentHint,
			Description:  fm.Description,
			Source:       src.String(),
			Path:         path,
		})
	}
	return cmds
}

// scanSkillsDeep walks <dir> for SKILL.md at any depth — plugins nest skills
// arbitrarily (e.g. skills/engineering/code-review/SKILL.md). The skill name
// is the directory containing SKILL.md. Safe for plugin cache dirs, which
// are real directories; do not use for user skills (symlinks, see above).
func scanSkillsDeep(dir string, src Source) []Command {
	var cmds []Command
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || d.Name() != "SKILL.md" {
			return nil //nolint:nilerr // unreadable entries are skipped, not fatal
		}
		f, err := os.Open(path)
		if err != nil {
			return nil
		}
		fm := parseFrontmatter(f)
		f.Close()
		cmds = append(cmds, Command{
			Name:         filepath.Base(filepath.Dir(path)),
			ArgumentHint: fm.ArgumentHint,
			Description:  fm.Description,
			Source:       src.String(),
			Path:         path,
		})
		return nil
	})
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
			ArgumentHint: fm.ArgumentHint,
			Description:  fm.Description,
			Source:       src.String(),
			Path:         path,
		})
		return nil
	})
	return cmds
}
