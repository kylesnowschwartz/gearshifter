# List all available gearshifter dev recipes
default:
    @just --list

# Pre-commit quality floor — gofmt check, go vet, and all Go tests; run after every substantive change
check:
    @test -z "$(gofmt -l .)" || { echo "gofmt needed:"; gofmt -l .; exit 1; }
    go vet ./...
    go test ./...

# Compile the gearshifter CLI into git-ignored bin/gearshifter
build:
    @go build -o bin/gearshifter ./cmd/gearshifter

# {{cwd}}: directory for project-scoped commands (defaults to where just was invoked)
# Browse the command catalog, one aligned row per command with the description truncated to fit the terminal
ls cwd=invocation_directory(): build
    @bin/gearshifter list --cwd "{{cwd}}" | awk -F'\t' -v w="$(tput cols)" 'BEGIN{d=w-58; if(d<20)d=20} {printf "%-28s %-8s %-16.16s %.*s\n", $1, $2, $3, d, $4}'

# NOTE: display-popup does NOT format-expand its shell command (verified tmux 3.6a) — resolve pane/cwd BEFORE opening the popup
# Open the gearshifter palette in a tmux popup; selecting a command injects it into the pane the popup was launched from
popup: build
    tmux display-popup -E -w 70% -h 60% "{{justfile_directory()}}/bin/gearshifter pick --pane '$TMUX_PANE' --cwd '{{invocation_directory()}}'"

# Bind prefix+C-g to open the palette over the current pane (dev daily-driving; run once per tmux server, `tmux unbind C-g` to remove)
bind-dev: build
    tmux bind-key C-g run-shell "tmux display-popup -E -w 70% -h 60% '{{justfile_directory()}}/bin/gearshifter pick --pane #{pane_id} --cwd \"#{pane_current_path}\"'"
    @echo "bound: prefix+C-g opens the palette (origin pane captured at keypress)"

# {{pane}}: target tmux pane id; find it by running `tmux display -p '#{{pane_id}}'` in that pane
# {{text}}: text to inject (defaults to /context)
# Inject a slash command into a live Claude Code pane and press Enter — the manual QA "money test"
inject pane text="/context": build
    bin/gearshifter inject --pane "{{pane}}" "{{text}}"

# End-to-end smoke in a disposable tmux session — lists the catalog, injects into a live pane, asserts execution; catches real-tmux quirks fakes cannot
qa-tmux: build
    #!/usr/bin/env bash
    set -euo pipefail
    session=gearshifter-qa
    tmux kill-session -t "$session" 2>/dev/null || true
    tmux new-session -d -s "$session" -x 80 -y 12
    trap 'tmux kill-session -t "$session" 2>/dev/null || true' EXIT
    pane=$(tmux display-message -p -t "$session" '#{pane_id}')
    sleep 1
    count=$(bin/gearshifter list --cwd "$PWD" | wc -l)
    [ "$count" -gt 50 ] || { echo "qa-tmux: FAIL — catalog only $count commands"; exit 1; }
    bin/gearshifter inject --pane "$pane" "echo gearshifter-qa-ok"
    sleep 1
    tmux capture-pane -p -t "$session" | grep -q gearshifter-qa-ok \
        || { echo "qa-tmux: FAIL — injected command did not run"; exit 1; }
    echo "qa-tmux: PASS ($count commands listed, injection executed)"

# Automated popup QA rig — drives the real palette in a display-popup with synthetic keys and mouse clicks, asserting filter/inject/Tab/Esc behavior end-to-end
qa-rig:
    bash scripts/qa/popup-rig.sh

# Regenerate the vendored builtins table from the official Claude Code commands docs; run on release or when Claude Code ships a new version
builtins:
    curl -sL https://code.claude.com/docs/en/commands.md -o /tmp/gearshifter-commands.md
    go run ./tools/genbuiltins -in /tmp/gearshifter-commands.md -out internal/catalog/builtins.tsv
