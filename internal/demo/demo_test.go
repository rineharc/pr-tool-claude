package demo

import (
	"context"
	"testing"

	"github.com/rineharc/pr-tool-claude/internal/diff"
	"github.com/rineharc/pr-tool-claude/internal/github"
	"github.com/rineharc/pr-tool-claude/internal/model"
)

func TestBundledPullRequest(t *testing.T) {
	client, err := github.New(github.Options{Host: Ref.Host, Token: Token, Transport: Transport()})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	pr, err := client.PullRequest(context.Background(), Ref)
	if err != nil {
		t.Fatalf("PullRequest() error = %v", err)
	}

	if pr.Name() != "acme/payments-api #482" || pr.Author != "mira-okafor" || pr.State != model.StateOpen {
		t.Errorf("got %s by %s in state %v", pr.Name(), pr.Author, pr.State)
	}
	if pr.BaseRef != "main" || pr.HeadRef != "feature/retry-timeouts" || pr.Commits != 6 {
		t.Errorf("got %d commits from %s into %s", pr.Commits, pr.HeadRef, pr.BaseRef)
	}
	if got := Now().Sub(pr.UpdatedAt).Hours(); got != 2 {
		t.Errorf("Now() is %v hours after the last update, want 2", got)
	}

	// The description of the pull request and its diff are separate
	// fixtures; they have to agree.
	type counts struct {
		status    diff.Status
		additions int
		deletions int
	}
	want := map[string]counts{
		"docs/runbooks/gateway-504.md":    {diff.Added, 38, 0},
		"go.mod":                          {diff.Modified, 1, 0},
		"go.sum":                          {diff.Modified, 2, 0},
		"internal/charge/handler.go":      {diff.Modified, 9, 3},
		"internal/charge/idempotency.go":  {diff.Modified, 21, 6},
		"internal/config/config.go":       {diff.Modified, 6, 0},
		"internal/gateway/client.go":      {diff.Modified, 48, 12},
		"internal/gateway/client_test.go": {diff.Modified, 96, 4},
		"internal/gateway/retry.go":       {diff.Added, 74, 0},
		"internal/gateway/retry_test.go":  {diff.Added, 112, 0},
	}
	if len(pr.Files) != len(want) || pr.ChangedFiles != len(want) {
		t.Fatalf("got %d files and a count of %d, want %d", len(pr.Files), pr.ChangedFiles, len(want))
	}
	additions, deletions := 0, 0
	for _, f := range pr.Files {
		w, ok := want[f.Path()]
		if !ok {
			t.Errorf("unexpected file %s", f.Path())
			continue
		}
		if f.Status != w.status || f.Additions != w.additions || f.Deletions != w.deletions {
			t.Errorf("%s: got %s +%d -%d, want %s +%d -%d", f.Path(), f.Status, f.Additions, f.Deletions, w.status, w.additions, w.deletions)
		}
		additions += f.Additions
		deletions += f.Deletions
	}
	if additions != pr.Additions || deletions != pr.Deletions {
		t.Errorf("files add up to +%d -%d, the pull request says +%d -%d", additions, deletions, pr.Additions, pr.Deletions)
	}
}

func TestOtherRequestsAreNotFound(t *testing.T) {
	client, err := github.New(github.Options{Host: Ref.Host, Token: Token, Transport: Transport()})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// The bundled description answers any GraphQL query, so the miss shows
	// up when the diff of the other pull request is requested.
	other := Ref
	other.Number = 1
	if _, err := client.PullRequest(context.Background(), other); err == nil {
		t.Fatal("PullRequest() returned a pull request that is not bundled")
	}
}
