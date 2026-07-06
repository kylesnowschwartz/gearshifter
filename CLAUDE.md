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
  `tmux` (tiles emit intent Msgs; `app` records the selection, `cmd` injects);
  `catalog`/`tmux` never import Bubble Tea (plain Go, powers the scriptable
  `list`/`inject` subcommands); `layout` is the one bridge importing
  widget+deck+catalog+agent (P4's layout.toml parses into its Placements);
  session-state readers live in `agent/claude` behind `agent.Provider` —
  catalog is commands-only.
- **Theme seam (M5 P1):** color literals ONLY in `internal/theme` palette
  constructors; widgets/app/palette take `*theme.Styles` at construction
  (Crush roles→styles registry, TUI-AESTHETICS.md — never a global).
  Themes change color, never geometry (hit-testing; pinned by test).
  Themes: `default` (placeholder charcoal + clawd orange — a stand-in,
  NOT the house palette, which is deliberately deferred to M5 P5) and
  `plain` (colorless; behavior-freeze + reduced-decoration path), via
  `--theme` / `@gearshifter-theme`.

## Shared library (agent-ouija)

Claude session-state mechanics (registry/pane resolution, transcript tail
scan, path encoding, settings decode) come from
`github.com/kylesnowschwartz/agent-ouija` (repo:
`~/Code/my-projects/agent-ouija`), confined to `internal/agent/claude`.

**Staying current**: pin tagged versions only
(`go get github.com/kylesnowschwartz/agent-ouija@vX.Y.Z && go mod tidy`,
read its CHANGELOG first — breaking changes bump the library minor).
After any bump: `just check` and `just qa-rig`. For cross-repo dev use a
git-ignored `go.work`; any commit finalizing a bump must pass
`GOWORK=off go build ./... && go test ./...`.

**The line (one-way dependency)**: agent-ouija reads Claude Code state
and returns data; gearshifter decides what it means. The
settings-vs-transcript **mtime arbitration** and all gear/deck/tmux
policy stay here — never propose moving them into the library, and never
add gearshifter-specific types to the library. Format drift and parsing
defects are ALWAYS fixed in agent-ouija (with a fixture), never worked
around here. The `agent.Provider` seam stays: `agent/claude` is the only
package importing agent-ouija.

## State / next steps

M0–M4 shipped, deck content settled from real usage data
(DECK-CONTENT.md: default = MODEL/EFFORT gears + COMPACT/COPY/CLEAR/
CONTEXT/RESUME/CONFIG six-pack; the personalized example
`examples/custom.toml` (né personal.toml; the maintainer's daily layout) adds a GOAL
insert-tile, RADIO, RELOAD — its STYLE gear was REMOVED 2026-07-05:
Claude Code dropped /output-style and /config rejects custom styles
non-interactively; revival route = a /style command in the
sc-output-styles plugin, DECK-CONTENT.md postscript). **M5 COMPLETE
(P5 decided 2026-07-06): P1 theme seam, P2 tile states + nameplate
buttons, P4 wordmark gradient SHIPPED (M5-AESTHETIC.md); frame-dedup
refactor done (theme.FrameStyles + widget.tileFrame + one Styles.Armed
role + theme.ApplySurface). P5: the P1 charcoal+clawd-orange stand-in
was DECLARED the house palette as-is ("orangish, and plain, boom") —
no designed-ramp session, DMG/PICO-8 alternates dropped;
theme.go var renamed placeholder→house. P3 mascot revived under strip
step 4 (below).** **Strip (STRIP-EMBED.md) steps 1–3
SHIPPED (step 1 2026-07-05; steps 2–3 compact chip flow 2026-07-06
after a full pre-merge review round): `gearshifter strip [--compact]`
runs the deck as a persistent pane widget — app.PersistentHooks
(Inject func(app.Fire)/Refresh/Seed) deliver each fire off-loop, the
target Claude pane re-resolved PER FIRE (explicit `--pane` pin, never
self, else window scan via `agent.Provider.HasSession`); `--compact`
is the chip flow (column-aligned chips, gear chips own their row and
expand to the horizontal gated row, theme glyph table + layout.toml
`glyph =` override — authored > table > •, glyphs must be EA-Neutral
single-cell, no wordmark). Settled behavior: click-fires return tmux
focus to the target (Fire.FromMouse → SelectPane), keyboard fires
don't; esc NEVER quits a persistent strip (q/ctrl+c do); error
notices sticky, confirmations expire in 5s; hover ring hides when the
pointer leaves all tiles and hover-away collapses gear rows; pointer
events bounded by RENDERED rows; model badge arbitrates by transcript
ENTRY time, not file mtime (agent-ouija v1.1.0 LastAssistantModelAt,
pin bumped). Live as a tcm companion pane (tcm-side spec:
tail-claude-mux/.agent-history/COMPANION-PANE-SPEC.md; the maintainer runs
`--theme plain`). Steps 4–5 in flight (2026-07-06 interview): step 5
embed docs = README persistent-pane section (the maintainer's primary
usage; manual install only — TPM QA explicitly skipped, "too much
trouble for no users yet"); step 4 mascot = deck (P3 k9s recipe) +
full strip (2–3-row variant authored at size) + OPT-IN compact footer
glyph U+100CC0 (clawd-icon Clawd.ttf, ~/Code/dotfiles/clawd-icon —
1-cell, inherits text color so it survives plain; own config path,
EXEMPT from the chip-glyph EA-Neutral rule; renders tofu without the
font, hence opt-in). NOT in compact rows (chips own every row) and no
sprite under plain. Perf notes in STRIP-EMBED.md CLOSED ("perf seems
fine").** Distribution is parked as M6
until someone besides the maintainer wants an install. The deck is the default
UI; a `run-shell .../gearshifter.tmux` line in tmux.conf owns the
permanent `prefix+C-g` binding (`just bind-dev` sources the same
file for dev); `@gearshifter-layout/width/height` are honored at
keypress time; telescope stays one flag away. Dev loop: `just check`
after every substantive change; `just qa-rig` (popup rig, 25
assertions incl. deck click, plugin binding, strip persistent-pane,
and compact-strip end-to-end) before merging UI work.

Done foundations (2026-07-04): M0 injection spike (recipe above); product
spec v1; interview settled D1–D4; architecture draft; prior-art survey
(SPEC §1.1 — no direct competitor; craftzdog/tmux-claude-session-manager
and sainnhe/tmux-fzf are the reference implementations; "gearshifter"
name unclaimed).

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
4. **M3 deck: DONE (2026-07-05, merged to main).** Full phase log in
   M3-DECK.md. 13-col Fibonacci grid (`deck`), Button/Gear/Launcher tiles
   (`widget`), lipgloss Layer/Compositor hit-testing (one compositor
   renders AND answers clicks), embedded palette behind the Launcher and
   `/`. V7 answered: gears read `~/.claude/settings.json` (model,
   effortLevel), session-specific model via pane pid → sessions registry →
   transcript scan, mtime-arbitrated (`internal/agent` Provider seam,
   `agent/claude` implementation; the registry/transcript/path mechanics
   are delegated to the shared agent-ouija library — only the
   mtime-arbitration policy lives here). layout.toml (`internal/layout`,
   go-toml/v2 strict): [[tile]] col+span, rows always derived by skyline
   `flow`, line-precise errors to the status line, `examples/layout.toml`
   pinned to `Default` by test; min-canvas rule degrades to a clear
   message. **Deck is the binary default**; telescope = `--layout
   telescope`, never delete it. Post-review refactor (e319781): app
   records selection / main injects; `layout` is the only bridge package.
5. **M4 local fit: DONE (2026-07-05, M4-LOCAL-FIT.md).** Re-scoped with
   the maintainer after M3: the M3 deliverable is a complete local MVP, so the old
   distribution-flavored M4 was premature. Shipped: `gearshifter.tmux`
   entry point (a tmux.conf `run-shell` line installs the binding
   permanently; `just bind-dev` sources the same file; builds the binary
   on first run when `go` exists) + `scripts/open-popup.sh` reading
   `@gearshifter-layout`/`-width`/`-height` at KEYPRESS time
   (`@gearshifter-key` at bind time); qa-rig step 11 drives prefix+C-g
   end-to-end. `internal/config` stays unbuilt — the shell seam covers
   @options until Go needs them.
6. **M5 aesthetic (in flight, M5-AESTHETIC.md rev 2):** P1 theme seam,
   P2 tile states + nameplate buttons, P4 wordmark gradient SHIPPED;
   P3 clawd mascot skipped for now (k9s recipe documented for later);
   structural frame-dedup done. **P5 DECIDED 2026-07-06, M5 COMPLETE:**
   the P1 stand-in IS the house palette (see State above); the
   TUI-AESTHETICS.md designed-ramp verdict and DMG/PICO-8 alternates
   were dropped, not deferred.
7. **M6 distribution (parked until an external user exists):** TPM
   bootstrap, goreleaser, CI, bump/release recipes; also parked: opt-in
   per-session effort shim (hook/statusline). The session-state package
   extraction formerly parked here SHIPPED as the shared agent-ouija
   library (2026-07-05) — see "Shared library" below.

Open items:

- Per-session /effort display: no per-session disk source exists (only
  statusline stdin / hook payloads carry effort.level) — opt-in shim
  parked for M4+; gears show the global value meanwhile.
- Smaller spec items (SPEC §12): V1 plugin active-version resolution, V5
  shadowing precedence, V6 input queuing while Claude is generating.
