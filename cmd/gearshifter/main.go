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
	"github.com/kylesnowschwartz/gearshifter/internal/theme"
	"github.com/kylesnowschwartz/gearshifter/internal/tmux"
)

var version = "dev" // set via -ldflags at release time

// Inbuilt layout names for the pick UI. deck is the M3 tile grid, the
// default since M3 close; telescope is the original fullscreen searchable
// palette (M2), kept forever as a user toggle. Custom layout.toml paths
// resolve through the same flag.
const (
	layoutTelescope = "telescope"
	layoutDeck      = "deck"
	defaultLayout   = layoutDeck
)

const usage = `gearshifter — a tmux control deck for Claude Code slash commands

Usage:
  gearshifter pick --pane PANE [--cwd DIR] [--sources ...] [--layout NAME] [--theme NAME]
  gearshifter strip [--pane PANE] [--cwd DIR] [--sources ...] [--layout NAME] [--theme NAME] [--compact]
  gearshifter list [--cwd DIR] [--sources user,project,builtin,plugin]
  gearshifter inject --pane PANE [--no-enter] [--no-clear] TEXT
  gearshifter version

Subcommands:
  pick     Open the interactive UI (run it inside tmux display-popup);
           selecting a command injects it into the target pane and
           presses Enter. --layout picks the UI: deck (the tile grid,
           the default), telescope (fullscreen searchable palette), or a
           path to a layout.toml (see examples/layout.toml). --theme
           picks the color theme: default, or plain (no color).
  strip    Run the deck as a persistent pane widget: the UI stays open
           across fires, injecting each fired tile into the window's
           Claude pane — auto-detected at fire time, or pinned with
           --pane — and re-polling gear state every few seconds.
           Same --layout choices as pick, minus telescope. --compact
           renders 1-row glyph chips for narrow sidebar panes.
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
	case "strip":
		err = runStrip(os.Args[2:])
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
	layoutName := fs.String("layout", defaultLayout, "UI layout to open: telescope, deck, or a path to a layout.toml")
	themeName := fs.String("theme", "default", "color theme: default, or plain (no color)")
	sortName := fs.String("sort", "", "sort order for the built-in deck's buttons: alpha (default: data-ranked six-pack + generic fillers); only valid with --layout deck")
	mascot := fs.Bool("mascot", true, "render the clawd mascot when the canvas has spare rows (colored themes only)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	inbuilt, layoutPath, err := resolveLayout(*layoutName)
	if err != nil {
		return fmt.Errorf("pick: %w", err)
	}
	sortMode, err := parseSort(*sortName)
	if err != nil {
		return fmt.Errorf("pick: %w", err)
	}
	if sortMode != layout.SortNone && inbuilt != layoutDeck {
		return fmt.Errorf("pick: --sort only applies to the built-in deck layout, not %q", *layoutName)
	}
	styles, err := theme.Load(*themeName)
	if err != nil {
		return fmt.Errorf("pick: %w", err)
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

	pick, ok, err := runPickUI(inbuilt, layoutPath, cmds, styles, client, *pane, sortMode, *mascot)
	if err != nil {
		return err
	}
	if !ok {
		return nil // cancelled: zero side effects
	}
	return injectSelection(client, *pane, pick)
}

// runStrip runs the deck as a persistent pane widget (STRIP-EMBED.md
// step 1). Unlike pick, there is no quit-then-inject handoff: the app
// stays alive and delivers each selection mid-loop through an injector
// composed here, so widget/app still never import tmux. The target is
// re-resolved at every fire — panes come and go under a long-lived strip.
func runStrip(args []string) error {
	fs := flag.NewFlagSet("strip", flag.ExitOnError)
	pane := fs.String("pane", "", "pin the target pane id (e.g. %12); default: scan this window for a Claude pane at fire time")
	cwd := fs.String("cwd", "", "directory for project-scoped commands; defaults to the target pane's cwd")
	sources := fs.String("sources", "", "comma-separated source filter: user,project,builtin,plugin (default: user,project,builtin)")
	layoutName := fs.String("layout", defaultLayout, "UI layout: deck or a path to a layout.toml (telescope quits on selection — not strip-compatible)")
	themeName := fs.String("theme", "default", "color theme: default, or plain (no color)")
	compact := fs.Bool("compact", false, "render the chip flow (1-row glyph chips) — sized for a ~33-col sidebar pane")
	sortName := fs.String("sort", "", "sort order for the built-in deck's buttons: alpha (default: data-ranked six-pack + generic fillers); only valid with --layout deck")
	mascot := fs.Bool("mascot", true, "render the clawd mascot when the pane has spare rows (colored themes only, never compact)")
	mascotGlyph := fs.Bool("mascot-glyph", false, "render the 1-cell clawd glyph in the compact footer (requires the Clawd.ttf font); only valid with --compact")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *mascotGlyph && !*compact {
		return fmt.Errorf("strip: --mascot-glyph is the compact footer's mascot; the full strip renders the sprite (drop the flag or add --compact)")
	}
	inbuilt, layoutPath, err := resolveLayout(*layoutName)
	if err != nil {
		return fmt.Errorf("strip: %w", err)
	}
	if inbuilt == layoutTelescope {
		return fmt.Errorf("strip: telescope quits on selection; strip needs a deck-shaped layout")
	}
	sortMode, err := parseSort(*sortName)
	if err != nil {
		return fmt.Errorf("strip: %w", err)
	}
	if sortMode != layout.SortNone && inbuilt != layoutDeck {
		return fmt.Errorf("strip: --sort only applies to the built-in deck layout, not %q", *layoutName)
	}
	styles, err := theme.Load(*themeName)
	if err != nil {
		return fmt.Errorf("strip: %w", err)
	}
	if os.Getenv("TMUX") == "" {
		return fmt.Errorf("strip: must run inside tmux (split a pane and run it there)")
	}

	client := tmux.NewClient(nil)
	var provider agent.Provider = claude.New()
	resolve := stripTarget(client, provider, *pane, os.Getenv("TMUX_PANE"))

	// Startup snapshot: the initial gear state, and the target's cwd for
	// project-scoped commands. A missing target is fine here — the strip
	// opens stateless and the refresh tick picks the state up once a
	// Claude pane exists.
	var state agent.State
	catalogCwd := *cwd
	if target, err := resolve(); err == nil {
		state = provider.State(target.PID, target.Cwd)
		if catalogCwd == "" {
			catalogCwd = target.Cwd
		}
	}
	cmds, err := buildCatalog(catalogCwd, *sources)
	if err != nil {
		return err
	}
	var placements []layout.Placement
	if layoutPath != "" {
		placements, err = layout.Load(layoutPath, cmds, state, styles)
		if err != nil {
			return fmt.Errorf("strip: %w", err)
		}
	} else {
		placements = layout.Default(cmds, state, styles, sortMode)
	}
	if *compact {
		placements = layout.Compacted(placements, state, styles)
	}

	inject := func(f app.Fire) error {
		target, err := resolve()
		if err != nil {
			return err
		}
		text, opts := buildInjection(selection{cmd: f.Cmd, arg: f.Arg, insertOnly: f.InsertOnly})
		if err := client.Inject(target.ID, text, opts); err != nil {
			return err
		}
		if f.FromMouse {
			// A click-fire stole tmux focus from the Claude pane — hand
			// it back (companion QA 2026-07-06). Keyboard fires keep the
			// user where they deliberately are. Best-effort — the
			// injection already landed.
			_ = client.SelectPane(target.ID)
		}
		return nil
	}
	refresh := func() map[string]string {
		target, err := resolve()
		if err != nil {
			// No target: gears honestly stateless (markers clear) instead
			// of advertising a dead session's state forever.
			return layout.GearSettings(agent.State{})
		}
		return layout.GearSettings(provider.State(target.PID, target.Cwd))
	}
	hooks := app.PersistentHooks{
		Inject:  inject,
		Refresh: refresh,
		// Seed with the startup snapshot the placements were built from,
		// so the first poll that matches it can't snap gear cursors.
		Seed: layout.GearSettings(state),
	}
	model := app.New(cmds, placements, styles).Persistent(hooks)
	if *compact {
		model = model.Compact()
	}
	if !*mascot {
		model = model.WithoutMascot()
	}
	if *mascotGlyph {
		model = model.WithMascotGlyph()
	}
	if _, err := tea.NewProgram(model).Run(); err != nil {
		return fmt.Errorf("strip: %w", err)
	}
	return nil
}

// stripTarget returns strip mode's per-fire target resolver. An explicit
// pane pin wins — existence-checked at fire time, not startup, because
// panes die under a long-lived strip. Otherwise the window containing
// self is scanned in index order for the first other pane running a
// resolvable Claude session.
func stripTarget(client *tmux.Client, provider agent.Provider, explicit, self string) func() (tmux.Pane, error) {
	return func() (tmux.Pane, error) {
		if explicit != "" {
			if explicit == self {
				// Injecting into the strip's own pane would loop: the
				// trailing Enter re-fires the focused tile, forever.
				return tmux.Pane{}, fmt.Errorf("target pane %s is the strip itself", explicit)
			}
			if !client.PaneExists(explicit) {
				return tmux.Pane{}, fmt.Errorf("target pane %s is gone", explicit)
			}
			pid, err := client.PanePID(explicit)
			if err != nil {
				return tmux.Pane{}, err
			}
			cwd, err := client.PaneCwd(explicit)
			if err != nil {
				return tmux.Pane{}, err
			}
			return tmux.Pane{ID: explicit, PID: pid, Cwd: cwd}, nil
		}
		panes, err := client.WindowPanes(self)
		if err != nil {
			return tmux.Pane{}, err
		}
		for _, p := range panes {
			if p.ID == self {
				continue
			}
			if provider.HasSession(p.PID, p.Cwd) {
				return p, nil
			}
		}
		return tmux.Pane{}, fmt.Errorf("no Claude pane in this window")
	}
}

// resolveLayout classifies --layout as an inbuilt name (telescope, deck)
// or a path to a layout.toml. Unknown values fail here, before any UI
// opens.
func resolveLayout(name string) (inbuilt, path string, err error) {
	switch name {
	case layoutTelescope, layoutDeck:
		return name, "", nil
	}
	if _, statErr := os.Stat(name); statErr != nil {
		// No subcommand prefix: pick and strip both resolve through here
		// and wrap with their own name.
		return "", "", fmt.Errorf("unknown layout %q — not an inbuilt (%s, %s) and not a readable layout.toml path",
			name, layoutTelescope, layoutDeck)
	}
	return "", name, nil
}

// parseSort validates --sort. A layout.toml already authors its own tile
// order, so sort only ever applies to the built-in deck — callers check
// that separately once resolveLayout has classified --layout.
func parseSort(raw string) (layout.Sort, error) {
	switch layout.Sort(raw) {
	case layout.SortNone, layout.SortAlpha:
		return layout.Sort(raw), nil
	default:
		return "", fmt.Errorf("unknown sort %q — valid: alpha", raw)
	}
}

// runPickUI runs the chosen layout's Bubble Tea program and reports the
// user's selection; ok is false when they cancelled.
func runPickUI(inbuilt, layoutPath string, cmds []catalog.Command, styles *theme.Styles, client *tmux.Client, pane string, sortMode layout.Sort, mascot bool) (selection, bool, error) {
	switch inbuilt {
	case layoutTelescope:
		final, err := tea.NewProgram(palette.New(cmds, styles)).Run()
		if err != nil {
			return selection{}, false, fmt.Errorf("pick: %w", err)
		}
		model := final.(palette.Model)
		sel, ok := model.Selection()
		return selection{cmd: sel, insertOnly: model.InsertOnly()}, ok, nil
	default: // deck, inbuilt or from a layout.toml — both consume placements
		// Session-specific model when the pane's Claude session is
		// resolvable; global settings otherwise (V7/P3.5, M3-DECK.md).
		var provider agent.Provider = claude.New()
		panePID, _ := client.PanePID(pane)
		paneCwd, _ := client.PaneCwd(pane)
		state := provider.State(panePID, paneCwd)
		placements := layout.Default(cmds, state, styles, sortMode)
		if layoutPath != "" {
			var err error
			placements, err = layout.Load(layoutPath, cmds, state, styles)
			if err != nil {
				// Popup stderr is invisible; a broken layout must fail
				// with words in the tmux status line (M2 lesson).
				_ = client.DisplayMessage("gearshifter: " + err.Error())
				return selection{}, false, fmt.Errorf("pick: %w", err)
			}
		}
		deckModel := app.New(cmds, placements, styles)
		if !mascot {
			deckModel = deckModel.WithoutMascot()
		}
		final, err := tea.NewProgram(deckModel).Run()
		if err != nil {
			return selection{}, false, fmt.Errorf("pick: %w", err)
		}
		model := final.(app.Model)
		sel, ok := model.Selection()
		return selection{cmd: sel, arg: model.Arg(), insertOnly: model.InsertOnly()}, ok, nil
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
