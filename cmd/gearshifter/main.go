// Command gearshifter is a tmux control deck for Claude Code slash commands.
// M1 ships the plumbing subcommands; the TUI (pick) arrives in M2/M3.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/kylesnowschwartz/gearshifter/internal/catalog"
	"github.com/kylesnowschwartz/gearshifter/internal/tmux"
)

var version = "dev" // set via -ldflags at release time

const usage = `gearshifter — a tmux control deck for Claude Code slash commands

Usage:
  gearshifter list [--cwd DIR] [--sources user,project,builtin,plugin]
  gearshifter inject --pane PANE [--no-enter] [--no-clear] TEXT
  gearshifter version

Subcommands:
  list     Print the available slash commands as TSV: name, source,
           argument hint, description. Default sources are
           user,project,builtin; add plugin explicitly to include
           installed-plugin commands (namespaced plugin:name).
  inject   Type TEXT into the target Claude Code pane and press Enter.
           Uses bracketed paste so the text lands literally.
  version  Print the gearshifter version.
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "list":
		err = runList(os.Args[2:])
	case "inject":
		err = runInject(os.Args[2:])
	case "version":
		fmt.Println(version)
	case "-h", "--help", "help":
		fmt.Print(usage)
	default:
		fmt.Fprintf(os.Stderr, "gearshifter: unknown subcommand %q\n\n%s", os.Args[1], usage)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "gearshifter:", err)
		os.Exit(1)
	}
}

func runList(args []string) error {
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	cwd := fs.String("cwd", "", "directory for project-scoped commands; defaults to the current directory (pass the target pane's cwd when invoking from a popup)")
	sources := fs.String("sources", "", "comma-separated source filter: user,project,builtin,plugin (default: user,project,builtin)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *cwd == "" {
		*cwd, _ = os.Getwd()
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	opts := catalog.Options{
		Home:          home,
		ProjectDir:    *cwd,
		ClaudeVersion: detectClaudeVersion(),
	}
	if *sources != "" {
		opts.Sources = map[string]bool{}
		for _, s := range strings.Split(*sources, ",") {
			opts.Sources[strings.TrimSpace(s)] = true
		}
	}

	cmds, err := catalog.Build(opts)
	if err != nil {
		return err
	}
	for _, c := range cmds {
		fmt.Printf("%s\t%s\t%s\t%s\n", c.Name, c.Source, c.ArgumentHint, c.Description)
	}
	return nil
}

func runInject(args []string) error {
	fs := flag.NewFlagSet("inject", flag.ExitOnError)
	pane := fs.String("pane", "", "target tmux pane id (e.g. %12); required")
	noEnter := fs.Bool("no-enter", false, "paste without pressing Enter")
	noClear := fs.Bool("no-clear", false, "skip clearing the prompt first")
	if err := fs.Parse(args); err != nil {
		return err
	}
	text := strings.Join(fs.Args(), " ")
	if *pane == "" {
		return fmt.Errorf("inject: --pane is required")
	}
	if text == "" {
		return fmt.Errorf("inject: TEXT argument is required")
	}

	client := tmux.NewClient(nil)
	if !client.PaneExists(*pane) {
		return fmt.Errorf("inject: pane %s not found", *pane)
	}
	return client.Inject(*pane, text, tmux.InjectOptions{
		NoClear: *noClear,
		NoEnter: *noEnter,
	})
}

// detectClaudeVersion asks the local claude binary for its version. Empty on
// failure, which disables builtin version-gating (all rows included).
func detectClaudeVersion() string {
	out, err := exec.Command("claude", "--version").Output()
	if err != nil {
		return ""
	}
	return catalog.ParseClaudeVersion(string(out))
}
