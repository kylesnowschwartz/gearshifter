#!/usr/bin/env bash
# gearshifter.tmux — tmux plugin entry point. Install by sourcing from
# ~/.tmux.conf (or via TPM later):
#
#   run-shell /path/to/gearshifter/gearshifter.tmux
#
# Binds @gearshifter-key (default: prefix+C-g) to open the deck popup
# over the current pane. All other @gearshifter-* options are read at
# KEYPRESS time by scripts/open-popup.sh, so changing them needs no
# re-sourcing; only the key itself is fixed at bind time.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Local-fit build (M4): compile on first run when a go toolchain exists.
# The no-toolchain bootstrap (prebuilt goreleaser assets) is M6 work.
if [ ! -x "$ROOT/bin/gearshifter" ]; then
  if command -v go >/dev/null 2>&1; then
    (cd "$ROOT" && go build -o bin/gearshifter ./cmd/gearshifter)
  else
    tmux display-message "gearshifter: bin/gearshifter missing and no go toolchain — run 'just build' in $ROOT"
    exit 0
  fi
fi

key="$(tmux show-option -gqv @gearshifter-key)"
# run-shell format-expands at keypress (M2 gotcha: display-popup does
# not), so the origin pane and its cwd are resolved here and handed to
# the opener as plain arguments.
tmux bind-key "${key:-C-g}" run-shell "\"$ROOT/scripts/open-popup.sh\" '#{pane_id}' '#{pane_current_path}'"
