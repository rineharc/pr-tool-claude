package diff

import "fmt"

// RowKind is the kind of one display row of a file's diff.
type RowKind int

const (
	RowHunk RowKind = iota
	RowContext
	RowAdded
	RowRemoved
	// RowNote is a message that stands in for content, such as the notice
	// for a binary file.
	RowNote
)

// Row is one terminal row of a file's diff. A long line becomes several rows;
// only the first carries line numbers.
type Row struct {
	Kind RowKind
	Old  int
	New  int
	Text string
	// Continuation is set on the second and later rows of a wrapped line.
	Continuation bool
	// Hunk is the index of the hunk the row belongs to, or -1.
	Hunk int
}

// DefaultTabWidth is the number of columns between tab stops.
const DefaultTabWidth = 4

// Rows lays out a file's diff for a code column of codeWidth columns.
func Rows(f *File, codeWidth int) []Row {
	if note := fileNote(f); note != "" && len(f.Hunks) == 0 {
		return []Row{{Kind: RowNote, Text: note, Hunk: -1}}
	}

	var rows []Row
	if f.OldMode != "" && f.NewMode != "" && f.OldMode != f.NewMode {
		rows = append(rows, Row{Kind: RowNote, Text: modeNote(f), Hunk: -1})
	}
	for hi, h := range f.Hunks {
		rows = append(rows, Row{Kind: RowHunk, Text: Display(h.Header(), DefaultTabWidth), Hunk: hi})
		for _, l := range h.Lines {
			kind := RowContext
			switch l.Kind {
			case LineAdded:
				kind = RowAdded
			case LineRemoved:
				kind = RowRemoved
			}
			for i, part := range Wrap(Display(l.Text, DefaultTabWidth), codeWidth) {
				row := Row{Kind: kind, Text: part, Hunk: hi, Continuation: i > 0}
				if i == 0 {
					row.Old, row.New = l.Old, l.New
				}
				rows = append(rows, row)
			}
			if l.NoNewline {
				rows = append(rows, Row{Kind: RowNote, Text: "No newline at end of file", Hunk: hi})
			}
		}
	}
	return rows
}

// fileNote explains why a file has no hunks to show.
func fileNote(f *File) string {
	switch {
	case f.Note != "":
		return f.Note
	case f.Missing:
		return "GitHub did not return a diff for this file because it is too large."
	case f.Binary:
		return "Binary file. There is no text diff to show."
	case len(f.Hunks) > 0:
		return ""
	case f.Status == Renamed:
		return "File renamed without changes."
	case f.Status == Copied:
		return "File copied without changes."
	case f.OldMode != "" && f.NewMode != "" && f.OldMode != f.NewMode:
		return modeNote(f)
	case f.Status == Added:
		return "Empty file added."
	case f.Status == Deleted:
		return "Empty file deleted."
	default:
		return "No changes to show."
	}
}

func modeNote(f *File) string {
	return fmt.Sprintf("File mode changed from %s to %s.", f.OldMode, f.NewMode)
}

// MaxLineNumber is the largest line number that appears in the file's diff.
// The gutter is sized from it.
func MaxLineNumber(f *File) int {
	largest := 0
	for _, h := range f.Hunks {
		if end := h.OldStart + h.OldLines; end > largest {
			largest = end
		}
		if end := h.NewStart + h.NewLines; end > largest {
			largest = end
		}
	}
	return largest
}
