// Package testutil holds cross-package test helpers.
package testutil

import (
	"os"
	"testing"
)

// SandboxHome points HOME at a throwaway directory for the whole test
// run, so no test — present or future — can reach the user's real
// ~/.claude, even through an accidental env-based lookup (M1 review
// invariant; one implementation so the packages can't drift). Call from
// TestMain: os.Exit(testutil.SandboxHome(m)).
func SandboxHome(m *testing.M) int {
	sandbox, err := os.MkdirTemp("", "gearshifter-test-home-")
	if err != nil {
		panic(err)
	}
	os.Setenv("HOME", sandbox)
	code := m.Run()
	os.RemoveAll(sandbox)
	return code
}
