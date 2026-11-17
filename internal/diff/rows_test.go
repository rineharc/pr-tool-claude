package diff

import (
	"os"
	"testing"
)

func TestRowsLayout(t *testing.T) {
	text := "diff --git a/f.go b/f.go\n--- a/f.go\n+++ b/f.go\n" +
		"@@ -10,3 +10,3 @@ func f() {\n" +
		" \tkeep\n" +
		"-\told := 1\n" +
		"+\tnewValue := compute(first, second, third, fourth)\n" +
		" }\n"
	f := mustParse(t, text)[0]

	rows := Rows(f, 30)
	type row struct {
		kind     RowKind
		old, new int
		text     string
		cont     bool
	}
	want := []row{
		{RowHunk, 0, 0, "@@ -10,3 +10,3 @@ func f() {", false},
		{RowContext, 10, 10, "    keep", false},
		{RowRemoved, 11, 0, "    old := 1", false},
		{RowAdded, 0, 11, "    newValue := compute(first,", false},
		{RowAdded, 0, 0, "    ↪ second, third, fourth)", true},
		{RowContext, 12, 12, "}", false},
	}
	if len(rows) != len(want) {
		t.Fatalf("got %d rows, want %d: %+v", len(rows), len(want), rows)
	}
	for i, w := range want {
		r := rows[i]
		if got := (row{r.Kind, r.Old, r.New, r.Text, r.Continuation}); got != w {
			t.Errorf("row %d = %+v, want %+v", i, got, w)
		}
		if r.Hunk != 0 {
			t.Errorf("row %d: hunk = %d, want 0", i, r.Hunk)
		}
	}
}

func TestRowsNotes(t *testing.T) {
	data, err := os.ReadFile("testdata/mixed.diff")
	if err != nil {
		t.Fatal(err)
	}
	files := mustParse(t, string(data))

	notes := map[string]string{
		"logo.png":         "Binary file. There is no text diff to show.",
		"pure renamed.txt": "File renamed without changes.",
		"run.sh":           "File mode changed from 100644 to 100755.",
		"empty.txt":        "Empty file added.",
	}
	for path, want := range notes {
		rows := Rows(byPath(t, files, path), 80)
		if len(rows) != 1 || rows[0].Kind != RowNote || rows[0].Text != want {
			t.Errorf("%s: rows = %+v, want one note %q", path, rows, want)
		}
	}

	rows := Rows(byPath(t, files, "nonl.txt"), 80)
	var kinds []RowKind
	for _, r := range rows {
		kinds = append(kinds, r.Kind)
	}
	wantKinds := []RowKind{RowHunk, RowRemoved, RowNote, RowAdded, RowNote}
	if len(kinds) != len(wantKinds) {
		t.Fatalf("nonl.txt: kinds = %v, want %v", kinds, wantKinds)
	}
	for i := range kinds {
		if kinds[i] != wantKinds[i] {
			t.Fatalf("nonl.txt: kinds = %v, want %v", kinds, wantKinds)
		}
	}

	missing := &File{NewPath: "big.json", Missing: true, Additions: 9000}
	if rows := Rows(missing, 80); len(rows) != 1 || rows[0].Kind != RowNote {
		t.Errorf("missing diff: rows = %+v", rows)
	}
}

func TestRowsCarriageReturnsAreHidden(t *testing.T) {
	data, err := os.ReadFile("testdata/mixed.diff")
	if err != nil {
		t.Fatal(err)
	}
	rows := Rows(byPath(t, mustParse(t, string(data)), "crlf.txt"), 80)
	for _, r := range rows {
		for _, c := range r.Text {
			if c == '\r' {
				t.Errorf("row %q still contains a carriage return", r.Text)
			}
		}
	}
}

func TestMaxLineNumber(t *testing.T) {
	f := &File{Hunks: []Hunk{{OldStart: 1, OldLines: 5, NewStart: 1, NewLines: 6}, {OldStart: 9990, OldLines: 8, NewStart: 9991, NewLines: 12}}}
	if got := MaxLineNumber(f); got != 10003 {
		t.Errorf("MaxLineNumber = %d, want 10003", got)
	}
}
