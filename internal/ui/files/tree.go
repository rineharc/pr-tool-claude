package files

import (
	"sort"
	"strings"

	"github.com/rineharc/pr-tool-claude/internal/diff"
)

// node is a folder or a file in the tree.
type node struct {
	// name is what the row shows. For a folder it can span several levels,
	// such as "docs/runbooks", when the levels in between hold nothing else.
	name     string
	file     *diff.File
	parent   *node
	children []*node
	depth    int
	// index is the file's position in display order, or -1 for a folder.
	index     int
	collapsed bool
}

func (n *node) isFolder() bool {
	return n.file == nil
}

// tree is the file tree with a cursor. Folders come before files, each sorted
// by name; this is also the order in which the diff view steps through files.
type tree struct {
	roots []*node
	// files holds the file nodes in display order.
	files []*node
	// visible holds the rows that are not hidden inside a collapsed folder.
	visible []*node
	cursor  int
	top     int
}

func newTree(changed []*diff.File) *tree {
	root := &node{}
	for _, f := range changed {
		parent := root
		parts := strings.Split(f.Path(), "/")
		for _, part := range parts[:len(parts)-1] {
			parent = parent.folder(part)
		}
		parent.children = append(parent.children, &node{name: parts[len(parts)-1], file: f, parent: parent})
	}

	t := &tree{}
	root.arrange(-1, &t.files)
	t.roots = root.children
	for _, n := range t.roots {
		n.parent = nil
	}
	t.refresh()
	if len(t.files) > 0 {
		t.selectFile(0)
	}
	return t
}

// folder returns the child folder called name, creating it when needed.
func (n *node) folder(name string) *node {
	for _, c := range n.children {
		if c.isFolder() && c.name == name {
			return c
		}
	}
	c := &node{name: name, parent: n, index: -1}
	n.children = append(n.children, c)
	return c
}

// arrange sorts the subtree, merges chains of single folders into one row,
// and numbers the files.
func (n *node) arrange(depth int, files *[]*node) {
	n.depth = depth
	if !n.isFolder() {
		n.index = len(*files)
		*files = append(*files, n)
		return
	}

	for i, c := range n.children {
		for c.isFolder() && len(c.children) == 1 && c.children[0].isFolder() {
			only := c.children[0]
			only.name = c.name + "/" + only.name
			only.parent = n
			c = only
		}
		n.children[i] = c
	}
	sort.SliceStable(n.children, func(i, j int) bool {
		a, b := n.children[i], n.children[j]
		if a.isFolder() != b.isFolder() {
			return a.isFolder()
		}
		if la, lb := strings.ToLower(a.name), strings.ToLower(b.name); la != lb {
			return la < lb
		}
		return a.name < b.name
	})
	for _, c := range n.children {
		c.arrange(depth+1, files)
	}
}

// refresh rebuilds the list of visible rows, keeping the cursor on the same
// node where possible.
func (t *tree) refresh() {
	current := t.current()
	t.visible = t.visible[:0]
	var walk func(nodes []*node)
	walk = func(nodes []*node) {
		for _, n := range nodes {
			t.visible = append(t.visible, n)
			if n.isFolder() && !n.collapsed {
				walk(n.children)
			}
		}
	}
	walk(t.roots)

	for i, n := range t.visible {
		if n == current {
			t.cursor = i
			return
		}
	}
	t.cursor = clamp(t.cursor, 0, len(t.visible)-1)
}

func (t *tree) current() *node {
	if t.cursor < 0 || t.cursor >= len(t.visible) {
		return nil
	}
	return t.visible[t.cursor]
}

// currentFile is the index of the file under the cursor, or -1 when the
// cursor is on a folder.
func (t *tree) currentFile() int {
	if n := t.current(); n != nil && !n.isFolder() {
		return n.index
	}
	return -1
}

func (t *tree) move(delta int) {
	t.cursor = clamp(t.cursor+delta, 0, len(t.visible)-1)
}

// selectFile puts the cursor on the file with the given index, opening the
// folders above it.
func (t *tree) selectFile(index int) {
	if index < 0 || index >= len(t.files) {
		return
	}
	target := t.files[index]
	for p := target.parent; p != nil; p = p.parent {
		p.collapsed = false
	}
	t.refresh()
	for i, n := range t.visible {
		if n == target {
			t.cursor = i
			return
		}
	}
}

// expand opens the folder under the cursor. It reports whether the cursor is
// on a file instead.
func (t *tree) expand() (onFile bool) {
	n := t.current()
	if n == nil {
		return false
	}
	if !n.isFolder() {
		return true
	}
	if n.collapsed {
		n.collapsed = false
		t.refresh()
	} else if len(n.children) > 0 {
		t.cursor++
	}
	return false
}

// collapse closes the folder under the cursor, or moves to the folder that
// holds the current row when there is nothing to close.
func (t *tree) collapse() {
	n := t.current()
	if n == nil {
		return
	}
	if n.isFolder() && !n.collapsed {
		n.collapsed = true
		t.refresh()
		return
	}
	if n.parent == nil {
		return
	}
	for i, v := range t.visible {
		if v == n.parent {
			t.cursor = i
			return
		}
	}
}

// reveal scrolls so that the cursor is within the height rows shown.
func (t *tree) reveal(height int) {
	if height <= 0 {
		return
	}
	if t.cursor < t.top {
		t.top = t.cursor
	}
	if t.cursor >= t.top+height {
		t.top = t.cursor - height + 1
	}
	t.top = clamp(t.top, 0, max(0, len(t.visible)-height))
}

func clamp(v, low, high int) int {
	if high < low {
		return low
	}
	return min(max(v, low), high)
}
