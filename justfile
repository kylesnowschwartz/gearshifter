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
    go build -o bin/gearshifter ./cmd/gearshifter

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

# Regenerate the vendored builtins table from the official Claude Code commands docs; run on release or when Claude Code ships a new version
builtins:
    curl -sL https://code.claude.com/docs/en/commands.md -o /tmp/gearshifter-commands.md
    go run ./tools/genbuiltins -in /tmp/gearshifter-commands.md -out internal/catalog/builtins.tsv
