// Package diff parses unified diffs as produced by git and GitHub and turns
// them into rows a terminal can display. It knows nothing about GitHub's API
// or about the user interface.
package diff

import "fmt"

// Status says what happened to a file in the diff.
type Status int

const (
	Modified Status = iota
	Added
	Deleted
	Renamed
	Copied
)

// Letter returns the one-letter code shown next to a file name.
func (s Status) Letter() string {
	switch s {
	case Added:
		return "A"
	case Deleted:
		return "D"
	case Renamed:
		return "R"
	case Copied:
		return "C"
	default:
		return "M"
	}
}

func (s Status) String() string {
	switch s {
	case Added:
		return "added"
	case Deleted:
		return "deleted"
	case Renamed:
		return "renamed"
	case Copied:
		return "copied"
	default:
		return "modified"
	}
}

// LineKind is the kind of one line inside a hunk.
type LineKind int

const (
	LineContext LineKind = iota
	LineAdded
	LineRemoved
)

// Line is one line of a hunk. Old and New are 1-based line numbers; the side
// that does not contain the line is 0.
type Line struct {
	Kind LineKind
	Old  int
	New  int
	// Text is the line without its leading marker and without the line break.
	Text string
	// NoNewline is set when the diff says the file does not end with a line
	// break after this line.
	NoNewline bool
}

// Hunk is one "@@" block of a file's diff.
type Hunk struct {
	OldStart int
	OldLines int
	NewStart int
	NewLines int
	// Section is the text git prints after the second "@@", usually the
	// enclosing function.
	Section string
	Lines   []Line
}

// Header renders the hunk's "@@" line.
func (h Hunk) Header() string {
	s := fmt.Sprintf("@@ -%s +%s @@", span(h.OldStart, h.OldLines), span(h.NewStart, h.NewLines))
	if h.Section != "" {
		s += " " + h.Section
	}
	return s
}

func span(start, count int) string {
	if count == 1 {
		return fmt.Sprintf("%d", start)
	}
	return fmt.Sprintf("%d,%d", start, count)
}

// File is the diff of one file.
type File struct {
	OldPath string
	NewPath string
	Status  Status
	// Binary is set when the diff carries no text for the file because git
	// treats it as binary.
	Binary  bool
	OldMode string
	NewMode string
	Hunks   []Hunk
	// Additions and Deletions count the added and removed lines.
	Additions int
	Deletions int
	// Missing is set when the source withheld the file's diff, which GitHub
	// does for very large files.
	Missing bool
	// Note, when set, is shown in place of the diff of a file without hunks.
	Note string
}

// Path is the path to show for the file: the new path, or the old one for a
// deleted file.
func (f *File) Path() string {
	if f.Status == Deleted || f.NewPath == "" {
		return f.OldPath
	}
	return f.NewPath
}
