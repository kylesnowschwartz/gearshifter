# Gearshifter

A tmux plugin: a `display-popup` **control deck** for Claude Code — a grid of
big clickable tiles (buttons, enum "gears", launchers) that fire or insert
slash commands into the Claude pane. Grew out of a command-palette concept;
the deck reframe is the current direction.

Sources of truth (all in `.agent-history/`, git-ignored, disk-only):

- `SPEC.md` — product spec (revised 2026-07-04)
- `ARCHITECTURE.md` — module boundaries, widget archetypes, message design
- `UI-RESEARCH.md` — sprite/palette/font research; read "Session state at
  wrap-up" before proposing UI work
- `prototypes/` — working sprite-pipeline scripts (Python, port 1:1 to Go)

## Locked decisions — do not re-litigate

- **Stack:** Go + Bubble Tea v2 + Lip Gloss v2 (not shell+fzf).
- **Injection recipe (M0 spike PASSED):** `tmux load-buffer` → `paste-buffer -p`
  → `send-keys Enter`; never inject partial command names.
- **Enter policy:** always-enter; Tab = insert-without-Enter override.
- **Pane targeting:** origin pane, captured at keybinding time.
- **Built-ins catalog:** vendored version-gated table filtered by
  `claude --version`; docs-scrape is experimental opt-in.
- **Distribution:** TPM + tmux-fastcopy bootstrap (build if `go` present,
  else curl goreleaser release asset); no binaries in git.
- **Aesthetic:** 8-bit/Game Boy vibe. Truecolor half-block ANSI sprites
  (`▀` fg=top px, bg=bottom px), assets shipped via `go:embed`. Sixel, Kitty
  graphics, octants, and Nerd Font icons ruled out (see UI-RESEARCH.md).
- **Sprites are mascot/flourish only, NEVER button icons** (decided
  2026-07-04 from live prototype renders): pixel art only reads at authored
  resolution — every downsample rags it — and native 24×24 cards fit only 3
  buttons per popup. Deck tiles are text + box-drawing chrome (mock D /
  mockups/01); charm budget = palette, borders, clawd mascot, animation.
  Prior-art synthesis in .agent-history/mockups/REFERENCES.md.
- **Icon source:** pixelarticons (MIT; `opensrc path pixelarticons`) via our
  dependency-free even-odd rasterizer; custom sprites are hand-authored layered
  text grids (chars = named palette roles). Clawd mascot source:
  `~/Code/dotfiles/clawd-icon/src/grid.json` (Anthropic trademark — acknowledge
  if published).
- **Rendering physics (fixed):** sprites are multi-cell blocks and can never
  sit inline with text; font glyphs are 1-cell but render monochrome. Any UI
  design must respect both.
- **Gear tile UX:** gated column — all enum values visible, current
  highlighted; click any value directly or j/k + Enter (not a stepper, not an
  H-gate).
- **Module dependency rules (ARCHITECTURE.md §2):** `widget` never imports
  `tmux` (tiles emit intent Msgs; `app` injects); `catalog`/`tmux` never import
  Bubble Tea (plain Go, powers the scriptable `list`/`inject` subcommands).

## State / next steps

No code yet — repo holds only this file plus `.agent-history/` docs.

Done (2026-07-04): M0 injection spike (passed, recipe above); product spec v1;
interview settled D1–D4; architecture draft (module tree, widget taxonomy,
message design, layout.toml sketch); prior-art survey (SPEC §1.1 — no direct
competitor; craftzdog/tmux-claude-session-manager and sainnhe/tmux-fzf are the
reference implementations; "gearshifter" name unclaimed).

Build order (ARCHITECTURE.md §8, supersedes SPEC §13 numbering):

1. **S1 spike: PASSED (2026-07-04).** `display-popup` forwards mouse to the
   inner Bubble Tea app on tmux 3.6a, with coordinates already translated to
   popup-local border-adjusted space (inner top-left = x0,y0). Method + details
   in ARCHITECTURE.md §4. Canvas-vs-bubblezone deferred to M3 (non-blocking).
2. **M1 plumbing: DONE (2026-07-04, branch `feat/m1-plumbing`).** `catalog`
   (user/project/builtin sources, symlink-safe scanners, vendored 101-command
   builtins.tsv regenerated via `tools/genbuiltins`) + `tmux` (M0 recipe behind
   a Runner interface, sequence locked by test) + `list`/`inject`/`version`
   subcommands. Stdlib-only, all tests green, e2e-verified against live tmux.
   Gotchas learned: skill dirs are often symlinks (open SKILL.md, don't trust
   DirEntry.IsDir); `tmux display-message` exits 0 for unknown targets (use
   `list-panes` for existence checks).
   Post-review additions (revdiff, 591eb0d): plugin source (opt-in via
   `--sources plugin`; enabled∩installed with stale-record checks, names
   `plugin:command`); frontmatter parsed with goccy/go-yaml (folded `>-`
   descriptions were being corrupted); tests sandbox HOME via TestMain;
   genbuiltins has named parseRow + uniqueness check + tests.
3. **M2 palette: DONE (2026-07-04, merged to main).** `pick` subcommand:
   fuzzy-filtered list (prefix>substring>subsequence on name; literal
   description fallback), vim keys + mouse click/wheel, **hint-aware Enter**
   (required-arg `<...>` hint → insert `/cmd` with NoEnter;
   `catalog.Command.RequiresArgument`), Tab = insert-only, Esc side-effect
   free, fail-with-words errors (tmux status line after popup close).
   Recipes: `popup`, `bind-dev` (prefix+C-g), `qa-rig` (automated popup rig,
   scripts/qa/popup-rig.sh). Stack verified: **charm.land/bubbletea/v2**
   (Init()→Cmd, View()→tea.View struct, per-view MouseMode) + lipgloss v2;
   hit-testing for M3 = lipgloss Layer/Compositor.Hit (bubblezone rejected).
   **P2 preview pane built then DESCOPED** — deck thesis is find-and-fire,
   not prose; re-justify every planned feature against it.
   Gotchas: `display-popup` does NOT format-expand its command (resolve
   pane/cwd first; `run-shell` in bindings does expand); NEVER launch the
   popup from a Claude `!` command — injection races the `!`-completion
   input flush and is silently discarded (V6, M2-PALETTE.md); nested
   lipgloss styles reset ANSI mid-row (style highlighted rows once,
   unstyled parts).
4. **M3 deck (next):** grid, Button/Gear/Launcher tiles, layout.toml,
   Layer/Compositor hit-testing; palette embeds behind the Launcher tile
   (`internal/palette.Model` is self-contained). Gear-state spike (V7) here.
   `pick --layout` flag already exists (inbuilt `telescope` = the M2
   fullscreen palette, current default): deck registers there, becomes the
   default, telescope stays a user toggle — never delete it. Custom
   layout.toml paths resolve through the same flag.
5. **M4 polish** (TPM bootstrap, goreleaser, CI, bump/release recipes) →
   **M5 aesthetic** (themes, sprites, animation; glamour "dark" hardcoding
   revisit if preview ever returns).

Open items:

- **Font A/B test:** build two mock variants and compare — (A) custom bundled
  TTF (clawd-icon `build-font.py` pattern; 1-cell pictograms, install +
  detection + Unicode fallback) vs (B) fontless richer UI (Unicode+color
  inline, sprites in block zones). Decision driver: information density per
  screen and whether a 'condensed' layout exists. Glyphs win density;
  sprites win portability/color.
- House palette: DMG green vs PICO-8 (clawd's orange favors PICO-8).
- V7 gear-state display (how tiles learn the session's current model/effort):
  capture-pane sniff vs state file vs stateless — ARCHITECTURE.md §5, target M3.
- Smaller spec items (SPEC §12): V1 plugin active-version resolution, V5
  shadowing precedence, V6 input queuing while Claude is generating.
