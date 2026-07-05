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

open_popup() { # extra pick args pass through (e.g. --layout telescope)
  tmux -L "$SOCK" display-popup -E -w 70% -h 60% \
    "$PWD/bin/gearshifter pick --pane $ORIGIN --cwd $PWD $*" &
  sleep 1.5
}

open_deck() { # -h 90%: the deck needs ~21 inner rows (min-canvas rule degrades below)
  tmux -L "$SOCK" display-popup -E -w 70% -h 90% \
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
move_to() { # col row (1-based): bare motion, no button held (SGR 35)
  printf '\033[<35;%d;%dM' "$1" "$2" \
    | tmux load-buffer -b rigsgr - && tmux paste-buffer -b rigsgr -d -t "$HOST"
}

echo "1. telescope popup opens and lists the catalog"
open_popup --layout telescope
host_screen | rg -q '/add-dir' && ok "catalog visible" || bad "catalog visible"

echo "2. typing filters; Enter injects with Enter (hintless command executes)"
send_host a g e n t s
sleep 0.4
host_screen | rg -q '> agents' && ok "query echoed" || bad "query echoed"
send_host Enter
sleep 1.5
origin_screen | rg -q 'no such file.*agents' && ok "/agents injected+executed" || bad "/agents injected+executed"

echo "3. required-arg command inserts WITHOUT Enter"
open_popup --layout telescope
send_host b t w
sleep 0.4
send_host Enter
sleep 1.5
if origin_screen | rg -q 'no such file.*btw'; then bad "/btw not executed"
elif origin_screen | rg -q '/btw'; then ok "/btw inserted unexecuted"
else bad "/btw inserted unexecuted"; fi
tmux -L "$SOCK" send-keys -t rig C-u

echo "4. Tab inserts without Enter regardless of hint"
open_popup --layout telescope
send_host c o n t e x
sleep 0.4
send_host Tab
sleep 1.5
if origin_screen | rg -q 'no such file.*context'; then bad "tab insert-only"
elif origin_screen | rg -q '/context'; then ok "tab insert-only"
else bad "tab insert-only"; fi
tmux -L "$SOCK" send-keys -t rig C-u

echo "5. mouse click on a located row selects it"
open_popup --layout telescope
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
open_popup --layout telescope
send_host Escape
sleep 1
after=$(origin_screen | tail -1)
[ "$before" = "$after" ] && ok "esc no side effects" || bad "esc no side effects"

echo "7. default layout is the deck (M3 flip)"
open_deck
host_screen | rg -q 'GEARSHIFTER' && ok "deck opens by default" || bad "deck opens by default"
host_screen | rg -q 'MODEL' && ok "gear rail visible" || bad "gear rail visible"

echo "8. clicking a button tile fires its command end-to-end"
SCREEN=$(host_screen)
ROW=$(echo "$SCREEN" | grep -n 'COMPACT' | head -1 | cut -d: -f1 || true)
if [ -z "$ROW" ]; then bad "deck button click (COMPACT not found)"; else
  LINE=$(echo "$SCREEN" | sed -n "${ROW}p")
  PREFIX=${LINE%%COMPACT*}
  click_at $(( ${#PREFIX} + 3 )) "$ROW"
  sleep 1.5
  origin_screen | rg -q 'no such file.*compact' && ok "deck button click" || bad "deck button click"
fi

echo "9. clicking a gear value injects '/model <value>'"
open_deck
SCREEN=$(host_screen)
ROW=$(echo "$SCREEN" | grep -n 'sonnet' | head -1 | cut -d: -f1 || true)
if [ -z "$ROW" ]; then bad "gear value click (sonnet not found)"; else
  LINE=$(echo "$SCREEN" | sed -n "${ROW}p")
  PREFIX=${LINE%%sonnet*}
  click_at $(( ${#PREFIX} + 2 )) "$ROW"
  sleep 1.5
  origin_screen | rg -q 'no such file.*model' && ok "gear value click" || bad "gear value click"
fi

echo "10. hover moves the focus ring (SGR 1003 forwarded into the popup)"
# Plain theme: focus = double-border charset, visible in an uncolored
# capture (the default theme signals focus by color alone).
open_popup --theme plain
SCREEN=$(host_screen)
ROW=$(echo "$SCREEN" | grep -n 'COPY' | head -1 | cut -d: -f1 || true)
if [ -z "$ROW" ]; then bad "hover focus (COPY not found)"; else
  LINE=$(echo "$SCREEN" | sed -n "${ROW}p")
  PREFIX=${LINE%%COPY*}
  move_to $(( ${#PREFIX} + 3 )) "$ROW"
  sleep 0.7
  host_screen | rg -q '║.*COPY' && ok "hover focus" || bad "hover focus"
fi
send_host Escape
sleep 0.5

echo "11. Esc on the deck cancels with zero side effects"
before=$(origin_screen | tail -1)
open_deck
send_host Escape
sleep 1
after=$(origin_screen | tail -1)
[ "$before" = "$after" ] && ok "deck esc no side effects" || bad "deck esc no side effects"

echo "12. plugin entry point binds the key; prefix+C-g opens the deck"
# The rig's popup height default (75%) is honored via open-popup.sh, so
# oversize it for the small rig client the same way open_deck does.
tmux -L "$SOCK" set -g @gearshifter-height 90%
tmux -L "$SOCK" run-shell "$PWD/gearshifter.tmux"
tmux -L "$SOCK" list-keys | rg -q 'C-g.*open-popup' && ok "plugin binds C-g" || bad "plugin binds C-g"
send_host C-b C-g # send-keys writes to the host pane's tty, so C-b reaches the inner server as its prefix
sleep 1.5
host_screen | rg -q 'GEARSHIFTER' && ok "prefix+C-g opens the deck" || bad "prefix+C-g opens the deck"
send_host Escape
sleep 0.5

echo "13. strip: persistent pane widget fires repeatedly and stays alive"
# Step 8 already injected /compact into the origin; clear its screen so
# this step's assertion can't match residue. The strip pins --pane
# explicitly: registry pane-resolution can't work on the throwaway
# server (auto-scan policy is unit-tested in main_test.go instead).
tmux -L "$SOCK" send-keys -t rig "clear" Enter
sleep 0.5
# Split BELOW at full width: the 4×4 deck needs ~24 rows but also full
# columns (a half-width pane truncates span-2 labels and breaks the
# grep), and the origin keeps 100 cols so error lines never wrap.
STRIP=$(tmux -L "$SOCK" split-window -v -l 24 -P -F '#{pane_id}' -t rig \
  "$PWD/bin/gearshifter strip --pane $ORIGIN --cwd $PWD")
sleep 1.5
host_screen | rg -q 'GEARSHIFTER' && ok "strip opens in a pane" || bad "strip opens in a pane"
SCREEN=$(host_screen)
ROW=$(echo "$SCREEN" | grep -n 'COMPACT' | head -1 | cut -d: -f1 || true)
if [ -z "$ROW" ]; then bad "strip fire (COMPACT not found)"; else
  LINE=$(echo "$SCREEN" | sed -n "${ROW}p")
  PREFIX=${LINE%%COMPACT*}
  click_at $(( ${#PREFIX} + 3 )) "$ROW"
  sleep 1.5
  # Captures always target by id: a click-fire hands tmux focus BACK to
  # the target pane (review 2026-07-06), so the active pane is the
  # origin again.
  tmux -L "$SOCK" capture-pane -p -t "$ORIGIN" | rg -q 'no such file.*compact' \
    && ok "strip fire lands in origin" || bad "strip fire lands in origin"
  [ "$(tmux -L "$SOCK" display-message -p '#{pane_id}')" = "$ORIGIN" ] \
    && ok "click fire returns focus to origin" || bad "click fire returns focus to origin"
fi
host_screen | rg -q 'GEARSHIFTER' && ok "strip survives the fire" || bad "strip survives the fire"
SCREEN=$(host_screen)
ROW=$(echo "$SCREEN" | grep -n 'COPY' | head -1 | cut -d: -f1 || true)
if [ -z "$ROW" ]; then bad "strip second fire (COPY not found)"; else
  LINE=$(echo "$SCREEN" | sed -n "${ROW}p")
  PREFIX=${LINE%%COPY*}
  click_at $(( ${#PREFIX} + 2 )) "$ROW"
  sleep 1.5
  tmux -L "$SOCK" capture-pane -p -t "$ORIGIN" | rg -q 'no such file.*copy' \
    && ok "strip fires again" || bad "strip fires again"
fi
# q targets the strip pane by id — focus-return means the active pane
# is the origin, so untargeted keys would land there.
tmux -L "$SOCK" send-keys -t "$STRIP" q
sleep 1
if tmux -L "$SOCK" list-panes -t rig -F '#{pane_id}' | rg -q "$STRIP"; then
  bad "q closes the strip"
else
  ok "q closes the strip"
fi

echo "14. compact strip: chip flow fires, esc survives, q quits"
tmux -L "$SOCK" send-keys -t "$ORIGIN" "clear" Enter
sleep 0.5
# 33 cols = the tcm sidebar default the flow is built for.
CSTRIP=$(tmux -L "$SOCK" split-window -h -l 33 -P -F '#{pane_id}' -t "$ORIGIN" \
  "$PWD/bin/gearshifter strip --compact --pane $ORIGIN --cwd $PWD")
sleep 1.5
SCREEN=$(host_screen)
ROW=$(echo "$SCREEN" | grep -n 'COMPACT' | head -1 | cut -d: -f1 || true)
if [ -z "$ROW" ]; then bad "compact chip visible (COMPACT not found)"; else
  ok "compact chip visible"
  LINE=$(echo "$SCREEN" | sed -n "${ROW}p")
  PREFIX=${LINE%%COMPACT*}
  click_at $(( ${#PREFIX} + 3 )) "$ROW"
  sleep 1.5
  tmux -L "$SOCK" capture-pane -p -t "$ORIGIN" | rg -q 'no such file.*compact' \
    && ok "compact chip fires into origin" || bad "compact chip fires into origin"
fi
# esc must NOT quit a persistent strip (review 2026-07-06).
tmux -L "$SOCK" send-keys -t "$CSTRIP" Escape
sleep 0.5
tmux -L "$SOCK" list-panes -t rig -F '#{pane_id}' | rg -q "$CSTRIP" \
  && ok "esc survives the compact strip" || bad "esc survives the compact strip"
tmux -L "$SOCK" send-keys -t "$CSTRIP" q
sleep 1
if tmux -L "$SOCK" list-panes -t rig -F '#{pane_id}' | rg -q "$CSTRIP"; then
  bad "q closes the compact strip"
else
  ok "q closes the compact strip"
fi

echo
echo "popup-rig: $pass passed, $fail failed"
[ "$fail" -eq 0 ]
