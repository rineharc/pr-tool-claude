package files

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/colorprofile"

	"github.com/rineharc/pr-tool-claude/internal/diff"
	"github.com/rineharc/pr-tool-claude/internal/ui/style"
	"github.com/rineharc/pr-tool-claude/internal/ui/uitest"
)

func newModel(t *testing.T, changed []*diff.File, width, height int) *Model {
	t.Helper()
	m := New(changed, style.New(true, colorprofile.TrueColor))
	m.SetSize(width, height)
	return m
}

func press(t *testing.T, m *Model, keys string) {
	t.Helper()
	for _, k := range uitest.Keys(keys) {
		m.Update(uitest.Key(t, k))
	}
}

func screen(t *testing.T, m *Model) []string {
	t.Helper()
	return uitest.Screen(t, m.View(), m.width, m.height)
}

// cursorLine returns the screen line that carries the diff cursor.
func cursorLine(t *testing.T, m *Model) string {
	t.Helper()
	for _, line := range screen(t, m) {
		if strings.Contains(line, "▌") {
			return line
		}
	}
	t.Fatal("no cursor on screen")
	return ""
}

func currentPath(m *Model) string {
	return m.tree.files[m.current].file.Path()
}

func TestStartsOnTheFirstFileWithTheDiffFocused(t *testing.T) {
	m := newModel(t, uitest.DemoPullRequest(t).Files, 120, 37)

	if got := currentPath(m); got != "docs/runbooks/gateway-504.md" {
		t.Errorf("first file is %s", got)
	}
	if m.focus != diffPane {
		t.Error("the diff does not have the keyboard")
	}
	lines := screen(t, m)
	if !strings.Contains(lines[0], "┌─ Files ─") || !strings.Contains(lines[0], "┌─ Diff ─") || !strings.Contains(lines[0], " file 1/10 ─┐") {
		t.Errorf("top border is %q", lines[0])
	}
	if !strings.Contains(lines[1], " 10 files ") || !strings.Contains(lines[1], "+407 -25│") {
		t.Errorf("summary is %q", lines[1])
	}
	if !strings.Contains(lines[1], " docs/runbooks/gateway-504.md  A +38 ") || !strings.Contains(lines[1], "hunk 1/1 │") {
		t.Errorf("file bar is %q", lines[1])
	}
}

func TestFileKeysStepThroughFilesInTreeOrder(t *testing.T) {
	m := newModel(t, uitest.DemoPullRequest(t).Files, 120, 37)

	want := []string{
		"docs/runbooks/gateway-504.md",
		"internal/charge/handler.go",
		"internal/charge/idempotency.go",
		"internal/config/config.go",
		"internal/gateway/client.go",
		"internal/gateway/client_test.go",
		"internal/gateway/retry.go",
		"internal/gateway/retry_test.go",
		"go.mod",
		"go.sum",
	}
	for i, path := range want {
		if got := currentPath(m); got != path {
			t.Fatalf("file %d is %s, want %s", i+1, got, path)
		}
		if got := m.tree.currentFile(); got != i {
			t.Errorf("the tree cursor is on file %d, want %d", got, i)
		}
		if line := screen(t, m)[0]; !strings.Contains(line, fmt.Sprintf(" file %d/10 ", i+1)) {
			t.Errorf("top border is %q", line)
		}
		press(t, m, "]")
	}
	if got := currentPath(m); got != "go.sum" {
		t.Errorf("] on the last file moved to %s", got)
	}

	press(t, m, "[ [ [")
	if got := currentPath(m); got != "internal/gateway/retry.go" {
		t.Errorf("after [ [ [ the file is %s", got)
	}
	press(t, m, "[ [ [ [ [ [ [ [ [ [ [ [")
	if got := currentPath(m); got != want[0] {
		t.Errorf("[ on the first file moved to %s", got)
	}
}

func TestAFileOpensAtItsTop(t *testing.T) {
	m := newModel(t, uitest.DemoPullRequest(t).Files, 120, 37)
	press(t, m, "G ] [")

	if m.diff.cursor != 0 || m.diff.top != 0 {
		t.Errorf("after leaving and returning, cursor = %d and top = %d, want 0 and 0", m.diff.cursor, m.diff.top)
	}
}

func TestLineKeys(t *testing.T) {
	m := newModel(t, uitest.DemoPullRequest(t).Files, 120, 37)
	press(t, m, "] ] ] ]")

	if line := cursorLine(t, m); !strings.Contains(line, "▌@@ -20,18 +20,18 @@") {
		t.Fatalf("cursor starts on %q", line)
	}
	press(t, m, "j j down")
	if line := cursorLine(t, m); !strings.Contains(line, "▌  22   22 ") {
		t.Errorf("after three steps down the cursor is on %q", line)
	}
	press(t, m, "k up")
	if line := cursorLine(t, m); !strings.Contains(line, "▌  20   20 ") {
		t.Errorf("after two steps up the cursor is on %q", line)
	}
	press(t, m, "k k k k")
	if m.diff.cursor != 0 {
		t.Errorf("the cursor left the top: %d", m.diff.cursor)
	}

	press(t, m, "G")
	if m.diff.cursor != len(m.diff.stops)-1 {
		t.Errorf("G put the cursor on line %d of %d", m.diff.cursor, len(m.diff.stops))
	}
	lines := screen(t, m)
	if last := lines[len(lines)-2]; !strings.Contains(last, "▌") {
		t.Errorf("the last diff row does not carry the cursor: %q", last)
	}
	press(t, m, "j end")
	if m.diff.cursor != len(m.diff.stops)-1 {
		t.Error("the cursor left the bottom")
	}

	press(t, m, "g")
	if m.diff.cursor != 0 || m.diff.top != 0 {
		t.Errorf("g put the cursor on %d with top %d", m.diff.cursor, m.diff.top)
	}
	press(t, m, "G home")
	if m.diff.cursor != 0 {
		t.Errorf("home put the cursor on %d", m.diff.cursor)
	}
}

func TestPageKeys(t *testing.T) {
	m := newModel(t, uitest.DemoPullRequest(t).Files, 120, 37)
	press(t, m, "] ] ] ] ] ] ]")
	rows := m.diffRows()
	if rows != 33 {
		t.Fatalf("the diff shows %d rows, want 33", rows)
	}

	press(t, m, "ctrl+d")
	if m.diff.cursor != rows/2 {
		t.Errorf("ctrl+d moved to line %d, want %d", m.diff.cursor, rows/2)
	}
	press(t, m, "pgdown")
	if m.diff.cursor != rows/2+rows {
		t.Errorf("pgdown moved to line %d, want %d", m.diff.cursor, rows/2+rows)
	}
	press(t, m, "ctrl+u")
	if m.diff.cursor != rows {
		t.Errorf("ctrl+u moved to line %d, want %d", m.diff.cursor, rows)
	}
	press(t, m, "pgup pgup")
	if m.diff.cursor != 0 {
		t.Errorf("pgup moved to line %d, want 0", m.diff.cursor)
	}
}

func TestScrollingKeepsAMarginAheadOfTheCursor(t *testing.T) {
	m := newModel(t, uitest.DemoPullRequest(t).Files, 120, 37)
	press(t, m, "] ] ] ] ] ] ]")
	rows := m.diffRows()

	for i := 0; i < 60; i++ {
		press(t, m, "j")
		first, last := m.diff.cursorRows()
		if first < m.diff.top || last >= m.diff.top+rows {
			t.Fatalf("step %d: the cursor rows %d-%d are outside the window starting at %d", i, first, last, m.diff.top)
		}
		if below := m.diff.top + rows - 1 - last; below < scrollMargin && m.diff.top+rows < len(m.diff.rows) {
			t.Fatalf("step %d: only %d rows below the cursor", i, below)
		}
	}
	for i := 0; i < 60; i++ {
		press(t, m, "k")
		first, _ := m.diff.cursorRows()
		if above := first - m.diff.top; above < scrollMargin && m.diff.top > 0 {
			t.Fatalf("step %d up: only %d rows above the cursor", i, above)
		}
	}
}

func TestHunkKeys(t *testing.T) {
	m := newModel(t, uitest.DemoPullRequest(t).Files, 120, 37)
	press(t, m, "] ] ] ]")

	headers := []string{
		"@@ -20,18 +20,18 @@",
		"@@ -42,9 +42,22 @@",
		"@@ -53,18 +66,23 @@",
		"@@ -72,8 +90,26 @@",
	}
	for i, header := range headers {
		if line := cursorLine(t, m); !strings.Contains(line, "▌"+header) {
			t.Fatalf("hunk %d: cursor is on %q, want %s", i+1, line, header)
		}
		if bar := screen(t, m)[1]; !strings.Contains(bar, fmt.Sprintf("hunk %d/4 │", i+1)) {
			t.Errorf("hunk %d: file bar is %q", i+1, bar)
		}
		press(t, m, "}")
	}
	if line := cursorLine(t, m); !strings.Contains(line, "▌"+headers[3]) {
		t.Errorf("} on the last hunk moved to %q", line)
	}

	// From inside a hunk, { goes to that hunk's header first.
	press(t, m, "j j j {")
	if line := cursorLine(t, m); !strings.Contains(line, "▌"+headers[3]) {
		t.Errorf("{ from inside the last hunk moved to %q", line)
	}
	press(t, m, "{ { { { {")
	if line := cursorLine(t, m); !strings.Contains(line, "▌"+headers[0]) {
		t.Errorf("{ on the first hunk moved to %q", line)
	}
}

func TestAHunkJumpShowsTheHunkFromTheTop(t *testing.T) {
	m := newModel(t, uitest.DemoPullRequest(t).Files, 120, 37)
	press(t, m, "] ] ] ] } }")

	lines := screen(t, m)
	if !strings.Contains(lines[4], "▌@@ -53,18 +66,23 @@") {
		t.Errorf("the hunk header is not the second diff row:\n%s", strings.Join(lines[2:6], "\n"))
	}
	// The next step down must not scroll back.
	top := m.diff.top
	press(t, m, "j")
	if m.diff.top != top {
		t.Errorf("stepping down after a jump scrolled from %d to %d", top, m.diff.top)
	}
}

func TestTheCursorTreatsAWrappedLineAsOne(t *testing.T) {
	m := newModel(t, uitest.DemoPullRequest(t).Files, 120, 37)
	press(t, m, "] ] ] ] }")
	// 21 steps from the hunk header is the added line 60, which is too long
	// for the pane and wraps.
	press(t, m, strings.Repeat("j ", 21))

	lines := screen(t, m)
	var marked []string
	for _, line := range lines {
		if strings.Contains(line, "▌") {
			marked = append(marked, line)
		}
	}
	if len(marked) != 2 {
		t.Fatalf("%d rows carry the cursor, want the 2 rows of the wrapped line:\n%s", len(marked), strings.Join(marked, "\n"))
	}
	if !strings.Contains(marked[0], `▌       60 +         return nil, fmt.Errorf("gateway charge after %d`) {
		t.Errorf("first row is %q", marked[0])
	}
	if !strings.Contains(marked[1], `▌          +         ↪ attempts: %w", c.retry.Attempts(), err)`) {
		t.Errorf("second row is %q", marked[1])
	}

	press(t, m, "j")
	if line := cursorLine(t, m); !strings.Contains(line, "▌  48   61 ") {
		t.Errorf("one step down from the wrapped line is %q", line)
	}
	press(t, m, "k k")
	if line := cursorLine(t, m); !strings.Contains(line, "▌  47      - ") {
		t.Errorf("two steps up is %q", line)
	}
}

func TestResizingKeepsTheCursorOnItsLine(t *testing.T) {
	m := newModel(t, uitest.DemoPullRequest(t).Files, 120, 37)
	press(t, m, "] ] ] ] }")
	press(t, m, strings.Repeat("j ", 22))
	want := "  48   61       }"
	if line := cursorLine(t, m); !strings.Contains(line, want) {
		t.Fatalf("cursor is on %q", line)
	}

	for _, size := range [][2]int{{200, 50}, {100, 20}, {80, 21}, {60, 9}, {143, 33}, {120, 37}} {
		m.SetSize(size[0], size[1])
		if line := cursorLine(t, m); !strings.Contains(line, want) {
			t.Errorf("at %dx%d the cursor is on %q", size[0], size[1], line)
		}
	}
}

func TestTreeKeys(t *testing.T) {
	m := newModel(t, uitest.DemoPullRequest(t).Files, 120, 37)
	press(t, m, "tab")
	if m.focus != treePane {
		t.Fatal("tab did not move to the tree")
	}

	// Moving onto a file shows it at once.
	press(t, m, "j j j")
	if got := currentPath(m); got != "internal/charge/handler.go" {
		t.Errorf("the diff shows %s", got)
	}
	// Moving onto a folder leaves the diff alone.
	press(t, m, "j j")
	if n := m.tree.current(); n == nil || n.name != "config" {
		t.Fatalf("the cursor is on %+v", n)
	}
	if got := currentPath(m); got != "internal/charge/idempotency.go" {
		t.Errorf("the diff shows %s", got)
	}

	// Closing a folder hides its files; they are still reachable with ].
	press(t, m, "h")
	lines := strings.Join(screen(t, m), "\n")
	if !strings.Contains(lines, "▸   ▸ config/") || strings.Contains(lines, "config.go ") {
		t.Errorf("the folder did not close:\n%s", lines)
	}
	press(t, m, "]")
	if got := currentPath(m); got != "internal/config/config.go" {
		t.Errorf("] moved to %s", got)
	}
	if n := m.tree.current(); n == nil || n.name != "config.go" {
		t.Errorf("the tree cursor did not follow: %+v", n)
	}

	// Opening a file hands the keyboard to the diff.
	press(t, m, "G")
	if got := currentPath(m); got != "go.sum" {
		t.Errorf("G moved to %s", got)
	}
	press(t, m, "g l")
	if n := m.tree.current(); n == nil || n.name != "gateway-504.md" {
		t.Errorf("l on an open folder did not step into it: %+v", n)
	}
	press(t, m, "enter")
	if m.focus != diffPane {
		t.Error("enter on a file did not move to the diff")
	}
	press(t, m, "shift+tab")
	if m.focus != treePane {
		t.Error("shift+tab did not move back to the tree")
	}
}

func TestHidingTheTree(t *testing.T) {
	m := newModel(t, uitest.DemoPullRequest(t).Files, 120, 37)
	press(t, m, "] ] ] ] } j j j")
	const cursor = "▌  44   44       }"
	if line := cursorLine(t, m); !strings.Contains(line, cursor) {
		t.Fatalf("cursor is on %q", line)
	}

	press(t, m, "t")
	lines := screen(t, m)
	if strings.Contains(lines[0], "Files") || !strings.HasPrefix(lines[0], "┌─ Diff ─") {
		t.Errorf("the tree is still there: %q", lines[0])
	}
	if line := cursorLine(t, m); !strings.HasPrefix(line, "│"+cursor) {
		t.Errorf("the cursor moved, or the diff does not start at the left edge: %q", line)
	}

	// With the tree hidden, tab brings it back and moves to it.
	press(t, m, "tab")
	if m.hidden || m.focus != treePane {
		t.Errorf("after tab: hidden=%v focus=%v", m.hidden, m.focus)
	}
	// Hiding the tree while it has the keyboard hands the keyboard to the diff.
	press(t, m, "t")
	if !m.hidden || m.focus != diffPane {
		t.Errorf("after t: hidden=%v focus=%v", m.hidden, m.focus)
	}
	press(t, m, "t")
	if m.hidden {
		t.Error("t did not bring the tree back")
	}
}

func TestNarrowTerminalShowsOnePaneAtATime(t *testing.T) {
	m := newModel(t, uitest.DemoPullRequest(t).Files, 80, 21)

	lines := screen(t, m)
	if !strings.HasPrefix(lines[0], "┌─ Diff ─") {
		t.Fatalf("a narrow terminal does not start on the diff: %q", lines[0])
	}

	press(t, m, "t")
	lines = screen(t, m)
	if !strings.HasPrefix(lines[0], "┌─ Files ─") || strings.Contains(lines[0], "Diff") {
		t.Fatalf("t did not show the tree alone: %q", lines[0])
	}
	if m.focus != treePane {
		t.Error("the tree does not have the keyboard")
	}

	press(t, m, "j j j enter")
	lines = screen(t, m)
	if !strings.HasPrefix(lines[0], "┌─ Diff ─") || !strings.Contains(lines[1], "internal/charge/handler.go") {
		t.Errorf("enter did not open the file:\n%s\n%s", lines[0], lines[1])
	}
	if m.focus != diffPane {
		t.Error("the diff does not have the keyboard")
	}

	press(t, m, "tab")
	if !m.overlay || m.focus != treePane {
		t.Error("tab did not switch to the tree")
	}
	press(t, m, "tab")
	if m.overlay || m.focus != diffPane {
		t.Error("tab did not switch back to the diff")
	}

	// Growing the window brings both panes back, whatever was showing.
	press(t, m, "t")
	m.SetSize(120, 37)
	lines = screen(t, m)
	if !strings.Contains(lines[0], "┌─ Files ─") || !strings.Contains(lines[0], "┌─ Diff ─") {
		t.Errorf("a wide window does not show both panes: %q", lines[0])
	}
	// Shrinking it while the tree has the keyboard must not leave the
	// keyboard with a pane that is not on screen.
	press(t, m, "tab")
	m.SetSize(80, 21)
	if m.focus == treePane && !m.treeVisible() {
		t.Error("the hidden tree has the keyboard")
	}
}

func TestAwkwardFiles(t *testing.T) {
	m := newModel(t, uitest.MixedFiles(t), 120, 37)

	notes := map[string]string{
		"empty.txt":        "Empty file added.",
		"logo.png":         "Binary file. There is no text diff to show.",
		"pure renamed.txt": "File renamed without changes.",
		"run.sh":           "File mode changed from 100644 to 100755.",
	}
	bars := map[string]string{
		"gone.txt":         " gone.txt  D -2 ",
		"pure renamed.txt": " pure rename.txt → pure renamed.txt  R ",
		"renamed-new.txt":  " renamed-old.txt → renamed-new.txt  R +1 -1 ",
		"naïve file.txt":   " naïve file.txt  A +1 ",
		"logo.png":         " logo.png  M ",
	}
	seen := map[string]bool{}
	for i := 0; i < len(m.tree.files); i++ {
		path := currentPath(m)
		seen[path] = true
		lines := screen(t, m)
		if want, ok := notes[path]; ok && !strings.Contains(lines[3], want) {
			t.Errorf("%s: first diff row is %q, want the note %q", path, lines[3], want)
		}
		if want, ok := bars[path]; ok && !strings.Contains(lines[1], want) {
			t.Errorf("%s: file bar is %q, want %q in it", path, lines[1], want)
		}
		press(t, m, "G g ]")
	}
	for path := range notes {
		if !seen[path] {
			t.Errorf("never saw %s", path)
		}
	}
	for path := range bars {
		if !seen[path] {
			t.Errorf("never saw %s", path)
		}
	}
}

func TestNoNewlineNoteLinesUpWithTheCode(t *testing.T) {
	m := newModel(t, uitest.MixedFiles(t), 120, 37)
	for currentPath(m) != "nonl.txt" {
		before := m.current
		press(t, m, "]")
		if m.current == before {
			t.Fatal("nonl.txt is not in the fixture")
		}
	}

	found := false
	for _, line := range screen(t, m) {
		if strings.Contains(line, `\ No newline at end of file`) {
			found = true
			if !strings.Contains(line, `│             \ No newline at end of file`) {
				t.Errorf("the note is not in the code column: %q", line)
			}
		}
	}
	if !found {
		t.Error("no note about the missing newline")
	}
}

func TestNoFiles(t *testing.T) {
	m := newModel(t, nil, 120, 37)

	lines := screen(t, m)
	if !strings.Contains(strings.Join(lines, "\n"), "This pull request changes no files.") {
		t.Errorf("no message for an empty pull request:\n%s", strings.Join(lines, "\n"))
	}
	if len(m.Hints()) != 0 {
		t.Error("an empty pull request offers keys")
	}
	press(t, m, "j k ] [ { } t tab enter g G ctrl+d")
	screen(t, m)
}

func TestLongNamesAreCutToFit(t *testing.T) {
	long := "a-directory-with-a-very-long-name/and-another-one-inside-it/a_file_whose_name_goes_on_and_on_and_on_for_a_while.go"
	m := newModel(t, []*diff.File{
		{OldPath: long, NewPath: long, Additions: 12345, Deletions: 678, Hunks: []diff.Hunk{{
			OldStart: 1, OldLines: 1, NewStart: 1, NewLines: 1,
			Lines: []diff.Line{{Kind: diff.LineContext, Old: 1, New: 1, Text: "x"}},
		}}},
		{OldPath: "short.go", NewPath: "short.go", Additions: 1},
	}, 100, 20)

	lines := screen(t, m)
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "…") {
		t.Errorf("nothing was cut:\n%s", joined)
	}
	if !strings.Contains(lines[1], "_for_a_while.go  M +12345 -678 ") {
		t.Errorf("the file bar lost the end of the path: %q", lines[1])
	}
	if !strings.Contains(joined, "+12345 -678│") {
		t.Errorf("the tree lost the counts:\n%s", joined)
	}
}

// Every screen has to fill its terminal exactly, at any size and after any
// keys, or the panes tear.
func TestViewAlwaysFillsTheTerminal(t *testing.T) {
	keys := uitest.Keys("j j } ] G ctrl+u tab j j h k l enter t ] ] pgdown { t tab G g [ ctrl+d shift+tab")
	sets := map[string][]*diff.File{
		"demo":  uitest.DemoPullRequest(t).Files,
		"mixed": uitest.MixedFiles(t),
	}
	for name, changed := range sets {
		for width := 60; width <= 200; width += 7 {
			for _, height := range []int{9, 13, 14, 21, 37, 58} {
				m := newModel(t, changed, width, height)
				screen(t, m)
				for _, k := range keys {
					m.Update(uitest.Key(t, k))
					screen(t, m)
					if t.Failed() {
						t.Fatalf("%s at %dx%d after %q", name, width, height, k)
					}
				}
			}
		}
	}
}

func TestLineNumbersWidenForLongFiles(t *testing.T) {
	m := newModel(t, []*diff.File{{
		OldPath: "big.txt", NewPath: "big.txt", Additions: 1,
		Hunks: []diff.Hunk{{
			OldStart: 123456, OldLines: 1, NewStart: 123456, NewLines: 2,
			Lines: []diff.Line{
				{Kind: diff.LineContext, Old: 123456, New: 123456, Text: "before"},
				{Kind: diff.LineAdded, New: 123457, Text: "after"},
			},
		}},
	}}, 120, 20)

	lines := screen(t, m)
	if !strings.Contains(lines[4], "│ 123456 123456   before") {
		t.Errorf("context row is %q", lines[4])
	}
	if !strings.Contains(lines[5], "│        123457 + after") {
		t.Errorf("added row is %q", lines[5])
	}
}
