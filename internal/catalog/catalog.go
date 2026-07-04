// Package catalog discovers the slash commands available to a Claude Code
// session: built-ins (vendored, version-gated), user skills/commands, and
// project skills/commands. It is plain Go with no TUI dependencies so the
// list/inject subcommands stay scriptable.
package catalog

import (
	"sort"
)

// Source identifies where a command definition came from. Precedence for
// name shadowing is the declaration order below (lower value wins), mirroring
// SPEC §5.3: user > project > plugin > builtin.
type Source int

const (
	SourceUser Source = iota
	SourceProject
	SourcePlugin // reserved for M2
	SourceBuiltin
)

func (s Source) String() string {
	switch s {
	case SourceUser:
		return "user"
	case SourceProject:
		return "project"
	case SourcePlugin:
		return "plugin"
	case SourceBuiltin:
		return "builtin"
	}
	return "unknown"
}

// Command is one available slash command. Name has no leading slash.
type Command struct {
	Name         string
	ArgumentHint string
	Description  string
	Source       string
	Path         string // definition file; empty for builtins
	MinVersion   string // builtins only
}

// Options configures Build.
type Options struct {
	Home          string // user home dir; sources 2-3 live under Home/.claude
	ProjectDir    string // target pane's cwd; project source skipped if empty
	ClaudeVersion string // filters builtins; empty includes all with a caveat
	Sources       map[string]bool
}

// WantSource reports whether a source is enabled. A nil/empty set means all.
func (o Options) WantSource(name string) bool {
	if len(o.Sources) == 0 {
		return true
	}
	return o.Sources[name]
}

// Build assembles the deduplicated, sorted command catalog.
func Build(opts Options) ([]Command, error) {
	type ranked struct {
		cmd  Command
		rank Source
	}
	best := map[string]ranked{}

	add := func(cmds []Command, rank Source) {
		for _, c := range cmds {
			if prev, ok := best[c.Name]; ok && prev.rank <= rank {
				continue
			}
			best[c.Name] = ranked{cmd: c, rank: rank}
		}
	}

	if opts.WantSource("user") && opts.Home != "" {
		add(scanSkills(userSkillsDir(opts.Home), SourceUser), SourceUser)
		add(scanCommands(userCommandsDir(opts.Home), SourceUser), SourceUser)
	}
	if opts.WantSource("project") && opts.ProjectDir != "" {
		add(scanSkills(projectSkillsDir(opts.ProjectDir), SourceProject), SourceProject)
		add(scanCommands(projectCommandsDir(opts.ProjectDir), SourceProject), SourceProject)
	}
	if opts.WantSource("builtin") {
		add(Builtins(opts.ClaudeVersion), SourceBuiltin)
	}

	out := make([]Command, 0, len(best))
	for _, r := range best {
		out = append(out, r.cmd)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}
