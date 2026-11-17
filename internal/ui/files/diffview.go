package files

import (
	"github.com/rineharc/pr-tool-claude/internal/diff"
)

// scrollMargin is how many rows stay visible beyond the cursor in the
// direction it moves.
const scrollMargin = 3

// diffView shows one file's diff as rows with a line cursor.
type diffView struct {
	file *diff.File
	// codeWidth is the width rows were laid out for; 0 means not laid out.
	codeWidth int
	rows      []diff.Row
	// stops are the rows the cursor can rest on: the first row of every
	// line. The rows a long line wraps onto are skipped.
	stops  []int
	cursor int
	top    int
}

func (d *diffView) setFile(f *diff.File) {
	if d.file == f {
		return
	}
	*d = diffView{file: f}
}

// layout computes the rows for the given code width. The cursor stays on the
// same line when only the width changes.
func (d *diffView) layout(codeWidth int) {
	if d.file == nil || d.codeWidth == codeWidth {
		return
	}
	d.codeWidth = codeWidth
	d.rows = diff.Rows(d.file, codeWidth)
	d.stops = d.stops[:0]
	for i, r := range d.rows {
		if !r.Continuation {
			d.stops = append(d.stops, i)
		}
	}
	d.cursor = clamp(d.cursor, 0, len(d.stops)-1)
}

// cursorRows returns the first and last row of the line under the cursor.
func (d *diffView) cursorRows() (first, last int) {
	if len(d.stops) == 0 {
		return 0, 0
	}
	first = d.stops[d.cursor]
	last = len(d.rows) - 1
	if d.cursor+1 < len(d.stops) {
		last = d.stops[d.cursor+1] - 1
	}
	return first, last
}

func (d *diffView) move(delta int) {
	d.cursor = clamp(d.cursor+delta, 0, len(d.stops)-1)
}

// jumpHunk moves to the next (direction 1) or previous (direction -1) hunk
// header and reports whether there was one.
func (d *diffView) jumpHunk(direction int) bool {
	for i := d.cursor + direction; i >= 0 && i < len(d.stops); i += direction {
		if d.rows[d.stops[i]].Kind == diff.RowHunk {
			d.cursor = i
			return true
		}
	}
	return false
}

// hunk returns the 1-based number of the hunk under the cursor, or 0.
func (d *diffView) hunk() int {
	if len(d.stops) == 0 {
		return 0
	}
	return d.rows[d.stops[d.cursor]].Hunk + 1
}

// reveal scrolls so that the cursor line is among the height rows shown.
// direction is the way the cursor moved (1 down, -1 up, 0 unknown); a margin
// is kept on that side so the reader sees what comes next.
func (d *diffView) reveal(height, direction int) {
	if height <= 0 {
		return
	}
	first, last := d.cursorRows()
	margin := min(scrollMargin, (height-1)/2)

	if direction >= 0 && last > d.top+height-1-margin {
		d.top = last - height + 1 + margin
	}
	if direction <= 0 && first < d.top+margin {
		d.top = first - margin
	}
	if last > d.top+height-1 {
		d.top = last - height + 1
	}
	if first < d.top {
		d.top = first
	}
	d.top = clamp(d.top, 0, max(0, len(d.rows)-height))
}

// showFromTop scrolls so that the cursor line is the second row shown, which
// is where a hunk header reads best.
func (d *diffView) showFromTop(height int) {
	first, _ := d.cursorRows()
	d.top = clamp(first-1, 0, max(0, len(d.rows)-height))
}
