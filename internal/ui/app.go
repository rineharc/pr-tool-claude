// Package ui is the root of the terminal user interface: it loads the pull
// request, draws the header and footer around the Files tab, and owns the
// keys that work everywhere.
package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"

	"github.com/rineharc/pr-tool-claude/internal/model"
	"github.com/rineharc/pr-tool-claude/internal/ui/files"
	"github.com/rineharc/pr-tool-claude/internal/ui/keymap"
	"github.com/rineharc/pr-tool-claude/internal/ui/style"
)

const (
	minWidth  = 60
	minHeight = 12
)

// Loader fetches the pull request to show. The user interface does not know
// where it comes from.
type Loader func(ctx context.Context) (*model.PullRequest, error)

// Options configures the user interface.
type Options struct {
	// Title names what is being loaded, such as "acme/api #482".
	Title string
	Load  Loader
	// Now is the clock used for relative times. It defaults to time.Now.
	Now func() time.Time
}

type loadedMsg struct{ pr *model.PullRequest }

type failedMsg struct{ err error }

// Model is the root Bubble Tea model.
type Model struct {
	title  string
	load   Loader
	now    func() time.Time
	ctx    context.Context
	cancel context.CancelFunc

	dark    bool
	profile colorprofile.Profile
	styles  style.Styles
	width   int
	height  int

	pr    *model.PullRequest
	files *files.Model
	err   error
	help  bool
}

// New returns the root model. Pass it to tea.NewProgram.
func New(opts Options) *Model {
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	ctx, cancel := context.WithCancel(context.Background())
	m := &Model{
		title:   opts.Title,
		load:    opts.Load,
		now:     now,
		ctx:     ctx,
		cancel:  cancel,
		dark:    true,
		profile: colorprofile.TrueColor,
	}
	m.restyle()
	return m
}

// Err returns the error that ended the program, if loading failed.
func (m *Model) Err() error {
	return m.err
}

// Init starts loading the pull request and asks the terminal whether its
// background is dark or light.
func (m *Model) Init() tea.Cmd {
	return tea.Batch(tea.RequestBackgroundColor, func() tea.Msg {
		pr, err := m.load(m.ctx)
		if err != nil {
			return failedMsg{err}
		}
		return loadedMsg{pr}
	})
}

// Update handles one message.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.resize()

	case tea.BackgroundColorMsg:
		m.dark = msg.IsDark()
		m.restyle()

	case tea.ColorProfileMsg:
		m.profile = msg.Profile
		m.restyle()

	case loadedMsg:
		m.pr = msg.pr
		m.files = files.New(msg.pr.Files, m.styles)
		m.resize()

	case failedMsg:
		m.err = msg.err
		return m, m.quit()

	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, keymap.ForceQuit):
			return m, m.quit()
		case m.help:
			if key.Matches(msg, keymap.Help, keymap.Close, keymap.Quit) {
				m.help = false
			}
		case key.Matches(msg, keymap.Quit):
			return m, m.quit()
		case key.Matches(msg, keymap.Help):
			m.help = true
		case m.files != nil:
			m.files.Update(msg)
		}
	}
	return m, nil
}

func (m *Model) quit() tea.Cmd {
	m.cancel()
	return tea.Quit
}

func (m *Model) restyle() {
	m.styles = style.New(m.dark, m.profile)
	if m.files != nil {
		m.files.SetStyles(m.styles)
	}
}

func (m *Model) resize() {
	if m.files != nil {
		m.files.SetSize(m.width, m.bodyHeight())
	}
}

// bodyHeight is the room left for the tab between the two header lines and
// the footer, less one line when there is a notice to show.
func (m *Model) bodyHeight() int {
	height := m.height - 3
	if m.pr != nil && m.pr.Notice != "" {
		height--
	}
	return max(0, height)
}

// View draws the whole screen.
func (m *Model) View() tea.View {
	view := tea.NewView(m.render())
	view.AltScreen = true
	view.WindowTitle = "prt · " + m.title
	return view
}

func (m *Model) render() string {
	switch {
	case m.width == 0 || m.height == 0:
		return ""
	case m.width < minWidth || m.height < minHeight:
		return m.message(fmt.Sprintf("prt needs a window of at least %d by %d. This one is %d by %d.", minWidth, minHeight, m.width, m.height))
	case m.pr == nil:
		return m.message("Loading " + m.title + "…")
	}

	lines := m.header()
	lines = append(lines, strings.Split(m.files.View(), "\n")...)
	if m.pr.Notice != "" {
		lines = append(lines, style.Fit(" "+m.styles.Notice.Render(m.pr.Notice), m.width))
	}
	lines = append(lines, m.footer())
	screen := strings.Join(lines, "\n")

	if m.help {
		return m.overlay(screen, m.helpBox())
	}
	return screen
}

// message fills the screen with one centered line.
func (m *Model) message(text string) string {
	lines := make([]string, m.height)
	width := ansi.StringWidth(text)
	if width > m.width {
		text, width = ansi.Truncate(text, m.width, "…"), m.width
	}
	lines[m.height/2] = strings.Repeat(" ", (m.width-width)/2) + m.styles.Dim.Render(text)
	return strings.Join(lines, "\n")
}

func (m *Model) header() []string {
	st, pr := m.styles, m.pr

	state := st.StateOpen
	switch {
	case pr.State == model.StateMerged:
		state = st.StateMerged
	case pr.State == model.StateClosed:
		state = st.StateClosed
	case pr.Draft:
		state = st.StateDraft
	}
	first := style.Spread(" "+st.Bold.Render(pr.Name())+"   "+pr.Title, "  "+state.Render(pr.StatusLabel())+" ", m.width)

	verb := "wants to merge"
	switch pr.State {
	case model.StateMerged:
		verb = "merged"
	case model.StateClosed:
		verb = "wanted to merge"
	}
	commits := "commits"
	if pr.Commits == 1 {
		commits = "commit"
	}
	second := fmt.Sprintf(" %s %s %d %s into %s from %s · updated %s", pr.Author, verb, pr.Commits, commits, pr.BaseRef, pr.HeadRef, ago(m.now(), pr.UpdatedAt))
	return []string{first, st.Dim.Render(style.Fit(second, m.width))}
}

// footer lists the keys for the focused pane. Help and quit are always
// there; the others are dropped from the end until the line fits.
func (m *Model) footer() string {
	st := m.styles
	always := []files.Hint{{Key: "?", Action: "help"}, {Key: "q", Action: "quit"}}
	hints := m.files.Hints()
	for {
		var parts []string
		for _, h := range append(hints[:len(hints):len(hints)], always...) {
			parts = append(parts, st.Key.Render(h.Key)+" "+st.Dim.Render(h.Action))
		}
		line := " " + strings.Join(parts, st.Dim.Render(" · "))
		if len(hints) == 0 || ansi.StringWidth(line) <= m.width {
			return style.Fit(line, m.width)
		}
		hints = hints[:len(hints)-1]
	}
}

func (m *Model) helpBox() string {
	st := m.styles
	sections := keymap.HelpSections()

	keyWidth := 0
	for _, s := range sections {
		for _, e := range s.Entries {
			keyWidth = max(keyWidth, ansi.StringWidth(e.Keys))
		}
	}

	var rows []string
	for i, s := range sections {
		if i > 0 {
			rows = append(rows, "")
		}
		rows = append(rows, st.Accent.Bold(true).Render(s.Title))
		for _, e := range s.Entries {
			rows = append(rows, st.Key.Render(style.Fit(e.Keys, keyWidth))+"   "+e.Action)
		}
	}
	rows = append(rows, "", st.Dim.Render("? or esc closes this help"))

	// The box never grows beyond the screen; rows that do not fit are cut.
	if limit := m.height - 2; len(rows) > limit {
		rows = rows[:max(0, limit)]
	}
	inner := 0
	for _, r := range rows {
		inner = max(inner, ansi.StringWidth(r))
	}
	inner = min(inner+4, m.width-2)

	border := st.BorderFocused
	title := " Keys "
	lines := []string{border.Render("┌─") + st.PaneTitle.Render(title) + border.Render(strings.Repeat("─", max(0, inner-1-ansi.StringWidth(title)))+"┐")}
	for _, r := range rows {
		lines = append(lines, border.Render("│")+style.Fit("  "+r, inner)+border.Render("│"))
	}
	lines = append(lines, border.Render("└"+strings.Repeat("─", inner)+"┘"))
	return strings.Join(lines, "\n")
}

// overlay draws box over the middle of screen.
func (m *Model) overlay(screen, box string) string {
	x := max(0, (m.width-lipgloss.Width(box))/2)
	y := max(0, (m.height-lipgloss.Height(box))/2)
	composed := lipgloss.NewCompositor(
		lipgloss.NewLayer(screen),
		lipgloss.NewLayer(box).X(x).Y(y).Z(1),
	).Render()

	// Compositing drops the spaces at the end of a line; put them back so
	// that every line still spans the terminal.
	lines := strings.Split(composed, "\n")
	for i, line := range lines {
		lines[i] = style.Fit(line, m.width)
	}
	return strings.Join(lines, "\n")
}

// ago says how long before now t was, the way GitHub does.
func ago(now, t time.Time) string {
	d := now.Sub(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	default:
		return "on " + t.Format("Jan 2, 2006")
	}
}
