package files

import (
	"strings"
	"testing"

	"github.com/rineharc/pr-tool-claude/internal/diff"
)

func changed(paths ...string) []*diff.File {
	files := make([]*diff.File, len(paths))
	for i, p := range paths {
		files[i] = &diff.File{OldPath: p, NewPath: p}
	}
	return files
}

// outline draws the visible rows of a tree, one per line, with ">" on the
// cursor row.
func outline(t *tree) string {
	var b strings.Builder
	for i, n := range t.visible {
		marker := "  "
		if i == t.cursor {
			marker = "> "
		}
		name := n.name
		if n.isFolder() {
			name += "/"
			if n.collapsed {
				name += " (closed)"
			}
		}
		b.WriteString(marker + strings.Repeat("  ", n.depth) + name + "\n")
	}
	return b.String()
}

func wantOutline(t *testing.T, tr *tree, want string) {
	t.Helper()
	want = strings.TrimPrefix(want, "\n")
	if got := outline(tr); got != want {
		t.Errorf("tree is\n%s\nwant\n%s", got, want)
	}
}

func TestTreeOrder(t *testing.T) {
	tr := newTree(changed(
		"go.sum",
		"internal/gateway/retry.go",
		"docs/runbooks/gateway-504.md",
		"internal/gateway/client.go",
		"README.md",
		"internal/charge/handler.go",
		"go.mod",
		"Makefile",
		"cmd/prt/main.go",
		"cmd/prt/internal/deep/only.go",
	))

	wantOutline(t, tr, `
  cmd/prt/
    internal/deep/
>     only.go
    main.go
  docs/runbooks/
    gateway-504.md
  internal/
    charge/
      handler.go
    gateway/
      client.go
      retry.go
  go.mod
  go.sum
  Makefile
  README.md
`)

	var order []string
	for _, n := range tr.files {
		order = append(order, n.file.Path())
	}
	want := []string{
		"cmd/prt/internal/deep/only.go",
		"cmd/prt/main.go",
		"docs/runbooks/gateway-504.md",
		"internal/charge/handler.go",
		"internal/gateway/client.go",
		"internal/gateway/retry.go",
		"go.mod",
		"go.sum",
		"Makefile",
		"README.md",
	}
	if strings.Join(order, "\n") != strings.Join(want, "\n") {
		t.Errorf("file order is\n%s\nwant\n%s", strings.Join(order, "\n"), strings.Join(want, "\n"))
	}
	for i, n := range tr.files {
		if n.index != i {
			t.Errorf("%s has index %d, want %d", n.file.Path(), n.index, i)
		}
	}
}

func TestTreeUsesTheOldPathOfADeletedFile(t *testing.T) {
	tr := newTree([]*diff.File{
		{OldPath: "old/gone.txt", Status: diff.Deleted},
		{OldPath: "a/before.txt", NewPath: "b/after.txt", Status: diff.Renamed},
	})
	wantOutline(t, tr, `
  b/
>   after.txt
  old/
    gone.txt
`)
}

func TestTreeFoldAndUnfold(t *testing.T) {
	tr := newTree(changed("a/one.txt", "a/two.txt", "b/three.txt", "top.txt"))
	wantOutline(t, tr, `
  a/
>   one.txt
    two.txt
  b/
    three.txt
  top.txt
`)

	// On a file, collapse moves to the folder that holds it.
	tr.collapse()
	if tr.currentFile() != -1 {
		t.Errorf("cursor is still on a file")
	}
	// On an open folder, it closes the folder.
	tr.collapse()
	wantOutline(t, tr, `
> a/ (closed)
  b/
    three.txt
  top.txt
`)
	// On a closed top-level folder there is nothing left to do.
	tr.collapse()
	wantOutline(t, tr, `
> a/ (closed)
  b/
    three.txt
  top.txt
`)

	// Expanding opens the folder; expanding again steps into it.
	if tr.expand() {
		t.Error("expand() on a folder reported a file")
	}
	if tr.expand() {
		t.Error("expand() on an open folder reported a file")
	}
	wantOutline(t, tr, `
  a/
>   one.txt
    two.txt
  b/
    three.txt
  top.txt
`)
	if !tr.expand() {
		t.Error("expand() on a file did not report it")
	}
}

func TestTreeSelectFileOpensClosedFolders(t *testing.T) {
	tr := newTree(changed("a/b/one.txt", "a/c/two.txt", "top.txt"))
	tr.move(-10)
	tr.collapse()
	wantOutline(t, tr, `
> a/ (closed)
  top.txt
`)

	tr.selectFile(1)
	wantOutline(t, tr, `
  a/
    b/
      one.txt
    c/
>     two.txt
  top.txt
`)
	if got := tr.currentFile(); got != 1 {
		t.Errorf("currentFile() = %d, want 1", got)
	}

	tr.selectFile(99)
	if got := tr.currentFile(); got != 1 {
		t.Errorf("selecting a file that does not exist moved the cursor to %d", got)
	}
}

func TestTreeMoveStopsAtTheEnds(t *testing.T) {
	tr := newTree(changed("a.txt", "b.txt", "c.txt"))
	tr.move(-5)
	if tr.cursor != 0 {
		t.Errorf("cursor = %d, want 0", tr.cursor)
	}
	tr.move(50)
	if tr.cursor != 2 {
		t.Errorf("cursor = %d, want 2", tr.cursor)
	}
}

func TestTreeReveal(t *testing.T) {
	var paths []string
	for _, name := range strings.Split("abcdefghijklmnopqrst", "") {
		paths = append(paths, name+".txt")
	}
	tr := newTree(changed(paths...))

	tr.move(12)
	tr.reveal(5)
	if tr.top != 8 {
		t.Errorf("top = %d after moving down, want 8", tr.top)
	}
	tr.move(-10)
	tr.reveal(5)
	if tr.top != 2 {
		t.Errorf("top = %d after moving up, want 2", tr.top)
	}
	// A taller pane shows the end of the list without leaving a gap.
	tr.move(100)
	tr.reveal(5)
	tr.reveal(15)
	if tr.top != 5 {
		t.Errorf("top = %d after growing, want 5", tr.top)
	}
}

func TestEmptyTree(t *testing.T) {
	tr := newTree(nil)
	if tr.current() != nil || tr.currentFile() != -1 {
		t.Error("an empty tree has a current row")
	}
	tr.move(1)
	tr.collapse()
	tr.expand()
	tr.reveal(10)
	tr.selectFile(0)
}
