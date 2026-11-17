package diff

import (
	"os"
	"strings"
	"testing"
)

func mustParse(t *testing.T, text string) []*File {
	t.Helper()
	files, err := Parse(text)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return files
}

func byPath(t *testing.T, files []*File, path string) *File {
	t.Helper()
	for _, f := range files {
		if f.Path() == path {
			return f
		}
	}
	var have []string
	for _, f := range files {
		have = append(have, f.Path())
	}
	t.Fatalf("no file %q in diff; have %q", path, have)
	return nil
}

// TestParseGitOutput reads a diff produced by a real `git diff` that covers
// the awkward cases in one go.
func TestParseGitOutput(t *testing.T) {
	data, err := os.ReadFile("testdata/mixed.diff")
	if err != nil {
		t.Fatal(err)
	}
	files := mustParse(t, string(data))

	// Expected values are what `git diff --numstat` reported for the same diff.
	want := []struct {
		path    string
		oldPath string
		status  Status
		adds    int
		dels    int
		hunks   int
		binary  bool
		modes   [2]string
	}{
		{path: "crlf.txt", oldPath: "crlf.txt", status: Modified, adds: 1, dels: 1, hunks: 1},
		{path: "empty.txt", oldPath: "empty.txt", status: Added, modes: [2]string{"", "100644"}},
		{path: "gone.txt", oldPath: "gone.txt", status: Deleted, dels: 2, hunks: 1, modes: [2]string{"100644", ""}},
		{path: "greek.txt", oldPath: "greek.txt", status: Modified, adds: 3, dels: 2, hunks: 2},
		{path: "keep.go", oldPath: "keep.go", status: Modified, adds: 5, dels: 1, hunks: 1},
		{path: "logo.png", oldPath: "logo.png", status: Modified, binary: true},
		{path: "naïve file.txt", oldPath: "naïve file.txt", status: Added, adds: 1, hunks: 1, modes: [2]string{"", "100644"}},
		{path: "nonl.txt", oldPath: "nonl.txt", status: Modified, adds: 1, dels: 1, hunks: 1},
		{path: "pure renamed.txt", oldPath: "pure rename.txt", status: Renamed},
		{path: "renamed-new.txt", oldPath: "renamed-old.txt", status: Renamed, adds: 1, dels: 1, hunks: 1},
		{path: "run.sh", oldPath: "run.sh", status: Modified, modes: [2]string{"100644", "100755"}},
		{path: "sp ace.txt", oldPath: "sp ace.txt", status: Modified, adds: 1, hunks: 1},
		{path: "tricky.txt", oldPath: "tricky.txt", status: Added, adds: 6, hunks: 1, modes: [2]string{"", "100644"}},
	}
	if len(files) != len(want) {
		t.Fatalf("got %d files, want %d", len(files), len(want))
	}
	for i, w := range want {
		f := files[i]
		if f.Path() != w.path || f.OldPath != w.oldPath {
			t.Errorf("file %d: paths = %q (old %q), want %q (old %q)", i, f.Path(), f.OldPath, w.path, w.oldPath)
			continue
		}
		if f.Status != w.status {
			t.Errorf("%s: status = %v, want %v", w.path, f.Status, w.status)
		}
		if f.Additions != w.adds || f.Deletions != w.dels {
			t.Errorf("%s: +%d -%d, want +%d -%d", w.path, f.Additions, f.Deletions, w.adds, w.dels)
		}
		if len(f.Hunks) != w.hunks {
			t.Errorf("%s: %d hunks, want %d", w.path, len(f.Hunks), w.hunks)
		}
		if f.Binary != w.binary {
			t.Errorf("%s: binary = %v, want %v", w.path, f.Binary, w.binary)
		}
		if got := [2]string{f.OldMode, f.NewMode}; got != w.modes {
			t.Errorf("%s: modes = %q, want %q", w.path, got, w.modes)
		}
	}
}

func TestParseLineNumbersAndSections(t *testing.T) {
	data, err := os.ReadFile("testdata/mixed.diff")
	if err != nil {
		t.Fatal(err)
	}
	f := byPath(t, mustParse(t, string(data)), "greek.txt")

	second := f.Hunks[1]
	if second.OldStart != 16 || second.OldLines != 5 || second.NewStart != 16 || second.NewLines != 6 {
		t.Errorf("second hunk range = -%d,%d +%d,%d", second.OldStart, second.OldLines, second.NewStart, second.NewLines)
	}
	if second.Section != "omicron" {
		t.Errorf("section = %q, want %q", second.Section, "omicron")
	}
	if got, want := second.Header(), "@@ -16,5 +16,6 @@ omicron"; got != want {
		t.Errorf("Header() = %q, want %q", got, want)
	}

	type line struct {
		kind     LineKind
		old, new int
		text     string
	}
	want := []line{
		{LineContext, 16, 16, "pi"},
		{LineContext, 17, 17, "rho"},
		{LineContext, 18, 18, "sigma"},
		{LineRemoved, 19, 0, "tau"},
		{LineAdded, 0, 19, "TAU"},
		{LineContext, 20, 20, "upsilon"},
		{LineAdded, 0, 21, "phi"},
	}
	if len(second.Lines) != len(want) {
		t.Fatalf("got %d lines, want %d", len(second.Lines), len(want))
	}
	for i, w := range want {
		l := second.Lines[i]
		if got := (line{l.Kind, l.Old, l.New, l.Text}); got != w {
			t.Errorf("line %d = %+v, want %+v", i, got, w)
		}
	}
}

func TestParseNoNewlineMarker(t *testing.T) {
	data, err := os.ReadFile("testdata/mixed.diff")
	if err != nil {
		t.Fatal(err)
	}
	f := byPath(t, mustParse(t, string(data)), "nonl.txt")
	lines := f.Hunks[0].Lines
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2 (the marker is not a line)", len(lines))
	}
	for i, l := range lines {
		if !l.NoNewline {
			t.Errorf("line %d: NoNewline not set", i)
		}
	}
}

func TestParseKeepsCarriageReturnsInContent(t *testing.T) {
	data, err := os.ReadFile("testdata/mixed.diff")
	if err != nil {
		t.Fatal(err)
	}
	f := byPath(t, mustParse(t, string(data)), "crlf.txt")
	if got := f.Hunks[0].Lines[1].Text; got != "second\r" {
		t.Errorf("removed line = %q, want %q", got, "second\r")
	}
}

// Content that looks like diff syntax must stay content: the hunk's line
// counts decide where it ends.
func TestParseDiffInsideDiff(t *testing.T) {
	data, err := os.ReadFile("testdata/mixed.diff")
	if err != nil {
		t.Fatal(err)
	}
	files := mustParse(t, string(data))
	f := byPath(t, files, "tricky.txt")
	lines := f.Hunks[0].Lines
	if len(lines) != 6 {
		t.Fatalf("got %d lines, want 6", len(lines))
	}
	if lines[0].Text != "diff --git a/x b/x" || lines[3].Text != "@@ -1 +1 @@" || lines[5].Text != "+new" {
		t.Errorf("content lines were misread: %q", []string{lines[0].Text, lines[3].Text, lines[5].Text})
	}
	if files[len(files)-1] != f {
		t.Errorf("content was split into extra files")
	}
}

func TestParseQuotedPaths(t *testing.T) {
	// GitHub quotes paths with non-ASCII bytes using octal escapes.
	text := strings.Join([]string{
		`diff --git "a/docs/na\303\257ve \"quoted\".md" "b/docs/na\303\257ve \"quoted\".md"`,
		`index 1111111..2222222 100644`,
		`--- "a/docs/na\303\257ve \"quoted\".md"`,
		`+++ "b/docs/na\303\257ve \"quoted\".md"`,
		`@@ -1 +1 @@`,
		`-a`,
		`+b`,
		`diff --git "a/bin/\346\227\245\346\234\254.dat" "b/bin/\346\227\245\346\234\254.dat"`,
		`index 1111111..2222222 100644`,
		`Binary files "a/bin/\346\227\245\346\234\254.dat" and "b/bin/\346\227\245\346\234\254.dat" differ`,
		``,
	}, "\n")
	files := mustParse(t, text)
	if len(files) != 2 {
		t.Fatalf("got %d files, want 2", len(files))
	}
	if got, want := files[0].Path(), `docs/naïve "quoted".md`; got != want {
		t.Errorf("path = %q, want %q", got, want)
	}
	if got, want := files[1].Path(), "bin/日本.dat"; got != want {
		t.Errorf("binary path = %q, want %q", got, want)
	}
	if !files[1].Binary {
		t.Errorf("binary file not marked as binary")
	}
}

func TestParseHeaderOnlyPathsWithSpaces(t *testing.T) {
	// Mode-only changes have no "---" or "+++" lines, so the paths come from
	// the "diff --git" line alone.
	text := "diff --git a/my scripts/run b.sh b/my scripts/run b.sh\nold mode 100644\nnew mode 100755\n"
	files := mustParse(t, text)
	if len(files) != 1 {
		t.Fatalf("got %d files, want 1", len(files))
	}
	if got, want := files[0].Path(), "my scripts/run b.sh"; got != want {
		t.Errorf("path = %q, want %q", got, want)
	}
}

func TestParseGitBinaryPatch(t *testing.T) {
	text := strings.Join([]string{
		"diff --git a/a.bin b/a.bin",
		"index 1111111..2222222 100644",
		"GIT binary patch",
		"literal 12",
		"TcmZQzU|?imU|?hbf&l>l",
		"",
		"literal 6",
		"NcmZQzU|?im009O700IC2",
		"",
		"diff --git a/b.txt b/b.txt",
		"index 1111111..2222222 100644",
		"--- a/b.txt",
		"+++ b/b.txt",
		"@@ -1 +1 @@",
		"-x",
		"+y",
		"",
	}, "\n")
	files := mustParse(t, text)
	if len(files) != 2 {
		t.Fatalf("got %d files, want 2", len(files))
	}
	if !files[0].Binary || len(files[0].Hunks) != 0 {
		t.Errorf("a.bin: binary = %v, hunks = %d", files[0].Binary, len(files[0].Hunks))
	}
	if files[1].Additions != 1 || files[1].Deletions != 1 {
		t.Errorf("b.txt: +%d -%d, want +1 -1", files[1].Additions, files[1].Deletions)
	}
}

func TestParseContextLineWithStrippedSpace(t *testing.T) {
	// Some tools strip the single space that marks an empty context line.
	text := "diff --git a/f b/f\n--- a/f\n+++ b/f\n@@ -1,3 +1,3 @@\n a\n\n-b\n+c\n"
	files := mustParse(t, text)
	lines := files[0].Hunks[0].Lines
	if len(lines) != 4 || lines[1].Kind != LineContext || lines[1].Text != "" {
		t.Fatalf("lines = %+v", lines)
	}
	if lines[3].New != 3 {
		t.Errorf("added line number = %d, want 3", lines[3].New)
	}
}

func TestParseMalformedHunkHeader(t *testing.T) {
	for _, header := range []string{"@@ -1,2 @@", "@@ -a,2 +1,2 @@", "@@ 1,2 +1,2 @@", "@@ -1,2 +1,2"} {
		text := "diff --git a/f b/f\n--- a/f\n+++ b/f\n" + header + "\n x\n"
		if _, err := Parse(text); err == nil {
			t.Errorf("Parse accepted hunk header %q", header)
		}
	}
}

func TestParseEmptyInput(t *testing.T) {
	for _, text := range []string{"", "\n", "not a diff\n"} {
		files, err := Parse(text)
		if err != nil || len(files) != 0 {
			t.Errorf("Parse(%q) = %d files, %v", text, len(files), err)
		}
	}
}

func TestParsePatch(t *testing.T) {
	patch := "@@ -1,3 +1,4 @@ func main() {\n a\n-b\n+B\n+B2\n c\n@@ -10 +11 @@\n-x\n\\ No newline at end of file\n+y"
	hunks, adds, dels, err := ParsePatch(patch)
	if err != nil {
		t.Fatal(err)
	}
	if len(hunks) != 2 || adds != 3 || dels != 2 {
		t.Fatalf("got %d hunks, +%d -%d; want 2 hunks, +3 -2", len(hunks), adds, dels)
	}
	if hunks[0].Section != "func main() {" {
		t.Errorf("section = %q", hunks[0].Section)
	}
	if hunks[1].OldLines != 1 || hunks[1].NewStart != 11 {
		t.Errorf("second hunk = %+v", hunks[1])
	}
	if !hunks[1].Lines[0].NoNewline || hunks[1].Lines[1].NoNewline {
		t.Errorf("NoNewline flags = %v, %v; want true, false", hunks[1].Lines[0].NoNewline, hunks[1].Lines[1].NoNewline)
	}
}
