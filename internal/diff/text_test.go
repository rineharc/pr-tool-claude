package diff

import (
	"reflect"
	"strings"
	"testing"
)

func TestDisplay(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plain", "return nil", "return nil"},
		{"leading tab", "\treturn nil", "    return nil"},
		{"tab to next stop", "ab\tc", "ab  c"},
		{"tab after wide text", "日本\tx", "日本    x"},
		{"trailing carriage return", "line\r", "line"},
		{"inner carriage return", "a\rb", "a·b"},
		{"escape sequence", "\x1b[31mred\x1b[0m", "·[31mred·[0m"},
		{"c1 control", "a\u009bb", "a·b"},
		{"delete", "a\x7fb", "a·b"},
		{"invalid utf-8", "a\xffb", "a\uFFFDb"},
		{"non-ascii untouched", "naïve café", "naïve café"},
	}
	for _, tt := range tests {
		if got := Display(tt.in, 4); got != tt.want {
			t.Errorf("%s: Display(%q) = %q, want %q", tt.name, tt.in, got, tt.want)
		}
	}
}

func TestWrap(t *testing.T) {
	tests := []struct {
		name  string
		in    string
		width int
		want  []string
	}{
		{"fits", "short line", 20, []string{"short line"}},
		{"exact fit", "12345678", 8, []string{"12345678"}},
		{"empty", "", 10, []string{""}},
		{
			"breaks at a space and keeps the indent",
			`        return nil, fmt.Errorf("gateway charge after %d attempts: %w", c.retry.Attempts(), err)`,
			64,
			[]string{
				`        return nil, fmt.Errorf("gateway charge after %d`,
				`        ↪ attempts: %w", c.retry.Attempts(), err)`,
			},
		},
		{
			"hard break without spaces",
			"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			12,
			[]string{"aaaaaaaaaaaa", "↪ aaaaaaaaaa", "↪ aaaaaaaa"},
		},
		{
			"does not break inside the indent",
			"            x := veryLongIdentifierNameThatGoesOn",
			24,
			[]string{"            x := veryLon", "            ↪ gIdentifie", "            ↪ rNameThatG", "            ↪ oesOn"},
		},
		{
			"drops the indent when it leaves no room",
			"                    deeply indented text here",
			24,
			[]string{"                    deep", "↪ ly indented text here"},
		},
		{
			"wide characters are not split",
			"日本語のテキストです",
			7,
			[]string{"日本語", "↪ のテ", "↪ キス", "↪ トで", "↪ す"},
		},
	}
	for _, tt := range tests {
		got := Wrap(tt.in, tt.width)
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%s:\n got  %q\n want %q", tt.name, got, tt.want)
		}
		for _, row := range got {
			if w := Width(row); w > tt.width {
				t.Errorf("%s: row %q is %d columns, limit %d", tt.name, row, w, tt.width)
			}
		}
	}
}

// Wrapping must never lose or reorder visible characters.
func TestWrapKeepsContent(t *testing.T) {
	inputs := []string{
		`    if resp.StatusCode == http.StatusGatewayTimeout { return retryable(ErrGatewayTimeout) }`,
		"no-spaces-at-all-in-this-very-long-token-which-must-be-cut-somewhere-anyway",
		"mixed 日本語 and ascii words in one fairly long line of text for wrapping",
		strings.Repeat("word ", 40),
	}
	for _, in := range inputs {
		for width := 1; width <= 40; width++ {
			var b strings.Builder
			for i, row := range Wrap(in, width) {
				if i > 0 {
					row = strings.TrimLeft(row, " ")
					row = strings.TrimPrefix(row, ContinuationMarker)
				}
				b.WriteString(row)
			}
			if got, want := strings.ReplaceAll(b.String(), " ", ""), strings.ReplaceAll(in, " ", ""); got != want {
				t.Fatalf("width %d: content changed\n got  %q\n want %q", width, got, want)
			}
		}
	}
}
