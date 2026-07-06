# gearshifter

A tmux plugin: a `display-popup` control deck for [Claude Code](https://claude.com/claude-code) — a grid of clickable tiles (buttons, gears, launcher) that fire or insert slash commands into a Claude pane.

## Requirements

- tmux 3.6a+
- Go 1.26+ (only needed to build; no prebuilt binaries are distributed yet)

## Install

Add to `~/.tmux.conf`:

```tmux
run-shell /path/to/gearshifter/gearshifter.tmux
```

Reload tmux (`prefix + r`). The plugin builds `bin/gearshifter` on first run if a `go` toolchain is present. This binds `prefix + C-g` to open the deck popup over the current pane.

## Options

Set via `tmux set-option -g`:

| Option | Values | Default |
|---|---|---|
| `@gearshifter-key` | any tmux key | `C-g` |
| `@gearshifter-layout` | `deck` \| `telescope` \| path to a `layout.toml` | `deck` |
| `@gearshifter-theme` | `default` \| `plain` | `default` |
| `@gearshifter-width` | popup width | `70%` |
| `@gearshifter-height` | popup height | `85%` |

See `examples/layout.toml` and `examples/dense.toml` for custom deck layouts.

## Run as a persistent pane (strip)

The popup deck fires once and closes. `gearshifter strip` runs the same deck as a long-lived pane widget: it stays open after every fire, so it can live in your window layout as a permanent control surface.

```sh
# a slim deck at the bottom of the current window
tmux split-window -v -l 8 'gearshifter strip'

# chip flow for a narrow sidebar column (~33 cols)
tmux split-window -v -l 8 'gearshifter strip --compact'
```

The strip re-resolves its target at **every fire**: it scans the current window for a pane running Claude Code (never itself), so Claude panes can come and go under a long-lived strip. Pin an explicit target with `--pane %12` if the window has several.

Behavior worth knowing before you rely on it:

- **Mouse fires return focus to the Claude pane; keyboard fires don't.** Click when you're done driving the strip, use the keyboard when you're staying in it.
- **`Esc` never quits a persistent strip** (it collapses menus). `q` or `ctrl+c` quit.
- **Tab inserts without Enter**, same as the popup; commands with required arguments always insert without firing.
- Gear values refresh on a timer, so the MODEL/EFFORT chips track settings changes made in Claude itself.

Flags: `--pane`, `--cwd`, `--sources`, `--layout` (deck or a `layout.toml` path — telescope is popup-only), `--theme`, `--compact`, `--sort alpha`.

### As a tail-claude-mux companion pane

If you run [tail-claude-mux](https://github.com/kylesnowschwartz/tail-claude-mux), its `companionPane` config embeds the strip below the sidebar in the same column:

```json
{
  "companionPane": {
    "command": "gearshifter strip --compact --theme plain",
    "rows": 8
  }
}
```

tcm treats the command as opaque — any shell command works there — but the compact strip is what the slot was sized for.

## Build from source

```sh
just build   # or: go build -o bin/gearshifter ./cmd/gearshifter
just check   # fmt, vet, test
```

## License

[MIT](LICENSE)
