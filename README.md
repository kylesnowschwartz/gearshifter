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

## Build from source

```sh
just build   # or: go build -o bin/gearshifter ./cmd/gearshifter
just check   # fmt, vet, test
```

## License

[MIT](LICENSE)
