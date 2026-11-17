// Package model holds the domain types the user interface works with. It has
// no knowledge of GitHub's wire format.
package model

import (
	"fmt"
	"time"

	"github.com/rineharc/pr-tool-claude/internal/diff"
)

// State is the lifecycle state of a pull request.
type State int

const (
	StateOpen State = iota
	StateClosed
	StateMerged
)

// PullRequest is one pull request together with the diff of its files.
type PullRequest struct {
	Host   string
	Owner  string
	Repo   string
	Number int

	Title  string
	URL    string
	Author string
	State  State
	Draft  bool

	// BaseRef is the branch the pull request merges into. HeadRef is the
	// branch it comes from, prefixed with "owner:" when that branch lives in
	// a fork.
	BaseRef string
	HeadRef string

	Commits      int
	ChangedFiles int
	Additions    int
	Deletions    int
	UpdatedAt    time.Time

	Files []*diff.File

	// Notice is a message for the reader when Files is not the full picture,
	// for example because GitHub limits how many files it lists.
	Notice string
}

// Name is the short name of the pull request, such as "acme/api #482".
func (pr *PullRequest) Name() string {
	return fmt.Sprintf("%s/%s #%d", pr.Owner, pr.Repo, pr.Number)
}

// StatusLabel is the word shown for the pull request's state.
func (pr *PullRequest) StatusLabel() string {
	switch {
	case pr.State == StateMerged:
		return "Merged"
	case pr.State == StateClosed:
		return "Closed"
	case pr.Draft:
		return "Draft"
	default:
		return "Open"
	}
}
