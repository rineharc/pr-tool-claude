// Package style holds the colors and text styles of the user interface, in a
// dark and a light variant.
package style

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"

	"github.com/rineharc/pr-tool-claude/internal/diff"
)

// Styles is the full set of styles for one background.
type Styles struct {
	Text   lipgloss.Style
	Bold   lipgloss.Style
	Dim    lipgloss.Style
	Accent lipgloss.Style
	Key    lipgloss.Style

	Border        lipgloss.Style
	BorderFocused lipgloss.Style
	PaneTitle     lipgloss.Style

	StateOpen   lipgloss.Style
	StateDraft  lipgloss.Style
	StateMerged lipgloss.Style
	StateClosed lipgloss.Style
	Notice      lipgloss.Style

	Added    lipgloss.Style
	Removed  lipgloss.Style
	Modified lipgloss.Style
	Moved    lipgloss.Style

	// Selected marks the tree row under the cursor while the tree has the
	// keyboard; Current marks it while the diff has it.
	Selected lipgloss.Style
	Current  lipgloss.Style

	CursorBar     lipgloss.Style
	LineNumber    lipgloss.Style
	Hunk          lipgloss.Style
	Note          lipgloss.Style
	AddedGutter   lipgloss.Style
	AddedCode     lipgloss.Style
	RemovedGutter lipgloss.Style
	RemovedCode   lipgloss.Style
}

// New returns the styles for a dark or a light terminal background and for
// the number of colors the terminal can show.
func New(dark bool, profile colorprofile.Profile) Styles {
	pick := lipgloss.LightDark(dark)
	text := func(light, darkHex string) lipgloss.Style {
		return lipgloss.NewStyle().Foreground(pick(lipgloss.Color(light), lipgloss.Color(darkHex)))
	}
	// A tint is a faint background. Reduced to 256 colors automatically, a
	// faint tint turns gray, so those are chosen by hand; with fewer colors
	// there is no tint and the signs and text colors carry the meaning.
	complete := lipgloss.Complete(profile)
	tint := func(light256, lightHex, dark256, darkHex string) color.Color {
		return complete(
			lipgloss.NoColor{},
			pick(lipgloss.Color(light256), lipgloss.Color(dark256)),
			pick(lipgloss.Color(lightHex), lipgloss.Color(darkHex)),
		)
	}

	var (
		dim    = text("#57606a", "#8b949e")
		accent = text("#0969da", "#58a6ff")
		green  = text("#1a7f37", "#3fb950")
		red    = text("#cf222e", "#f85149")
		amber  = text("#9a6700", "#d29922")
		purple = text("#8250df", "#a371f7")

		addedBg    = tint("194", "#e6ffec", "22", "#12261e")
		removedBg  = tint("224", "#ffebe9", "52", "#301b1f")
		hunkBg     = tint("195", "#ddf4ff", "17", "#121d2f")
		selectedBg = complete(lipgloss.Color("4"), lipgloss.Color("26"), pick(lipgloss.Color("#0969da"), lipgloss.Color("#1f6feb")))
	)

	return Styles{
		Text:   lipgloss.NewStyle(),
		Bold:   lipgloss.NewStyle().Bold(true),
		Dim:    dim,
		Accent: accent,
		Key:    lipgloss.NewStyle().Bold(true),

		Border:        dim,
		BorderFocused: accent,
		PaneTitle:     lipgloss.NewStyle().Bold(true),

		StateOpen:   green.Bold(true),
		StateDraft:  dim.Bold(true),
		StateMerged: purple.Bold(true),
		StateClosed: red.Bold(true),
		Notice:      amber,

		Added:    green,
		Removed:  red,
		Modified: amber,
		Moved:    accent,

		Selected: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#ffffff")).Background(selectedBg),
		Current:  lipgloss.NewStyle().Bold(true),

		CursorBar:     accent,
		LineNumber:    dim,
		Hunk:          accent.Background(hunkBg),
		Note:          dim.Italic(true),
		AddedGutter:   green.Background(addedBg),
		AddedCode:     lipgloss.NewStyle().Background(addedBg),
		RemovedGutter: red.Background(removedBg),
		RemovedCode:   lipgloss.NewStyle().Background(removedBg),
	}
}

// Status returns the style for a file's change type.
func (s Styles) Status(status diff.Status) lipgloss.Style {
	switch status {
	case diff.Added:
		return s.Added
	case diff.Deleted:
		return s.Removed
	case diff.Renamed, diff.Copied:
		return s.Moved
	default:
		return s.Modified
	}
}

// Fit makes s exactly width columns wide: it pads with spaces, or cuts and
// ends with an ellipsis. Styling inside s is kept.
func Fit(s string, width int) string {
	if width <= 0 {
		return ""
	}
	w := ansi.StringWidth(s)
	if w > width {
		s = ansi.Truncate(s, width, "…")
		w = ansi.StringWidth(s)
	}
	if w < width {
		s += strings.Repeat(" ", width-w)
	}
	return s
}

// FitLeft is Fit, but cuts from the start, which keeps the end of a long
// path readable.
func FitLeft(s string, width int) string {
	if width <= 0 {
		return ""
	}
	w := ansi.StringWidth(s)
	if w <= width {
		return s + strings.Repeat(" ", width-w)
	}
	s = "…" + ansi.TruncateLeft(s, w-width+1, "")
	if w = ansi.StringWidth(s); w < width {
		s += strings.Repeat(" ", width-w)
	}
	return s
}

// Spread puts left at the start and right at the end of a line of width
// columns. When both do not fit, left is cut.
func Spread(left, right string, width int) string {
	rw := ansi.StringWidth(right)
	if rw >= width {
		return Fit(right, width)
	}
	return Fit(left, width-rw) + right
}
