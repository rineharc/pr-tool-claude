package diff

import (
	"strings"
	"unicode/utf8"

	"github.com/rivo/uniseg"
)

// ContinuationMarker starts every continuation row of a wrapped line.
const ContinuationMarker = "↪ "

// minWrapTail is the smallest number of columns a word break may leave on a
// row. Below it the line is cut mid-word instead.
const minWrapTail = 8

// Display prepares a line of file content for the terminal: tabs become
// spaces up to the next tab stop, a trailing carriage return is dropped, and
// other control characters are replaced so they cannot move the cursor or
// change colors.
func Display(s string, tabWidth int) string {
	s = strings.TrimSuffix(s, "\r")
	if !needsCleaning(s) {
		return s
	}
	if !utf8.ValidString(s) {
		s = strings.ToValidUTF8(s, "\uFFFD")
	}
	if tabWidth < 1 {
		tabWidth = 1
	}
	var b strings.Builder
	col := 0
	state := -1
	for len(s) > 0 {
		var cluster string
		var width int
		cluster, s, width, state = uniseg.FirstGraphemeClusterInString(s, state)
		switch {
		case cluster == "\t":
			n := tabWidth - col%tabWidth
			b.WriteString(strings.Repeat(" ", n))
			col += n
		case isControl(cluster):
			b.WriteString("·")
			col++
		default:
			b.WriteString(cluster)
			col += width
		}
	}
	return b.String()
}

func needsCleaning(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < 0x20 || s[i] == 0x7f || s[i] >= 0xc0 {
			return true
		}
	}
	return false
}

// isControl reports whether a grapheme cluster is a C0 or C1 control
// character or DEL.
func isControl(cluster string) bool {
	for _, r := range cluster {
		if r < 0x20 || (r >= 0x7f && r < 0xa0) {
			return true
		}
	}
	return false
}

type cell struct {
	text  string
	width int
}

func cells(s string) []cell {
	out := make([]cell, 0, len(s))
	state := -1
	for len(s) > 0 {
		var cluster string
		var width int
		cluster, s, width, state = uniseg.FirstGraphemeClusterInString(s, state)
		out = append(out, cell{cluster, width})
	}
	return out
}

// Width is the number of terminal columns s occupies.
func Width(s string) int {
	return uniseg.StringWidth(s)
}

// Wrap breaks a line of code into rows of at most width columns. It prefers
// to break at a space; continuation rows keep the line's indentation and
// start with ContinuationMarker. Text that fits is returned unchanged.
func Wrap(s string, width int) []string {
	if width < 1 {
		width = 1
	}
	if len(s) <= width && isASCII(s) {
		return []string{s}
	}
	cs := cells(s)
	total := 0
	for _, c := range cs {
		total += c.width
	}
	if total <= width {
		return []string{s}
	}

	leading := 0
	for leading < len(cs) && cs[leading].text == " " {
		leading++
	}
	indent := leading
	markerWidth := Width(ContinuationMarker)
	// Deeply indented code would leave no room on continuation rows.
	if width-indent-markerWidth < minWrapTail {
		indent = 0
	}
	prefix := strings.Repeat(" ", indent) + ContinuationMarker
	contWidth := width - indent - markerWidth
	if contWidth < 1 {
		prefix, contWidth = "", width
	}

	var rows []string
	rest := cs
	first := true
	for len(rest) > 0 {
		limit := contWidth
		minBreak := minWrapTail
		if first {
			limit = width
			minBreak = leading + minWrapTail
		}
		n, fits := take(rest, limit)
		if !fits {
			if b := lastSpace(rest, n, minBreak); b > 0 {
				n = b
			}
		}
		if n == 0 {
			// A single cell wider than the row: emit it anyway.
			n = 1
		}
		row := join(rest[:n])
		rest = rest[n:]
		if !fits {
			for len(rest) > 0 && rest[0].text == " " {
				rest = rest[1:]
			}
		}
		if first {
			rows = append(rows, row)
		} else {
			rows = append(rows, prefix+row)
		}
		first = false
	}
	return rows
}

// take returns how many leading cells fit in limit columns and whether that
// is all of them.
func take(cs []cell, limit int) (n int, all bool) {
	w := 0
	for i, c := range cs {
		if w+c.width > limit {
			return i, false
		}
		w += c.width
	}
	return len(cs), true
}

// lastSpace finds the last space among cs[:n+1] that leaves at least minBreak
// cells before it. It returns the index of that space, or 0 if there is none.
func lastSpace(cs []cell, n, minBreak int) int {
	if n >= len(cs) {
		n = len(cs) - 1
	}
	for i := n; i >= minBreak && i > 0; i-- {
		if cs[i].text == " " {
			return i
		}
	}
	return 0
}

func join(cs []cell) string {
	var b strings.Builder
	for _, c := range cs {
		b.WriteString(c.text)
	}
	return b.String()
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 || s[i] < 0x20 {
			return false
		}
	}
	return true
}
