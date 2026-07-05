package tmux

import (
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/kylesnowschwartz/gearshifter/internal/testutil"
)

// TestMain sandboxes HOME (shared invariant; tests must never be able to
// touch the user's real ~/.claude).
func TestMain(m *testing.M) {
	os.Exit(testutil.SandboxHome(m))
}

type fakeRunner struct {
	calls  []string // "stdin>args joined" per call
	stdins []string
}

func (f *fakeRunner) Run(stdin string, args ...string) (string, error) {
	f.calls = append(f.calls, strings.Join(args, " "))
	f.stdins = append(f.stdins, stdin)
	return "", nil
}

// TestInjectRecipe locks in the M0-verified injection sequence (SPEC §6):
// clear (recoverable C-u), load-buffer from stdin, bracketed paste, Enter.
func TestInjectRecipe(t *testing.T) {
	f := &fakeRunner{}
	c := NewClient(f)

	if err := c.Inject("%7", "/model opus", InjectOptions{}); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"send-keys -t %7 C-u",
		"load-buffer -b gearshifter -",
		"paste-buffer -p -d -b gearshifter -t %7",
		"send-keys -t %7 Enter",
	}
	if !reflect.DeepEqual(f.calls, want) {
		t.Errorf("call sequence:\n got %v\nwant %v", f.calls, want)
	}
	if f.stdins[1] != "/model opus" {
		t.Errorf("load-buffer stdin = %q, want the command text", f.stdins[1])
	}
}

func TestInjectNoEnterNoClear(t *testing.T) {
	f := &fakeRunner{}
	c := NewClient(f)

	if err := c.Inject("%7", "/advisor ", InjectOptions{NoClear: true, NoEnter: true}); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"load-buffer -b gearshifter -",
		"paste-buffer -p -d -b gearshifter -t %7",
	}
	if !reflect.DeepEqual(f.calls, want) {
		t.Errorf("call sequence:\n got %v\nwant %v", f.calls, want)
	}
}

func TestInjectValidation(t *testing.T) {
	c := NewClient(&fakeRunner{})
	if err := c.Inject("", "/x", InjectOptions{}); err == nil {
		t.Error("empty pane accepted")
	}
	if err := c.Inject("%7", "", InjectOptions{}); err == nil {
		t.Error("empty text accepted")
	}
}

type cannedRunner struct {
	out      string
	lastArgs []string
}

func (r *cannedRunner) Run(stdin string, args ...string) (string, error) {
	r.lastArgs = args
	return r.out, nil
}

func TestPanePID(t *testing.T) {
	r := &cannedRunner{out: "12345"}
	pid, err := NewClient(r).PanePID("%7")
	if err != nil || pid != 12345 {
		t.Errorf("PanePID = %d, %v; want 12345", pid, err)
	}
	if got := strings.Join(r.lastArgs, " "); got != "display-message -p -t %7 #{pane_pid}" {
		t.Errorf("argv = %q", got)
	}
	if _, err := NewClient(&cannedRunner{out: "not-a-pid"}).PanePID("%7"); err == nil {
		t.Error("garbage pid output must error")
	}
}

func TestWindowPanes(t *testing.T) {
	r := &cannedRunner{out: "%0\t100\t/home/a\n%2\t200\t/home/b"}
	panes, err := NewClient(r).WindowPanes("%0")
	if err != nil {
		t.Fatal(err)
	}
	want := []Pane{{ID: "%0", PID: 100, Cwd: "/home/a"}, {ID: "%2", PID: 200, Cwd: "/home/b"}}
	if !reflect.DeepEqual(panes, want) {
		t.Errorf("WindowPanes = %+v, want %+v", panes, want)
	}
	if got := strings.Join(r.lastArgs, " "); got != "list-panes -t %0 -F #{pane_id}\t#{pane_pid}\t#{pane_current_path}" {
		t.Errorf("argv = %q", got)
	}
}

func TestWindowPanesBadRowsError(t *testing.T) {
	if _, err := NewClient(&cannedRunner{out: "%0\tnot-a-pid\t/x"}).WindowPanes("%0"); err == nil {
		t.Error("garbage pid must error")
	}
	if _, err := NewClient(&cannedRunner{out: "one-field-only"}).WindowPanes("%0"); err == nil {
		t.Error("short row must error")
	}
}
