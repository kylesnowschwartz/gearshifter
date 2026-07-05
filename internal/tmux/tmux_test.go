package tmux

import (
	"os"
	"reflect"
	"strings"
	"testing"
)

// TestMain sandboxes HOME — same invariant as the catalog package: tests
// must never be able to touch the user's real ~/.claude.
func TestMain(m *testing.M) {
	sandbox, err := os.MkdirTemp("", "gearshifter-test-home-")
	if err != nil {
		panic(err)
	}
	os.Setenv("HOME", sandbox)
	code := m.Run()
	os.RemoveAll(sandbox)
	os.Exit(code)
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
