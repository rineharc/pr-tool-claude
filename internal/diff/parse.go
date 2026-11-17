package diff

import (
	"fmt"
	"strconv"
	"strings"
)

// Parse reads a unified diff covering any number of files, in the format
// `git diff` and GitHub's diff media type produce.
func Parse(text string) ([]*File, error) {
	p := &parser{lines: splitLines(text)}
	var files []*File
	for p.more() {
		if !strings.HasPrefix(p.peek(), "diff --git ") {
			p.next()
			continue
		}
		f, err := p.file()
		if err != nil {
			return nil, err
		}
		files = append(files, f)
	}
	return files, nil
}

// ParsePatch reads the hunks of a single file, as found in the "patch" field
// of GitHub's list of pull request files. It returns the hunks and the number
// of added and removed lines.
func ParsePatch(patch string) (hunks []Hunk, additions, deletions int, err error) {
	p := &parser{lines: splitLines(patch)}
	f := &File{}
	for p.more() {
		if !strings.HasPrefix(p.peek(), "@@ ") {
			p.next()
			continue
		}
		if err := p.hunk(f); err != nil {
			return nil, 0, 0, err
		}
	}
	return f.Hunks, f.Additions, f.Deletions, nil
}

func splitLines(text string) []string {
	lines := strings.Split(text, "\n")
	// A trailing line break leaves one empty element that is not a line.
	if n := len(lines); n > 0 && lines[n-1] == "" {
		lines = lines[:n-1]
	}
	return lines
}

type parser struct {
	lines []string
	pos   int
}

func (p *parser) more() bool   { return p.pos < len(p.lines) }
func (p *parser) peek() string { return p.lines[p.pos] }
func (p *parser) next() string {
	l := p.lines[p.pos]
	p.pos++
	return l
}

func (p *parser) file() (*File, error) {
	f := &File{}
	f.OldPath, f.NewPath = gitHeaderPaths(strings.TrimPrefix(p.next(), "diff --git "))

	for p.more() {
		line := p.peek()
		switch {
		case strings.HasPrefix(line, "diff --git "):
			return f, nil
		case strings.HasPrefix(line, "@@ "):
			if err := p.hunk(f); err != nil {
				return nil, fmt.Errorf("%s: %w", f.Path(), err)
			}
			continue
		case strings.HasPrefix(line, "old mode "):
			f.OldMode = strings.TrimPrefix(line, "old mode ")
		case strings.HasPrefix(line, "new mode "):
			f.NewMode = strings.TrimPrefix(line, "new mode ")
		case strings.HasPrefix(line, "new file mode "):
			f.Status = Added
			f.NewMode = strings.TrimPrefix(line, "new file mode ")
		case strings.HasPrefix(line, "deleted file mode "):
			f.Status = Deleted
			f.OldMode = strings.TrimPrefix(line, "deleted file mode ")
		case strings.HasPrefix(line, "rename from "):
			f.Status = Renamed
			f.OldPath = unquotePath(strings.TrimPrefix(line, "rename from "))
		case strings.HasPrefix(line, "rename to "):
			f.Status = Renamed
			f.NewPath = unquotePath(strings.TrimPrefix(line, "rename to "))
		case strings.HasPrefix(line, "copy from "):
			f.Status = Copied
			f.OldPath = unquotePath(strings.TrimPrefix(line, "copy from "))
		case strings.HasPrefix(line, "copy to "):
			f.Status = Copied
			f.NewPath = unquotePath(strings.TrimPrefix(line, "copy to "))
		case strings.HasPrefix(line, "--- "):
			if path, ok := sidePath(strings.TrimPrefix(line, "--- "), "a/"); ok {
				f.OldPath = path
			}
		case strings.HasPrefix(line, "+++ "):
			if path, ok := sidePath(strings.TrimPrefix(line, "+++ "), "b/"); ok {
				f.NewPath = path
			}
		case strings.HasPrefix(line, "Binary files ") && strings.HasSuffix(line, " differ"),
			line == "GIT binary patch":
			f.Binary = true
		}
		p.next()
	}
	return f, nil
}

// hunk parses one hunk, starting at its "@@" line, and appends it to f. The
// line counts in the header decide where the hunk ends, so content that looks
// like a diff header is still read as content.
func (p *parser) hunk(f *File) error {
	header := p.next()
	h, err := parseHunkHeader(header)
	if err != nil {
		return err
	}
	oldLeft, newLeft := h.OldLines, h.NewLines
	oldNo, newNo := h.OldStart, h.NewStart

	for p.more() {
		line := p.peek()
		if strings.HasPrefix(line, `\`) {
			// "\ No newline at end of file" belongs to the line before it,
			// even when the counts are already used up.
			if n := len(h.Lines); n > 0 {
				h.Lines[n-1].NoNewline = true
			}
			p.next()
			continue
		}
		if oldLeft <= 0 && newLeft <= 0 {
			break
		}
		marker, text := byte(' '), ""
		if line != "" {
			marker, text = line[0], line[1:]
		}
		switch marker {
		case '+':
			h.Lines = append(h.Lines, Line{Kind: LineAdded, New: newNo, Text: text})
			newNo++
			newLeft--
			f.Additions++
		case '-':
			h.Lines = append(h.Lines, Line{Kind: LineRemoved, Old: oldNo, Text: text})
			oldNo++
			oldLeft--
			f.Deletions++
		case ' ':
			h.Lines = append(h.Lines, Line{Kind: LineContext, Old: oldNo, New: newNo, Text: text})
			oldNo++
			newNo++
			oldLeft--
			newLeft--
		default:
			return fmt.Errorf("unexpected line in hunk %q: %q", header, line)
		}
		p.next()
	}
	f.Hunks = append(f.Hunks, h)
	return nil
}

// parseHunkHeader reads "@@ -a,b +c,d @@ section". A missing count means 1.
func parseHunkHeader(line string) (Hunk, error) {
	bad := func() (Hunk, error) { return Hunk{}, fmt.Errorf("malformed hunk header %q", line) }

	rest, ok := strings.CutPrefix(line, "@@ ")
	if !ok {
		return bad()
	}
	ranges, section, ok := strings.Cut(rest, " @@")
	if !ok {
		return bad()
	}
	oldRange, newRange, ok := strings.Cut(ranges, " ")
	if !ok || !strings.HasPrefix(oldRange, "-") || !strings.HasPrefix(newRange, "+") {
		return bad()
	}
	var h Hunk
	var err error
	if h.OldStart, h.OldLines, err = parseRange(oldRange[1:]); err != nil {
		return bad()
	}
	if h.NewStart, h.NewLines, err = parseRange(newRange[1:]); err != nil {
		return bad()
	}
	h.Section = strings.TrimPrefix(section, " ")
	return h, nil
}

func parseRange(s string) (start, count int, err error) {
	a, b, hasCount := strings.Cut(s, ",")
	if start, err = strconv.Atoi(a); err != nil {
		return 0, 0, err
	}
	count = 1
	if hasCount {
		if count, err = strconv.Atoi(b); err != nil {
			return 0, 0, err
		}
	}
	if start < 0 || count < 0 {
		return 0, 0, fmt.Errorf("negative range %q", s)
	}
	return start, count, nil
}

// gitHeaderPaths extracts the two paths from the text after "diff --git ".
// The format is ambiguous when unquoted paths contain " b/", so the "---",
// "+++" and rename lines that follow take precedence when present.
func gitHeaderPaths(s string) (oldPath, newPath string) {
	if strings.HasPrefix(s, `"`) || strings.HasSuffix(s, `"`) {
		a, b := splitQuotedPair(s)
		return strings.TrimPrefix(unquotePath(a), "a/"), strings.TrimPrefix(unquotePath(b), "b/")
	}
	// The usual case: both paths are the same, so the text is "a/P b/P".
	if len(s)%2 == 1 {
		half := len(s) / 2
		a, b := s[:half], s[half+1:]
		if s[half] == ' ' && strings.HasPrefix(a, "a/") && strings.HasPrefix(b, "b/") && a[2:] == b[2:] {
			return a[2:], b[2:]
		}
	}
	if i := strings.Index(s, " b/"); i >= 0 {
		return strings.TrimPrefix(s[:i], "a/"), s[i+3:]
	}
	return strings.TrimPrefix(s, "a/"), ""
}

// splitQuotedPair splits `"a/x y" "b/x y"`, `a/x "b/y"` and `"a/x" b/y`.
func splitQuotedPair(s string) (string, string) {
	if strings.HasPrefix(s, `"`) {
		for i := 1; i < len(s); i++ {
			if s[i] == '\\' {
				i++
				continue
			}
			if s[i] == '"' {
				return s[:i+1], strings.TrimPrefix(s[i+1:], " ")
			}
		}
		return s, ""
	}
	if i := strings.Index(s, ` "`); i >= 0 {
		return s[:i], s[i+1:]
	}
	return s, ""
}

// sidePath reads the path from a "---" or "+++" line. It reports false for
// /dev/null, which carries no path.
func sidePath(s, prefix string) (string, bool) {
	// git appends a tab after paths that contain spaces; other tools append
	// a tab and a timestamp.
	if i := strings.IndexByte(s, '\t'); i >= 0 && !strings.HasPrefix(s, `"`) {
		s = s[:i]
	}
	s = unquotePath(s)
	if s == "/dev/null" {
		return "", false
	}
	return strings.TrimPrefix(s, prefix), true
}

// unquotePath undoes git's C-style quoting of paths with unusual characters.
func unquotePath(s string) string {
	s = strings.TrimRight(s, "\t")
	if len(s) >= 2 && strings.HasPrefix(s, `"`) && strings.HasSuffix(s, `"`) {
		if u, err := strconv.Unquote(s); err == nil {
			return u
		}
	}
	return s
}
