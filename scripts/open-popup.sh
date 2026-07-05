#!/usr/bin/env bash
# open-popup.sh PANE_ID PANE_CWD — open the gearshifter popup over the
# origin pane, honoring tmux user options at keypress time:
#
#   @gearshifter-layout   deck | telescope | /abs/path/layout.toml  (default: deck)
#   @gearshifter-width    popup width                               (default: 70%)
#   @gearshifter-height   popup height                              (default: 75% — the deck needs ~21 rows)
#
# WARNING (V6): never trigger this from a Claude `!` command — injection
# races the !-completion input flush and is silently discarded.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
pane="$1"
cwd="$2"

opt() {
  local v
  v="$(tmux show-option -gqv "$1")"
  echo "${v:-$2}"
}

exec tmux display-popup -E \
  -w "$(opt @gearshifter-width 70%)" -h "$(opt @gearshifter-height 75%)" \
  "\"$ROOT/bin/gearshifter\" pick --layout \"$(opt @gearshifter-layout deck)\" --pane \"$pane\" --cwd \"$cwd\""
