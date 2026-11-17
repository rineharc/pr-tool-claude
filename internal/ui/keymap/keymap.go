// Package keymap is the one place that says which key does what. The help
// overlay is drawn from it, so the two cannot drift apart.
package keymap

import "charm.land/bubbles/v2/key"

var (
	Down         = bind("Down", "j", "down")
	Up           = bind("Up", "k", "up")
	Top          = bind("Top", "g", "home")
	Bottom       = bind("Bottom", "G", "end")
	HalfPageDown = bind("Half a page down", "ctrl+d")
	HalfPageUp   = bind("Half a page up", "ctrl+u")
	PageDown     = bind("Page down", "pgdown")
	PageUp       = bind("Page up", "pgup")

	NextPane = bind("Next pane", "tab")
	PrevPane = bind("Previous pane", "shift+tab")
	NextFile = bind("Next file", "]")
	PrevFile = bind("Previous file", "[")
	NextHunk = bind("Next hunk", "}")
	PrevHunk = bind("Previous hunk", "{")

	Open     = bind("Open file or expand folder", "enter", "l", "right")
	Collapse = bind("Collapse folder", "h", "left")
	Tree     = bind("Show or hide the file tree", "t")

	Help  = bind("Help", "?")
	Close = bind("Close", "esc")
	Quit  = bind("Quit", "q")
	// ForceQuit works everywhere, including while something else has the
	// keyboard.
	ForceQuit = bind("Quit", "ctrl+c")
)

func bind(help string, keys ...string) key.Binding {
	return key.NewBinding(key.WithKeys(keys...), key.WithHelp(keys[0], help))
}

// Entry is one line of the help overlay.
type Entry struct {
	Keys   string
	Action string
}

// Section is a titled group of help entries.
type Section struct {
	Title   string
	Entries []Entry
}

// HelpSections lists every binding the way the help overlay shows it.
func HelpSections() []Section {
	return []Section{
		{Title: "Move", Entries: []Entry{
			{"j / k, ↓ / ↑", "Down / up"},
			{"g / G, home / end", "Top / bottom"},
			{"ctrl+d / ctrl+u", "Half a page down / up"},
			{"pgdn / pgup", "Page down / up"},
		}},
		{Title: "Files", Entries: []Entry{
			{"[ / ]", "Previous / next file"},
			{"{ / }", "Previous / next hunk"},
			{"tab / shift+tab", "Switch between the tree and the diff"},
			{"enter or l / h", "Tree: open file or expand folder / collapse"},
			{"t", "Show / hide the file tree"},
		}},
		{Title: "General", Entries: []Entry{
			{"?", "Show / hide this help"},
			{"q, ctrl+c", "Quit"},
		}},
	}
}
