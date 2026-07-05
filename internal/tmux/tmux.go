// Package tmux is the only place Gearshifter talks to tmux. It implements
// the injection recipe verified by the M0 spike (SPEC §6): bracketed paste
// via load-buffer/paste-buffer so Claude Code's autocomplete menu can't
// reinterpret the text, then a separate Enter.
package tmux

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// bufferName is the transient tmux paste buffer used for injection.
const bufferName = "gearshifter"

// Runner executes a tmux command. Split out so tests can assert the exact
// argv sequences without a live tmux server.
type Runner interface {
	Run(stdin string, args ...string) (string, error)
}

// ExecRunner runs the real tmux binary.
type ExecRunner struct{}

func (ExecRunner) Run(stdin string, args ...string) (string, error) {
	cmd := exec.Command("tmux", args...)
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("tmux %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

// Client wraps tmux operations against a Runner.
type Client struct {
	run Runner
}

func NewClient(r Runner) *Client {
	if r == nil {
		r = ExecRunner{}
	}
	return &Client{run: r}
}

// InjectOptions controls Inject behavior. Defaults (zero value) implement
// the decided policy: clear the prompt first (recoverable via Ctrl+Y in
// Claude Code) and press Enter (always-enter, decision D2).
type InjectOptions struct {
	NoClear bool // skip the C-u kill-line before pasting
	NoEnter bool // paste only; leave the user to submit
}

// Inject types text into the target pane using the M0-verified recipe.
func (c *Client) Inject(pane, text string, opts InjectOptions) error {
	if pane == "" {
		return fmt.Errorf("inject: target pane required")
	}
	if text == "" {
		return fmt.Errorf("inject: empty text")
	}
	if !opts.NoClear {
		if _, err := c.run.Run("", "send-keys", "-t", pane, "C-u"); err != nil {
			return err
		}
	}
	if _, err := c.run.Run(text, "load-buffer", "-b", bufferName, "-"); err != nil {
		return err
	}
	// -p requests bracketed paste; -d deletes the buffer after pasting.
	if _, err := c.run.Run("", "paste-buffer", "-p", "-d", "-b", bufferName, "-t", pane); err != nil {
		return err
	}
	if !opts.NoEnter {
		if _, err := c.run.Run("", "send-keys", "-t", pane, "Enter"); err != nil {
			return err
		}
	}
	return nil
}

// PaneCwd returns the current working directory of the target pane —
// project-scoped commands resolve relative to the Claude pane's cwd, not
// Gearshifter's (SPEC §5.1 #4).
func (c *Client) PaneCwd(pane string) (string, error) {
	out, err := c.run.Run("", "display-message", "-p", "-t", pane, "#{pane_current_path}")
	if err != nil {
		return "", err
	}
	// display-message exits 0 with empty output for unknown targets.
	if out == "" {
		return "", fmt.Errorf("pane %s not found", pane)
	}
	return out, nil
}

// PanePID returns the target pane's shell process id — the root of the
// process tree that contains the pane's Claude session (V7 P3.5).
func (c *Client) PanePID(pane string) (int, error) {
	out, err := c.run.Run("", "display-message", "-p", "-t", pane, "#{pane_pid}")
	if err != nil {
		return 0, err
	}
	pid, err := strconv.Atoi(out)
	if err != nil {
		return 0, fmt.Errorf("pane %s: bad pid %q", pane, out)
	}
	return pid, nil
}

// PaneExists reports whether the target pane is still alive — the sanity
// check before injecting (SPEC §8). list-panes is used because
// display-message succeeds silently on unknown targets.
func (c *Client) PaneExists(pane string) bool {
	_, err := c.run.Run("", "list-panes", "-t", pane)
	return err == nil
}

// DisplayMessage flashes text in the tmux status line — the only error
// surface a user can see after a display-popup has closed (popup stderr is
// discarded).
func (c *Client) DisplayMessage(text string) error {
	_, err := c.run.Run("", "display-message", text)
	return err
}

// Pane is one row of a window's pane listing — what strip-mode target
// resolution scans (id to inject into, pid+cwd to ask the agent provider
// about).
type Pane struct {
	ID  string
	PID int
	Cwd string
}

// WindowPanes lists the panes of the window containing pane, in tmux
// index order.
func (c *Client) WindowPanes(pane string) ([]Pane, error) {
	out, err := c.run.Run("", "list-panes", "-t", pane, "-F",
		"#{pane_id}\t#{pane_pid}\t#{pane_current_path}")
	if err != nil {
		return nil, err
	}
	var panes []Pane
	for _, line := range strings.Split(out, "\n") {
		if line == "" {
			continue
		}
		fields := strings.SplitN(line, "\t", 3)
		if len(fields) != 3 {
			return nil, fmt.Errorf("list-panes: bad row %q", line)
		}
		pid, err := strconv.Atoi(fields[1])
		if err != nil {
			return nil, fmt.Errorf("pane %s: bad pid %q", fields[0], fields[1])
		}
		panes = append(panes, Pane{ID: fields[0], PID: pid, Cwd: fields[2]})
	}
	return panes, nil
}
