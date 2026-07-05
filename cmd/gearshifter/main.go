// Command gearshifter is a tmux control deck for Claude Code slash commands.
// M1 ships the plumbing subcommands; the TUI (pick) arrives in M2/M3.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/kylesnowschwartz/gearshifter/internal/agent"
	"github.com/kylesnowschwartz/gearshifter/internal/agent/claude"
	"github.com/kylesnowschwartz/gearshifter/internal/app"
	"github.com/kylesnowschwartz/gearshifter/internal/catalog"
	"github.com/kylesnowschwartz/gearshifter/internal/layout"
	"github.com/kylesnowschwartz/gearshifter/internal/palette"
	"github.com/kylesnowschwartz/gearshifter/internal/tmux"
)

var version = "dev" // set via -ldflags at release time

// Inbuilt layout names for the pick UI. telescope is the original
// fullscreen searchable palette (M2); deck is the M3 tile grid (becomes
// the default at M3 close). Custom layout.toml paths resolve here too;
// telescope stays available as a user toggle.
const (
	layoutTelescope = "telescope"
	layoutDeck      = "deck"
	defaultLayout   = layoutTelescope
)

const usage = `gearshifter — a tmux control deck for Claude Code slash commands

Usage:
  gearshifter pick --pane PANE [--cwd DIR] [--sources ...] [--layout NAME]
  gearshifter list [--cwd DIR] [--sources user,project,builtin,plugin]
  gearshifter inject --pane PANE [--no-enter] [--no-clear] TEXT
  gearshifter version

Subcommands:
  pick     Open the interactive UI (run it inside tmux display-popup);
           selecting a command injects it into the target pane and
           presses Enter. --layout picks the UI: telescope (fullscreen
           searchable palette, the default); the deck grid joins in M3.
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
	case "pick":
		err = runPick(os.Args[2:])
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
	cmds, err := buildCatalog(*cwd, *sources)
	if err != nil {
		return err
	}
	for _, c := range cmds {
		fmt.Printf("%s\t%s\t%s\t%s\n", c.Name, c.Source, c.ArgumentHint, c.Description)
	}
	return nil
}

// buildCatalog assembles catalog.Options from the shared --cwd/--sources
// flag values and builds the command list. cwd defaults to the current
// directory; empty sources means the catalog default (user,project,builtin).
func buildCatalog(cwd, sources string) ([]catalog.Command, error) {
	if cwd == "" {
		cwd, _ = os.Getwd()
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	opts := catalog.Options{
		Home:          home,
		ProjectDir:    cwd,
		ClaudeVersion: detectClaudeVersion(),
	}
	if sources != "" {
		opts.Sources = map[string]bool{}
		for _, s := range strings.Split(sources, ",") {
			opts.Sources[strings.TrimSpace(s)] = true
		}
	}
	return catalog.Build(opts)
}

// selection is what the pick UI hands back: the chosen command plus the
// modifiers that shape its injection (a committed gear value, Tab's
// insert-without-Enter request).
type selection struct {
	cmd        catalog.Command
	arg        string
	insertOnly bool
}

// HasArg reports whether a gear value came with the command — the
// injection becomes "/command value".
func (s selection) HasArg() bool { return s.arg != "" }

// runPick opens the pick UI and injects the chosen command into the
// target pane. Meant to run inside `tmux display-popup -E`. Three steps,
// one function each: validate flags/env here, runPickUI records what the
// user chose, injectSelection delivers it.
func runPick(args []string) error {
	fs := flag.NewFlagSet("pick", flag.ExitOnError)
	pane := fs.String("pane", "", "target tmux pane id (e.g. %12); required")
	cwd := fs.String("cwd", "", "directory for project-scoped commands; pass the target pane's cwd")
	sources := fs.String("sources", "", "comma-separated source filter: user,project,builtin,plugin (default: user,project,builtin)")
	layoutName := fs.String("layout", defaultLayout, "UI layout to open (inbuilt: telescope, deck)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *layoutName != layoutTelescope && *layoutName != layoutDeck {
		return fmt.Errorf("pick: unknown layout %q (inbuilt: %s, %s)", *layoutName, layoutTelescope, layoutDeck)
	}
	if os.Getenv("TMUX") == "" {
		return fmt.Errorf("pick: must run inside tmux (use `just popup` or a keybinding)")
	}
	if *pane == "" {
		return fmt.Errorf("pick: --pane is required")
	}

	client := tmux.NewClient(nil)
	if !client.PaneExists(*pane) {
		return fmt.Errorf("pick: pane %s not found", *pane)
	}
	cmds, err := buildCatalog(*cwd, *sources)
	if err != nil {
		return err
	}

	pick, ok, err := runPickUI(*layoutName, cmds, client, *pane)
	if err != nil {
		return err
	}
	if !ok {
		return nil // cancelled: zero side effects
	}
	return injectSelection(client, *pane, pick)
}

// runPickUI runs the chosen layout's Bubble Tea program and reports the
// user's selection; ok is false when they cancelled.
func runPickUI(layoutName string, cmds []catalog.Command, client *tmux.Client, pane string) (selection, bool, error) {
	switch layoutName {
	case layoutDeck:
		home, _ := os.UserHomeDir()
		// Session-specific model when the pane's Claude session is
		// resolvable; global settings otherwise (V7/P3.5, M3-DECK.md).
		var provider agent.Provider = claude.New(home)
		panePID, _ := client.PanePID(pane)
		paneCwd, _ := client.PaneCwd(pane)
		state := provider.State(panePID, paneCwd)
		final, err := tea.NewProgram(app.New(cmds, layout.Default(cmds, state))).Run()
		if err != nil {
			return selection{}, false, fmt.Errorf("pick: %w", err)
		}
		model := final.(app.Model)
		sel, ok := model.Selection()
		return selection{cmd: sel, arg: model.Arg(), insertOnly: model.InsertOnly()}, ok, nil
	default: // telescope
		final, err := tea.NewProgram(palette.New(cmds)).Run()
		if err != nil {
			return selection{}, false, fmt.Errorf("pick: %w", err)
		}
		model := final.(palette.Model)
		sel, ok := model.Selection()
		return selection{cmd: sel, insertOnly: model.InsertOnly()}, ok, nil
	}
}

// injectSelection delivers the selection to the target pane. The target
// can die while the UI is open, and popup stderr is invisible — failures
// surface in the tmux status line instead.
func injectSelection(client *tmux.Client, pane string, pick selection) error {
	text, opts := buildInjection(pick)
	if !client.PaneExists(pane) {
		_ = client.DisplayMessage(fmt.Sprintf("gearshifter: target pane %s is gone; %s not injected", pane, text))
		return fmt.Errorf("pick: target pane %s is gone; %s not injected", pane, strings.TrimSpace(text))
	}
	if err := client.Inject(pane, text, opts); err != nil {
		_ = client.DisplayMessage("gearshifter: inject failed: " + err.Error())
		return fmt.Errorf("pick: %w", err)
	}
	return nil
}

// buildInjection turns a selection into the exact injected text. Hint-aware
// Enter policy (D2): commands with a required argument (`<...>` hint) are
// inserted with a trailing space, ready for typing, instead of submitted
// bare (which misfires — e.g. /btw with no question); insertOnly (Tab)
// requests the same treatment explicitly. A gear value satisfies the
// argument itself — "/model opus", always-enter.
func buildInjection(pick selection) (string, tmux.InjectOptions) {
	text := "/" + pick.cmd.Name
	switch {
	case pick.HasArg():
		return text + " " + pick.arg, tmux.InjectOptions{}
	case pick.cmd.RequiresArgument() || pick.insertOnly:
		return text + " ", tmux.InjectOptions{NoEnter: true}
	}
	return text, tmux.InjectOptions{}
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
