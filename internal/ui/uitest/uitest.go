// Package uitest holds helpers shared by the tests of the user interface
// packages: key presses, screen checks, golden files and fixtures.
package uitest

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/rineharc/pr-tool-claude/internal/demo"
	"github.com/rineharc/pr-tool-claude/internal/diff"
	"github.com/rineharc/pr-tool-claude/internal/github"
	"github.com/rineharc/pr-tool-claude/internal/model"
)

// UpdateEnv is the environment variable that, when set to 1, makes Golden
// rewrite the golden files instead of comparing against them.
const UpdateEnv = "PRT_UPDATE_GOLDEN"

var named = map[string]tea.KeyPressMsg{
	"tab":       {Code: tea.KeyTab},
	"shift+tab": {Code: tea.KeyTab, Mod: tea.ModShift},
	"enter":     {Code: tea.KeyEnter},
	"esc":       {Code: tea.KeyEscape},
	"up":        {Code: tea.KeyUp},
	"down":      {Code: tea.KeyDown},
	"left":      {Code: tea.KeyLeft},
	"right":     {Code: tea.KeyRight},
	"home":      {Code: tea.KeyHome},
	"end":       {Code: tea.KeyEnd},
	"pgup":      {Code: tea.KeyPgUp},
	"pgdown":    {Code: tea.KeyPgDown},
	"ctrl+c":    {Code: 'c', Mod: tea.ModCtrl},
	"ctrl+d":    {Code: 'd', Mod: tea.ModCtrl},
	"ctrl+u":    {Code: 'u', Mod: tea.ModCtrl},
}

// Key returns the message Bubble Tea sends for a key, named the way the key
// map names it: "j", "G", "tab", "ctrl+d".
func Key(t testing.TB, name string) tea.KeyPressMsg {
	t.Helper()
	msg, ok := named[name]
	if !ok {
		runes := []rune(name)
		if len(runes) != 1 {
			t.Fatalf("uitest.Key: unknown key %q", name)
		}
		msg = tea.KeyPressMsg{Code: runes[0], Text: name}
	}
	if got := msg.String(); got != name {
		t.Fatalf("uitest.Key: the message for %q reads %q", name, got)
	}
	return msg
}

// Keys splits a space-separated list such as "] ] } j j" into key names.
func Keys(list string) []string {
	return strings.Fields(list)
}

// Screen removes the styling from a rendered view and checks that it fills
// the terminal exactly: height lines, each width columns wide.
func Screen(t testing.TB, view string, width, height int) []string {
	t.Helper()
	lines := strings.Split(ansi.Strip(view), "\n")
	if len(lines) != height {
		t.Errorf("the view has %d lines, want %d", len(lines), height)
	}
	for i, line := range lines {
		if w := ansi.StringWidth(line); w != width {
			t.Errorf("line %d is %d columns wide, want %d: %q", i+1, w, width, line)
		}
	}
	return lines
}

// Golden compares a screen with the golden file of that name under the
// calling package's testdata directory. Trailing spaces are not stored, so
// that the files survive editors. Run the tests with PRT_UPDATE_GOLDEN=1 to
// rewrite the files.
func Golden(t testing.TB, name string, lines []string) {
	t.Helper()
	trimmed := make([]string, len(lines))
	for i, line := range lines {
		trimmed[i] = strings.TrimRight(line, " ")
	}
	got := strings.Join(trimmed, "\n") + "\n"

	path := filepath.Join("testdata", name+".golden")
	if os.Getenv(UpdateEnv) == "1" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden file (run the tests with %s=1 to create it): %v", UpdateEnv, err)
	}
	if got != string(want) {
		t.Errorf("the screen differs from %s (run the tests with %s=1 if the change is intended)\n--- got ---\n%s--- want ---\n%s", path, UpdateEnv, got, want)
	}
}

// DemoPullRequest loads the bundled demo pull request the way the program
// does: through the GitHub client.
func DemoPullRequest(t testing.TB) *model.PullRequest {
	t.Helper()
	client, err := github.New(github.Options{Host: demo.Ref.Host, Token: demo.Token, Transport: demo.Transport()})
	if err != nil {
		t.Fatal(err)
	}
	pr, err := client.PullRequest(context.Background(), demo.Ref)
	if err != nil {
		t.Fatal(err)
	}
	return pr
}

// MixedFiles parses the diff fixture that holds one file of every awkward
// kind: renamed, binary, deleted, empty, mode change, no final newline, CRLF.
func MixedFiles(t testing.TB) []*diff.File {
	t.Helper()
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate the fixtures")
	}
	text, err := os.ReadFile(filepath.Join(filepath.Dir(here), "..", "..", "diff", "testdata", "mixed.diff"))
	if err != nil {
		t.Fatal(err)
	}
	files, err := diff.Parse(string(text))
	if err != nil {
		t.Fatal(err)
	}
	return files
}
