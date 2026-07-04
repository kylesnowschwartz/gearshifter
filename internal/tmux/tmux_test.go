package tmux

import (
	"reflect"
	"strings"
	"testing"
)

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
