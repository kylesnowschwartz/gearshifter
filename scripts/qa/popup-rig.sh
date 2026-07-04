#!/usr/bin/env bash
# popup-rig: automated end-to-end QA of the pick palette inside a real
# display-popup, driven by synthetic keys and SGR mouse clicks.
#
# Method (S1/P0 rig): a throwaway tmux server (-L) hosts the popup; a pane in
# the CURRENT server attaches to it as a client. Keys and SGR mouse bytes
# pasted into that host pane exercise real tmux input routing. Popups render
# on the client, not the pane, so assertions about popup CONTENT read the
# composited host pane; assertions about INJECTION read the origin pane on
# the throwaway server. Popup geometry depends on the client size — always
# locate rows by grepping the composited screen, never by arithmetic.
#
# Requires: running inside tmux (for the host pane).
set -euo pipefail

cd "$(dirname "$0")/../.."
[ -n "${TMUX:-}" ] || { echo "popup-rig: must run inside tmux"; exit 1; }
go build -o bin/gearshifter ./cmd/gearshifter

SOCK=gearshifter-rig
LABEL=gearshifter-rig
pass=0 fail=0
ok()   { echo "  PASS $1"; pass=$((pass+1)); }
bad()  { echo "  FAIL $1"; fail=$((fail+1)); }

tmux -L "$SOCK" kill-server 2>/dev/null || true
tmux -L "$SOCK" -f /dev/null new-session -d -s rig -x 100 -y 30 -c "$PWD"
tmux -L "$SOCK" set -g mouse on
HOST=$(tmux new-window -d -P -F '#{pane_id}' -n "$LABEL" "tmux -L $SOCK attach -t rig")
cleanup() {
  tmux -L "$SOCK" kill-server 2>/dev/null || true
  tmux kill-window -t "$LABEL" 2>/dev/null || true
}
trap cleanup EXIT
sleep 1
ORIGIN=$(tmux -L "$SOCK" display-message -p -t rig '#{pane_id}')

open_popup() {
  tmux -L "$SOCK" display-popup -E -w 70% -h 60% \
    "$PWD/bin/gearshifter pick --pane $ORIGIN --cwd $PWD" &
  sleep 1.5
}

host_screen() { tmux capture-pane -p -t "$HOST"; }
origin_screen() { tmux -L "$SOCK" capture-pane -p -t rig; }
send_host() { tmux send-keys -t "$HOST" "$@"; }
click_at() { # col row (1-based screen coords)
  printf '\033[<0;%d;%dM\033[<0;%d;%dm' "$1" "$2" "$1" "$2" \
    | tmux load-buffer -b rigsgr - && tmux paste-buffer -b rigsgr -d -t "$HOST"
}

echo "1. popup opens and lists the catalog"
open_popup
host_screen | rg -q '/add-dir' && ok "catalog visible" || bad "catalog visible"

echo "2. typing filters; Enter injects with Enter (hintless command executes)"
send_host a g e n t s
sleep 0.4
host_screen | rg -q '> agents' && ok "query echoed" || bad "query echoed"
send_host Enter
sleep 1.5
origin_screen | rg -q 'no such file.*agents' && ok "/agents injected+executed" || bad "/agents injected+executed"

echo "3. required-arg command inserts WITHOUT Enter"
open_popup
send_host b t w
sleep 0.4
send_host Enter
sleep 1.5
if origin_screen | rg -q 'no such file.*btw'; then bad "/btw not executed"
elif origin_screen | rg -q '/btw'; then ok "/btw inserted unexecuted"
else bad "/btw inserted unexecuted"; fi
tmux -L "$SOCK" send-keys -t rig C-u

echo "4. Tab inserts without Enter regardless of hint"
open_popup
send_host c o n t e x
sleep 0.4
send_host Tab
sleep 1.5
if origin_screen | rg -q 'no such file.*context'; then bad "tab insert-only"
elif origin_screen | rg -q '/context'; then ok "tab insert-only"
else bad "tab insert-only"; fi
tmux -L "$SOCK" send-keys -t rig C-u

echo "5. mouse click on a located row selects it"
open_popup
send_host d o c t
sleep 0.4
SCREEN=$(host_screen)
ROW=$(echo "$SCREEN" | grep -n '/doctor' | head -1 | cut -d: -f1 || true)
if [ -z "$ROW" ]; then bad "click select (/doctor row not found)"; else
  LINE=$(echo "$SCREEN" | sed -n "${ROW}p")
  PREFIX=${LINE%%/doctor*}
  click_at $(( ${#PREFIX} + 3 )) "$ROW"
  sleep 1.5
  origin_screen | rg -q 'no such file.*doctor' && ok "click select" || bad "click select"
fi

echo "6. Esc cancels with zero side effects"
before=$(origin_screen | tail -1)
open_popup
send_host Escape
sleep 1
after=$(origin_screen | tail -1)
[ "$before" = "$after" ] && ok "esc no side effects" || bad "esc no side effects"

echo
echo "popup-rig: $pass passed, $fail failed"
[ "$fail" -eq 0 ]
