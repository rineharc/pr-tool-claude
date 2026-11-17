package ui

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image/color"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/teatest/v2"

	"github.com/rineharc/pr-tool-claude/internal/demo"
	"github.com/rineharc/pr-tool-claude/internal/model"
	"github.com/rineharc/pr-tool-claude/internal/ui/uitest"
)

// newLoaded returns a model that has received its window size and the pull
// request, as it would shortly after starting.
func newLoaded(t *testing.T, pr *model.PullRequest, width, height int) *Model {
	t.Helper()
	m := New(Options{
		Title: pr.Name(),
		Now:   demo.Now,
		Load: func(context.Context) (*model.PullRequest, error) {
			t.Error("the test model must not load anything")
			return nil, nil
		},
	})
	m.Update(tea.WindowSizeMsg{Width: width, Height: height})
	m.Update(loadedMsg{pr})
	return m
}

func press(t *testing.T, m *Model, keys string) tea.Cmd {
	t.Helper()
	var cmd tea.Cmd
	for _, k := range uitest.Keys(keys) {
		_, cmd = m.Update(uitest.Key(t, k))
	}
	return cmd
}

func screen(t *testing.T, m *Model) []string {
	t.Helper()
	return uitest.Screen(t, m.View().Content, m.width, m.height)
}

func quits(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	_, ok := cmd().(tea.QuitMsg)
	return ok
}

func TestGoldenScreens(t *testing.T) {
	// Twelve steps from the second hunk header of client.go is new line 52,
	// where the mockup in the plan has its cursor.
	toLine52 := "] ] ] ] } " + strings.Repeat("j ", 12)

	scenarios := []struct {
		name string
		keys string
	}{
		{"first-file", ""},
		{"client-line-52", toLine52},
		{"tree-focused", toLine52 + "tab k k k k h"},
		{"tree-toggled", toLine52 + "t"},
		{"help", toLine52 + "?"},
		{"end-of-file", "] ] ] ] G"},
	}
	sizes := [][2]int{{120, 40}, {80, 24}}

	for _, sc := range scenarios {
		for _, size := range sizes {
			name := fmt.Sprintf("%s-%dx%d", sc.name, size[0], size[1])
			t.Run(name, func(t *testing.T) {
				m := newLoaded(t, uitest.DemoPullRequest(t), size[0], size[1])
				press(t, m, sc.keys)
				uitest.Golden(t, name, screen(t, m))
			})
		}
	}
}

func TestGoldenAwkwardPullRequest(t *testing.T) {
	pr := &model.PullRequest{
		Host:         "github.com",
		Owner:        "octo-org",
		Repo:         "a-repository-with-a-rather-long-name",
		Number:       12345,
		Title:        "Rename things, replace the logo, and fix the line endings in a title that is far too long for one line of any terminal",
		Author:       "ghost",
		State:        model.StateMerged,
		BaseRef:      "release/2026.11",
		HeadRef:      "octo-fork:tidy-up",
		Commits:      1,
		ChangedFiles: 4200,
		UpdatedAt:    demo.Now().Add(-49 * time.Hour),
		Files:        uitest.MixedFiles(t),
		Notice:       "Showing 13 of 4200 files: GitHub lists at most 3000 files of a pull request.",
	}
	for _, sc := range []struct{ name, keys string }{
		{"awkward-renamed", "] ] ] ] ] ] ] ] ]"},
		{"awkward-binary", "] ] ] ] ]"},
	} {
		for _, size := range [][2]int{{120, 40}, {80, 24}} {
			name := fmt.Sprintf("%s-%dx%d", sc.name, size[0], size[1])
			t.Run(name, func(t *testing.T) {
				m := newLoaded(t, pr, size[0], size[1])
				press(t, m, sc.keys)
				uitest.Golden(t, name, screen(t, m))
			})
		}
	}
}

func TestLoadingScreen(t *testing.T) {
	m := New(Options{Title: "acme/payments-api #482", Load: func(context.Context) (*model.PullRequest, error) { return nil, nil }})
	if got := m.View().Content; got != "" {
		t.Errorf("before the window size is known the view is %q", got)
	}

	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	view := m.View()
	if !view.AltScreen {
		t.Error("the view does not use the alternate screen")
	}
	if view.WindowTitle != "prt · acme/payments-api #482" {
		t.Errorf("window title is %q", view.WindowTitle)
	}
	lines := strings.Split(ansi.Strip(view.Content), "\n")
	if len(lines) != 40 {
		t.Fatalf("the loading screen has %d lines", len(lines))
	}
	if got := strings.TrimSpace(lines[20]); got != "Loading acme/payments-api #482…" {
		t.Errorf("the middle line is %q", got)
	}

	// Keys other than quit do nothing while loading.
	if cmd := press(t, m, "j ] tab ?"); cmd != nil && quits(cmd) {
		t.Error("a navigation key quit the program")
	}
}

func TestQuit(t *testing.T) {
	for _, keys := range []string{"q", "ctrl+c", "j ] q", "? ctrl+c"} {
		m := newLoaded(t, uitest.DemoPullRequest(t), 120, 40)
		if cmd := press(t, m, keys); !quits(cmd) {
			t.Errorf("%q did not quit", keys)
		}
	}

	loading := New(Options{Title: "x", Load: func(context.Context) (*model.PullRequest, error) { return nil, nil }})
	for _, k := range []string{"q", "ctrl+c"} {
		if cmd := press(t, loading, k); !quits(cmd) {
			t.Errorf("%q did not quit while loading", k)
		}
	}
}

func TestQuitCancelsTheLoad(t *testing.T) {
	started := make(chan struct{})
	done := make(chan error, 1)
	m := New(Options{Title: "x", Load: func(ctx context.Context) (*model.PullRequest, error) {
		close(started)
		<-ctx.Done()
		done <- ctx.Err()
		return nil, ctx.Err()
	}})

	go m.load(m.ctx)
	<-started
	press(t, m, "q")

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("the load ended with %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("quitting did not cancel the load")
	}
}

func TestFailedLoadQuitsAndKeepsTheError(t *testing.T) {
	m := New(Options{Title: "x", Load: func(context.Context) (*model.PullRequest, error) { return nil, nil }})
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	boom := errors.New("could not find pull request acme/payments-api #1")

	_, cmd := m.Update(failedMsg{boom})

	if !quits(cmd) {
		t.Error("a failed load did not quit")
	}
	if !errors.Is(m.Err(), boom) {
		t.Errorf("Err() = %v", m.Err())
	}
}

func TestHelp(t *testing.T) {
	m := newLoaded(t, uitest.DemoPullRequest(t), 120, 40)
	before := screen(t, m)

	press(t, m, "?")
	help := strings.Join(screen(t, m), "\n")
	for _, want := range []string{"┌─ Keys ─", "j / k, ↓ / ↑", "Previous / next file", "Previous / next hunk", "Show / hide the file tree", "q, ctrl+c", "Quit"} {
		if !strings.Contains(help, want) {
			t.Errorf("the help does not mention %q", want)
		}
	}
	// The screen underneath stays visible around the box.
	if !strings.Contains(help, "acme/payments-api #482") || !strings.Contains(help, "┌─ Files ─") {
		t.Error("the help replaced the screen instead of covering part of it")
	}

	// While the help is open, other keys do nothing.
	press(t, m, "j j ] tab t")
	press(t, m, "esc")
	after := screen(t, m)
	if strings.Join(before, "\n") != strings.Join(after, "\n") {
		t.Error("keys pressed while the help was open changed the screen underneath")
	}

	for _, closer := range []string{"?", "q", "esc"} {
		press(t, m, "?")
		if !m.help {
			t.Fatal("? did not open the help")
		}
		if cmd := press(t, m, closer); quits(cmd) || m.help {
			t.Errorf("%q with the help open: quit=%v, still open=%v", closer, quits(cmd), m.help)
		}
	}
}

func TestHelpFitsASmallWindow(t *testing.T) {
	m := newLoaded(t, uitest.DemoPullRequest(t), minWidth, minHeight)
	press(t, m, "?")
	lines := screen(t, m)
	if !strings.Contains(strings.Join(lines, "\n"), "┌─ Keys ─") {
		t.Errorf("no help box:\n%s", strings.Join(lines, "\n"))
	}
}

func TestWindowTooSmall(t *testing.T) {
	m := newLoaded(t, uitest.DemoPullRequest(t), 120, 40)
	for _, size := range [][2]int{{59, 40}, {120, 11}, {20, 5}, {1, 1}} {
		m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		content := ansi.Strip(m.View().Content)
		lines := strings.Split(content, "\n")
		if len(lines) != size[1] {
			t.Errorf("%dx%d: %d lines", size[0], size[1], len(lines))
		}
		for _, line := range lines {
			if w := ansi.StringWidth(line); w > size[0] {
				t.Errorf("%dx%d: a line is %d columns wide", size[0], size[1], w)
			}
		}
		if size[0] >= 59 && !strings.Contains(content, "prt needs a window of at least 60 by 12.") {
			t.Errorf("%dx%d: no message about the size: %q", size[0], size[1], content)
		}
	}

	// Growing the window again brings the pull request back.
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	if lines := screen(t, m); !strings.Contains(lines[0], "acme/payments-api #482") {
		t.Errorf("header is %q", lines[0])
	}
}

func TestHeader(t *testing.T) {
	base := func() *model.PullRequest { return uitest.DemoPullRequest(t) }
	tests := []struct {
		name       string
		change     func(*model.PullRequest)
		wantFirst  string
		wantSecond string
	}{
		{
			name:       "open",
			change:     func(*model.PullRequest) {},
			wantFirst:  "Open",
			wantSecond: " mira-okafor wants to merge 6 commits into main from feature/retry-timeouts · updated 2h ago",
		},
		{
			name:       "draft with one commit",
			change:     func(pr *model.PullRequest) { pr.Draft, pr.Commits = true, 1 },
			wantFirst:  "Draft",
			wantSecond: " mira-okafor wants to merge 1 commit into main from feature/retry-timeouts · updated 2h ago",
		},
		{
			name: "merged from a fork",
			change: func(pr *model.PullRequest) {
				pr.State, pr.HeadRef = model.StateMerged, "octo-fork:feature/retry-timeouts"
				pr.UpdatedAt = demo.Now().Add(-72 * time.Hour)
			},
			wantFirst:  "Merged",
			wantSecond: " mira-okafor merged 6 commits into main from octo-fork:feature/retry-timeouts · updated 3d ago",
		},
		{
			name: "closed long ago",
			change: func(pr *model.PullRequest) {
				pr.State = model.StateClosed
				pr.UpdatedAt = time.Date(2025, time.March, 9, 12, 0, 0, 0, time.UTC)
			},
			wantFirst:  "Closed",
			wantSecond: " mira-okafor wanted to merge 6 commits into main from feature/retry-timeouts · updated on Mar 9, 2025",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pr := base()
			tt.change(pr)
			lines := screen(t, newLoaded(t, pr, 120, 40))

			if want := " acme/payments-api #482   Retry idempotent charge requests on gateway timeouts"; !strings.HasPrefix(lines[0], want) {
				t.Errorf("first line is %q", lines[0])
			}
			if !strings.HasSuffix(lines[0], "  "+tt.wantFirst+" ") {
				t.Errorf("first line ends with %q, want the state %q", lines[0][len(lines[0])-12:], tt.wantFirst)
			}
			if got := strings.TrimRight(lines[1], " "); got != tt.wantSecond {
				t.Errorf("second line is\n %q\nwant\n %q", got, tt.wantSecond)
			}
		})
	}
}

func TestAgo(t *testing.T) {
	now := time.Date(2026, time.November, 16, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		before time.Duration
		want   string
	}{
		{0, "just now"},
		{59 * time.Second, "just now"},
		{-5 * time.Minute, "just now"},
		{time.Minute, "1m ago"},
		{59 * time.Minute, "59m ago"},
		{time.Hour, "1h ago"},
		{23*time.Hour + 59*time.Minute, "23h ago"},
		{24 * time.Hour, "1d ago"},
		{29 * 24 * time.Hour, "29d ago"},
		{30 * 24 * time.Hour, "on Oct 17, 2026"},
	}
	for _, tt := range tests {
		if got := ago(now, now.Add(-tt.before)); got != tt.want {
			t.Errorf("ago(%v) = %q, want %q", tt.before, got, tt.want)
		}
	}
}

func TestFooter(t *testing.T) {
	m := newLoaded(t, uitest.DemoPullRequest(t), 120, 40)
	last := func() string {
		lines := screen(t, m)
		return strings.TrimRight(lines[len(lines)-1], " ")
	}

	if got, want := last(), " j/k line · [ ] file · { } hunk · tab files · t tree · ? help · q quit"; got != want {
		t.Errorf("footer with the diff focused is %q, want %q", got, want)
	}
	press(t, m, "tab")
	if got, want := last(), " j/k move · enter open · h/l fold · tab diff · t tree · ? help · q quit"; got != want {
		t.Errorf("footer with the tree focused is %q, want %q", got, want)
	}

	// On a narrow window hints are dropped whole, and help and quit stay.
	press(t, m, "tab")
	m.Update(tea.WindowSizeMsg{Width: 60, Height: 20})
	if got, want := last(), " j/k line · [ ] file · { } hunk · ? help · q quit"; got != want {
		t.Errorf("footer at 60 columns is %q, want %q", got, want)
	}
}

func TestNoticeTakesALineFromTheBody(t *testing.T) {
	pr := uitest.DemoPullRequest(t)
	pr.Notice = "Showing 10 of 4200 files: GitHub lists at most 3000 files of a pull request."
	m := newLoaded(t, pr, 120, 40)

	lines := screen(t, m)
	if got := strings.TrimRight(lines[38], " "); got != " "+pr.Notice {
		t.Errorf("the line above the footer is %q", got)
	}
	if !strings.HasPrefix(lines[37], "└") {
		t.Errorf("the panes do not end above the notice: %q", lines[37])
	}
}

func TestLightAndReducedColorStylesFillTheTerminal(t *testing.T) {
	m := newLoaded(t, uitest.DemoPullRequest(t), 120, 40)
	press(t, m, "] ] ] ] }")
	dark := strings.Join(screen(t, m), "\n")

	m.Update(tea.BackgroundColorMsg{Color: color.White})
	if m.dark {
		t.Error("a white background did not select the light styles")
	}
	for _, profile := range []colorprofile.Profile{colorprofile.TrueColor, colorprofile.ANSI256, colorprofile.ANSI, colorprofile.Ascii, colorprofile.NoTTY} {
		m.Update(tea.ColorProfileMsg{Profile: profile})
		if got := strings.Join(screen(t, m), "\n"); got != dark {
			t.Errorf("with profile %v the text on screen differs from the default", profile)
		}
	}
}

func TestProgramFlow(t *testing.T) {
	pr := uitest.DemoPullRequest(t)
	m := New(Options{
		Title: pr.Name(),
		Now:   demo.Now,
		Load:  func(context.Context) (*model.PullRequest, error) { return pr, nil },
	})
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(120, 40))

	// The renderer only writes the cells that changed, so text that replaces
	// other text need not appear in the output in one piece. Only the first
	// frame after loading is matched; the rest is checked on the final model.
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("gateway-504.md"))
	}, teatest.WithDuration(30*time.Second), teatest.WithCheckInterval(20*time.Millisecond))

	// While the help is open it takes the keys: "]" must not change the file
	// and the first "q" only closes the help.
	tm.Type("]]]]}")
	tm.Type("?]q")
	tm.Type("q")
	tm.WaitFinished(t, teatest.WithFinalTimeout(30*time.Second))

	final, ok := tm.FinalModel(t).(*Model)
	if !ok {
		t.Fatalf("the final model is a %T", tm.FinalModel(t))
	}
	if final.Err() != nil {
		t.Errorf("Err() = %v", final.Err())
	}
	if final.help {
		t.Error("the help was still open at the end")
	}
	if lines := screen(t, final); !strings.Contains(lines[3], "internal/gateway/client.go") || !strings.Contains(lines[3], "hunk 2/4") {
		t.Errorf("the program ended on %q", lines[3])
	}
}

func TestProgramEndsWhenTheLoadFails(t *testing.T) {
	boom := errors.New("GitHub rejected the token")
	m := New(Options{
		Title: "acme/payments-api #482",
		Load:  func(context.Context) (*model.PullRequest, error) { return nil, boom },
	})
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(120, 40))

	tm.WaitFinished(t, teatest.WithFinalTimeout(10*time.Second))

	final := tm.FinalModel(t).(*Model)
	if !errors.Is(final.Err(), boom) {
		t.Errorf("Err() = %v, want the load error", final.Err())
	}
}
