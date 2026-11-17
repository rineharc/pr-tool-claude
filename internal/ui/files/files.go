// Package files is the Files tab: the tree of changed files next to the
// unified diff of the selected file.
package files

import (
	"fmt"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/rineharc/pr-tool-claude/internal/diff"
	"github.com/rineharc/pr-tool-claude/internal/ui/keymap"
	"github.com/rineharc/pr-tool-claude/internal/ui/style"
)

const (
	// Below this width the tree and the diff no longer fit side by side;
	// the tree is then shown on its own, on request.
	sideBySideWidth = 100
	wideTreeWidth   = 42
	narrowTreeWidth = 34
	// The legend under the tree needs this much room.
	legendMinHeight = 14

	minNumberWidth = 4
	minCodeWidth   = 10

	legend = " M/A/D/R modified/added/deleted/renamed"
)

type pane int

const (
	diffPane pane = iota
	treePane
)

// Hint is a key and what it does, for the footer.
type Hint struct {
	Key    string
	Action string
}

// Model is the state of the Files tab.
type Model struct {
	styles style.Styles
	width  int
	height int

	tree *tree
	diff diffView
	// current is the index, in tree order, of the file the diff shows.
	current int
	focus   pane

	// hidden is the reader's choice, on a wide terminal, to work without the
	// tree. overlay says that the tree is shown in place of the diff on a
	// narrow one.
	hidden  bool
	overlay bool

	additions int
	deletions int
	// The widths of the two count columns in the tree.
	addWidth int
	delWidth int
}

// New returns the Files tab for the given files.
func New(changed []*diff.File, styles style.Styles) *Model {
	m := &Model{styles: styles, tree: newTree(changed)}
	for _, f := range changed {
		m.additions += f.Additions
		m.deletions += f.Deletions
		m.addWidth = max(m.addWidth, len(count("+", f.Additions)))
		m.delWidth = max(m.delWidth, len(count("-", f.Deletions)))
	}
	m.showFile(0)
	return m
}

// SetStyles switches to another set of styles.
func (m *Model) SetStyles(styles style.Styles) {
	m.styles = styles
}

// SetSize tells the tab how much room it has.
func (m *Model) SetSize(width, height int) {
	m.width, m.height = width, height
	if !m.treeVisible() {
		m.focus = diffPane
	} else if !m.diffVisible() {
		m.focus = treePane
	}
	m.relayout()
	m.diff.reveal(m.diffRows(), 0)
}

// Hints lists the keys worth showing in the footer for the focused pane.
func (m *Model) Hints() []Hint {
	if len(m.tree.files) == 0 {
		return nil
	}
	if m.focus == treePane {
		return []Hint{{"j/k", "move"}, {"enter", "open"}, {"h/l", "fold"}, {"tab", "diff"}, {"t", "tree"}}
	}
	return []Hint{{"j/k", "line"}, {"[ ]", "file"}, {"{ }", "hunk"}, {"tab", "files"}, {"t", "tree"}}
}

func (m *Model) wide() bool {
	return m.width >= sideBySideWidth
}

func (m *Model) treeVisible() bool {
	if m.wide() {
		return !m.hidden
	}
	return m.overlay
}

func (m *Model) diffVisible() bool {
	return m.wide() || !m.overlay
}

func (m *Model) treeWidth() int {
	switch {
	case !m.treeVisible():
		return 0
	case !m.wide():
		return m.width
	case m.width >= 120:
		return wideTreeWidth
	default:
		return narrowTreeWidth
	}
}

func (m *Model) diffWidth() int {
	if !m.diffVisible() {
		return 0
	}
	return m.width - m.treeWidth()
}

// diffRows is the number of diff rows that fit: the pane without its
// borders, the file bar and the line under it.
func (m *Model) diffRows() int {
	return max(0, m.height-4)
}

// treeRows is the number of tree rows that fit: the pane without its
// borders, the summary and the blank row under it, and the legend.
func (m *Model) treeRows() int {
	rows := m.height - 4
	if m.legendShown() {
		rows -= 2
	}
	return max(0, rows)
}

func (m *Model) legendShown() bool {
	return m.height >= legendMinHeight && m.treeWidth()-2 >= len(legend)
}

func (m *Model) numberWidth() int {
	if m.diff.file == nil {
		return minNumberWidth
	}
	return max(minNumberWidth, len(strconv.Itoa(diff.MaxLineNumber(m.diff.file))))
}

// gutterWidth is the cursor bar, both line numbers and the sign, with the
// spaces between them.
func (m *Model) gutterWidth() int {
	return 2*m.numberWidth() + 5
}

func (m *Model) relayout() {
	width := m.diffWidth()
	if width == 0 {
		// The diff is not on screen; keep the layout it had.
		width = m.width
	}
	m.diff.layout(max(minCodeWidth, width-2-m.gutterWidth()))
	m.tree.reveal(m.treeRows())
}

func (m *Model) showFile(index int) {
	if index < 0 || index >= len(m.tree.files) {
		return
	}
	m.current = index
	m.diff.setFile(m.tree.files[index].file)
	m.relayout()
}

// Update handles a key press.
func (m *Model) Update(msg tea.KeyPressMsg) {
	if len(m.tree.files) == 0 {
		return
	}
	switch {
	case key.Matches(msg, keymap.NextFile):
		m.stepFile(1)
	case key.Matches(msg, keymap.PrevFile):
		m.stepFile(-1)
	case key.Matches(msg, keymap.Tree):
		m.toggleTree()
	case key.Matches(msg, keymap.NextPane, keymap.PrevPane):
		m.switchPane()
	case m.focus == treePane:
		m.updateTree(msg)
	default:
		m.updateDiff(msg)
	}
}

func (m *Model) stepFile(delta int) {
	index := clamp(m.current+delta, 0, len(m.tree.files)-1)
	m.tree.selectFile(index)
	m.showFile(index)
}

func (m *Model) toggleTree() {
	if m.wide() {
		m.hidden = !m.hidden
		if m.hidden {
			m.focus = diffPane
		}
	} else {
		m.overlay = !m.overlay
		m.focus = diffPane
		if m.overlay {
			m.focus = treePane
		}
	}
	m.relayout()
	m.diff.reveal(m.diffRows(), 0)
}

func (m *Model) switchPane() {
	switch {
	case !m.wide():
		m.toggleTree()
	case m.hidden:
		m.hidden = false
		m.focus = treePane
		m.relayout()
		m.diff.reveal(m.diffRows(), 0)
	case m.focus == treePane:
		m.focus = diffPane
	default:
		m.focus = treePane
	}
}

func (m *Model) updateTree(msg tea.KeyPressMsg) {
	t := m.tree
	switch {
	case key.Matches(msg, keymap.Down):
		t.move(1)
	case key.Matches(msg, keymap.Up):
		t.move(-1)
	case key.Matches(msg, keymap.HalfPageDown):
		t.move(max(1, m.treeRows()/2))
	case key.Matches(msg, keymap.HalfPageUp):
		t.move(-max(1, m.treeRows()/2))
	case key.Matches(msg, keymap.PageDown):
		t.move(max(1, m.treeRows()))
	case key.Matches(msg, keymap.PageUp):
		t.move(-max(1, m.treeRows()))
	case key.Matches(msg, keymap.Top):
		t.move(-len(t.visible))
	case key.Matches(msg, keymap.Bottom):
		t.move(len(t.visible))
	case key.Matches(msg, keymap.Collapse):
		t.collapse()
	case key.Matches(msg, keymap.Open):
		if t.expand() {
			m.focus = diffPane
			m.overlay = false
		}
	default:
		return
	}

	if index := t.currentFile(); index >= 0 && index != m.current {
		m.showFile(index)
	}
	m.relayout()
	m.diff.reveal(m.diffRows(), 0)
}

func (m *Model) updateDiff(msg tea.KeyPressMsg) {
	d := &m.diff
	rows := m.diffRows()
	direction := 1
	switch {
	case key.Matches(msg, keymap.Down):
		d.move(1)
	case key.Matches(msg, keymap.Up):
		d.move(-1)
		direction = -1
	case key.Matches(msg, keymap.HalfPageDown):
		d.move(max(1, rows/2))
	case key.Matches(msg, keymap.HalfPageUp):
		d.move(-max(1, rows/2))
		direction = -1
	case key.Matches(msg, keymap.PageDown):
		d.move(max(1, rows))
	case key.Matches(msg, keymap.PageUp):
		d.move(-max(1, rows))
		direction = -1
	case key.Matches(msg, keymap.Top):
		d.move(-len(d.stops))
		direction = -1
	case key.Matches(msg, keymap.Bottom):
		d.move(len(d.stops))
	case key.Matches(msg, keymap.NextHunk):
		if d.jumpHunk(1) {
			d.showFromTop(rows)
		}
	case key.Matches(msg, keymap.PrevHunk):
		if d.jumpHunk(-1) {
			d.showFromTop(rows)
		}
		direction = -1
	default:
		return
	}
	d.reveal(rows, direction)
}

// View draws the tab as exactly the height and width it was given.
func (m *Model) View() string {
	if m.width <= 0 || m.height <= 0 {
		return ""
	}
	if len(m.tree.files) == 0 {
		return m.emptyView()
	}

	var columns [][]string
	if w := m.treeWidth(); w > 0 {
		columns = append(columns, m.renderTree(w))
	}
	if w := m.diffWidth(); w > 0 {
		columns = append(columns, m.renderDiff(w))
	}

	lines := make([]string, m.height)
	for i := range lines {
		for _, column := range columns {
			lines[i] += column[i]
		}
	}
	return strings.Join(lines, "\n")
}

func (m *Model) emptyView() string {
	body := make([]string, max(0, m.height-2))
	for i := range body {
		body[i] = strings.Repeat(" ", m.width-2)
	}
	if len(body) > 0 {
		body[len(body)/2] = center(m.styles.Dim.Render("This pull request changes no files."), m.width-2)
	}
	return strings.Join(m.frame("Files", "", m.width, false, body), "\n")
}

func (m *Model) renderTree(width int) []string {
	st := m.styles
	inner := width - 2
	t := m.tree

	noun := "files"
	if len(t.files) == 1 {
		noun = "file"
	}
	totals := strings.TrimSpace(st.Added.Render(count("+", m.additions)) + " " + st.Removed.Render(count("-", m.deletions)))
	top := []string{
		style.Spread(fmt.Sprintf(" %d %s", len(t.files), noun), totals, inner),
		strings.Repeat(" ", inner),
	}

	rows := m.treeRows()
	for i := t.top; i < t.top+rows; i++ {
		if i < len(t.visible) {
			top = append(top, m.treeRow(t.visible[i], i == t.cursor, inner))
		} else {
			top = append(top, strings.Repeat(" ", inner))
		}
	}

	sections := [][]string{top}
	if m.legendShown() {
		sections = append(sections, []string{style.Fit(st.Dim.Render(legend), inner)})
	}
	return m.frame("Files", "", width, m.focus == treePane, sections...)
}

func (m *Model) treeRow(n *node, selected bool, width int) string {
	st := m.styles
	marker := " "
	if selected {
		marker = "▸"
	}
	indent := " " + strings.Repeat("  ", n.depth)

	if n.isFolder() {
		arrow := "▾"
		if n.collapsed {
			arrow = "▸"
		}
		label := arrow + " " + n.name + "/"
		switch {
		case selected && m.focus == treePane:
			return st.Selected.Render(style.Fit(marker+indent+label, width))
		case selected:
			return style.Fit(st.Accent.Render(marker)+indent+st.Current.Render(label), width)
		default:
			return style.Fit(marker+indent+label, width)
		}
	}

	f := n.file
	letter := f.Status.Letter()
	adds := fmt.Sprintf("%*s", m.addWidth, count("+", f.Additions))
	dels := fmt.Sprintf("%*s", m.delWidth, count("-", f.Deletions))
	// The counts keep their place; the name gives way when room is short.
	nameWidth := max(1, width-ansi.StringWidth(marker+indent)-2-1-m.addWidth-1-m.delWidth)
	name := style.Fit(n.name, nameWidth)

	switch {
	case selected && m.focus == treePane:
		return st.Selected.Render(style.Fit(marker+indent+letter+" "+name+" "+adds+" "+dels, width))
	case selected:
		name = st.Current.Render(name)
		marker = st.Accent.Render(marker)
	}
	return style.Fit(marker+indent+st.Status(f.Status).Render(letter)+" "+name+" "+st.Added.Render(adds)+" "+st.Removed.Render(dels), width)
}

func (m *Model) renderDiff(width int) []string {
	st := m.styles
	inner := width - 2
	d := &m.diff
	f := d.file

	position := fmt.Sprintf("file %d/%d", m.current+1, len(m.tree.files))

	path := f.Path()
	if (f.Status == diff.Renamed || f.Status == diff.Copied) && f.OldPath != "" && f.OldPath != f.NewPath {
		path = f.OldPath + " → " + f.NewPath
	}
	path = diff.Display(path, diff.DefaultTabWidth)
	stats := st.Status(f.Status).Render(f.Status.Letter())
	if f.Additions > 0 {
		stats += " " + st.Added.Render(count("+", f.Additions))
	}
	if f.Deletions > 0 {
		stats += " " + st.Removed.Render(count("-", f.Deletions))
	}
	where := ""
	if len(f.Hunks) > 0 {
		where = st.Dim.Render(fmt.Sprintf("hunk %d/%d", max(1, d.hunk()), len(f.Hunks))) + " "
	}
	pathWidth := inner - 1 - 2 - ansi.StringWidth(stats) - 2 - ansi.StringWidth(where)
	if pathWidth < 12 {
		pathWidth += ansi.StringWidth(where)
		where = ""
	}
	if ansi.StringWidth(path) > pathWidth {
		path = strings.TrimRight(style.FitLeft(path, max(1, pathWidth)), " ")
	}
	bar := style.Spread(" "+st.Bold.Render(path)+"  "+stats, where, inner)

	rows := m.diffRows()
	body := make([]string, 0, rows)
	first, last := d.cursorRows()
	numberWidth := m.numberWidth()
	for i := d.top; i < d.top+rows; i++ {
		if i < len(d.rows) {
			body = append(body, m.diffRow(d.rows[i], i >= first && i <= last, inner, numberWidth))
		} else {
			body = append(body, strings.Repeat(" ", inner))
		}
	}
	return m.frame("Diff", position, width, m.focus == diffPane, []string{bar}, body)
}

func (m *Model) diffRow(r diff.Row, cursor bool, width, numberWidth int) string {
	st := m.styles
	bar := " "
	if cursor {
		if m.focus == diffPane {
			bar = st.CursorBar.Render("▌")
		} else {
			bar = st.Dim.Render("▌")
		}
	}
	rest := width - 1

	switch r.Kind {
	case diff.RowHunk:
		return bar + st.Hunk.Render(style.Fit(r.Text, rest))
	case diff.RowNote:
		if r.Hunk < 0 {
			return bar + st.Note.Render(style.Fit(r.Text, rest))
		}
		// A note inside a hunk lines up with the code it belongs to.
		gutter := strings.Repeat(" ", 2*numberWidth+4)
		return bar + style.Fit(gutter+st.Note.Render(`\ `+r.Text), rest)
	}

	number := func(n int) string {
		if n == 0 {
			return strings.Repeat(" ", numberWidth)
		}
		return fmt.Sprintf("%*d", numberWidth, n)
	}
	code := style.Fit(r.Text, rest-(2*numberWidth+4))

	switch r.Kind {
	case diff.RowAdded:
		return bar + st.AddedGutter.Render(number(r.Old)+" "+number(r.New)+" + ") + st.AddedCode.Render(code)
	case diff.RowRemoved:
		return bar + st.RemovedGutter.Render(number(r.Old)+" "+number(r.New)+" - ") + st.RemovedCode.Render(code)
	default:
		return bar + st.LineNumber.Render(number(r.Old)+" "+number(r.New)) + "   " + code
	}
}

// frame draws a bordered pane. Sections are separated by a horizontal line;
// every row must already be as wide as the inside of the pane.
func (m *Model) frame(title, right string, width int, focused bool, sections ...[]string) []string {
	st := m.styles
	border := st.Border
	if focused {
		border = st.BorderFocused
	}
	inner := width - 2
	side := border.Render("│")

	lines := []string{topBorder(title, right, width, border, st.PaneTitle, st.Dim)}
	for i, section := range sections {
		if i > 0 {
			lines = append(lines, border.Render("├"+strings.Repeat("─", inner)+"┤"))
		}
		for _, row := range section {
			lines = append(lines, side+row+side)
		}
	}
	return append(lines, border.Render("└"+strings.Repeat("─", inner)+"┘"))
}

// topBorder draws "┌─ Title ───── right ─┐". The text on the right is
// dropped when the pane is too narrow for both.
func topBorder(title, right string, width int, border, titleStyle, rightStyle lipgloss.Style) string {
	inner := width - 2
	fill := inner - 2 - ansi.StringWidth(title) - 1
	if right != "" {
		if withRight := fill - 1 - ansi.StringWidth(right) - 2; withRight >= 1 {
			return border.Render("┌─ ") + titleStyle.Render(title) + border.Render(" "+strings.Repeat("─", withRight)+" ") +
				rightStyle.Render(right) + border.Render(" ─┐")
		}
	}
	if fill < 0 {
		return border.Render("┌" + strings.Repeat("─", inner) + "┐")
	}
	return border.Render("┌─ ") + titleStyle.Render(title) + border.Render(" "+strings.Repeat("─", fill)+"┐")
}

func center(s string, width int) string {
	w := ansi.StringWidth(s)
	if w >= width {
		return style.Fit(s, width)
	}
	left := (width - w) / 2
	return strings.Repeat(" ", left) + s + strings.Repeat(" ", width-w-left)
}

// count formats a line count with its sign, or nothing for zero.
func count(sign string, n int) string {
	if n == 0 {
		return ""
	}
	return sign + strconv.Itoa(n)
}
