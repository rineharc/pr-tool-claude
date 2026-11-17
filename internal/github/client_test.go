package github

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/rineharc/pr-tool-claude/internal/diff"
	"github.com/rineharc/pr-tool-claude/internal/model"
)

// fakeGitHub answers API requests from a table and records what it was asked.
type fakeGitHub struct {
	t *testing.T
	// routes maps "METHOD path?query" to an answer.
	routes   map[string]answer
	requests []*http.Request
	bodies   []string
}

type answer struct {
	status      int
	contentType string
	body        string
	err         error
}

func (f *fakeGitHub) RoundTrip(req *http.Request) (*http.Response, error) {
	body := ""
	if req.Body != nil {
		data, _ := io.ReadAll(req.Body)
		req.Body.Close()
		body = string(data)
	}
	f.requests = append(f.requests, req)
	f.bodies = append(f.bodies, body)

	key := req.Method + " " + req.URL.Path
	if req.URL.RawQuery != "" {
		key += "?" + req.URL.RawQuery
	}
	a, ok := f.routes[key]
	if !ok {
		f.t.Errorf("unexpected request: %s", key)
		a = answer{status: http.StatusNotFound, body: `{"message":"Not Found"}`}
	}
	if a.err != nil {
		return nil, a.err
	}
	if a.contentType == "" {
		a.contentType = "application/json; charset=utf-8"
	}
	return &http.Response{
		StatusCode: a.status,
		Header:     http.Header{"Content-Type": []string{a.contentType}},
		Body:       io.NopCloser(strings.NewReader(a.body)),
		Request:    req,
	}, nil
}

func newTestClient(t *testing.T, routes map[string]answer) (*Client, *fakeGitHub) {
	t.Helper()
	fake := &fakeGitHub{t: t, routes: routes}
	client, err := New(Options{Host: "github.com", Token: "secret", TokenSource: "the GH_TOKEN environment variable", Transport: fake})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return client, fake
}

var testRef = Ref{Host: "github.com", Owner: "acme", Repo: "payments-api", Number: 482}

const (
	graphqlRoute = "POST /graphql"
	diffRoute    = "GET /repos/acme/payments-api/pulls/482"
)

func filesRoute(page int) string {
	return fmt.Sprintf("GET /repos/acme/payments-api/pulls/482/files?per_page=100&page=%d", page)
}

func pullRequestJSON(overrides map[string]any) string {
	pr := map[string]any{
		"number":              482,
		"title":               "Retry idempotent charge requests on gateway timeouts",
		"url":                 "https://github.com/acme/payments-api/pull/482",
		"state":               "OPEN",
		"isDraft":             false,
		"author":              map[string]any{"login": "mira-okafor"},
		"baseRefName":         "main",
		"headRefName":         "feature/retry-timeouts",
		"isCrossRepository":   false,
		"headRepositoryOwner": map[string]any{"login": "acme"},
		"updatedAt":           "2026-11-12T09:14:00Z",
		"additions":           3,
		"deletions":           1,
		"changedFiles":        2,
		"commits":             map[string]any{"totalCount": 6},
	}
	for k, v := range overrides {
		pr[k] = v
	}
	data, _ := json.Marshal(map[string]any{"data": map[string]any{"repository": map[string]any{"pullRequest": pr}}})
	return string(data)
}

const smallDiff = `diff --git a/go.mod b/go.mod
index 1111111..2222222 100644
--- a/go.mod
+++ b/go.mod
@@ -3,3 +3,4 @@ module github.com/acme/payments-api
 go 1.27
 
-require github.com/go-chi/chi/v5 v5.2.2
+require github.com/go-chi/chi/v5 v5.2.3
+require golang.org/x/sync v0.24.0
diff --git a/docs/notes.md b/docs/notes.md
new file mode 100644
index 0000000..3333333
--- /dev/null
+++ b/docs/notes.md
@@ -0,0 +1 @@
+Retries are on.
`

func TestPullRequest(t *testing.T) {
	client, fake := newTestClient(t, map[string]answer{
		graphqlRoute: {status: 200, body: pullRequestJSON(nil)},
		diffRoute:    {status: 200, contentType: "text/plain; charset=utf-8", body: smallDiff},
	})

	pr, err := client.PullRequest(context.Background(), testRef)
	if err != nil {
		t.Fatalf("PullRequest() error = %v", err)
	}

	want := model.PullRequest{
		Host:         "github.com",
		Owner:        "acme",
		Repo:         "payments-api",
		Number:       482,
		Title:        "Retry idempotent charge requests on gateway timeouts",
		URL:          "https://github.com/acme/payments-api/pull/482",
		Author:       "mira-okafor",
		State:        model.StateOpen,
		BaseRef:      "main",
		HeadRef:      "feature/retry-timeouts",
		Commits:      6,
		ChangedFiles: 2,
		Additions:    3,
		Deletions:    1,
		UpdatedAt:    time.Date(2026, time.November, 12, 9, 14, 0, 0, time.UTC),
	}
	got := *pr
	got.Files = nil
	if !reflect.DeepEqual(got, want) {
		t.Errorf("PullRequest() =\n %+v\nwant\n %+v", got, want)
	}

	if len(pr.Files) != 2 {
		t.Fatalf("got %d files, want 2", len(pr.Files))
	}
	if f := pr.Files[0]; f.Path() != "go.mod" || f.Status != diff.Modified || f.Additions != 2 || f.Deletions != 1 {
		t.Errorf("first file = %s %s +%d -%d", f.Path(), f.Status, f.Additions, f.Deletions)
	}
	if f := pr.Files[1]; f.Path() != "docs/notes.md" || f.Status != diff.Added || f.Additions != 1 {
		t.Errorf("second file = %s %s +%d", f.Path(), f.Status, f.Additions)
	}

	if len(fake.requests) != 2 {
		t.Fatalf("made %d requests, want 2", len(fake.requests))
	}
	query, diffReq := fake.requests[0], fake.requests[1]
	if query.URL.String() != "https://api.github.com/graphql" {
		t.Errorf("query went to %s", query.URL)
	}
	if diffReq.URL.String() != "https://api.github.com/repos/acme/payments-api/pulls/482" {
		t.Errorf("diff request went to %s", diffReq.URL)
	}
	if got := diffReq.Header.Get("Accept"); got != "application/vnd.github.diff" {
		t.Errorf("diff request Accept = %q", got)
	}
	for _, req := range fake.requests {
		if got := req.Header.Get("Authorization"); got != "token secret" {
			t.Errorf("%s %s: Authorization = %q", req.Method, req.URL.Path, got)
		}
	}

	var sent struct {
		Query     string
		Variables map[string]any
	}
	if err := json.Unmarshal([]byte(fake.bodies[0]), &sent); err != nil {
		t.Fatalf("query body is not JSON: %v", err)
	}
	if sent.Query != pullRequestQuery {
		t.Errorf("sent a query other than the embedded one")
	}
	if sent.Variables["owner"] != "acme" || sent.Variables["repo"] != "payments-api" || sent.Variables["number"] != float64(482) {
		t.Errorf("variables = %v", sent.Variables)
	}
}

func TestPullRequestStates(t *testing.T) {
	tests := []struct {
		name      string
		overrides map[string]any
		wantState model.State
		wantLabel string
		wantHead  string
		wantBy    string
	}{
		{
			name:      "draft",
			overrides: map[string]any{"isDraft": true},
			wantState: model.StateOpen,
			wantLabel: "Draft",
			wantHead:  "feature/retry-timeouts",
			wantBy:    "mira-okafor",
		},
		{
			name:      "merged",
			overrides: map[string]any{"state": "MERGED"},
			wantState: model.StateMerged,
			wantLabel: "Merged",
			wantHead:  "feature/retry-timeouts",
			wantBy:    "mira-okafor",
		},
		{
			name:      "closed",
			overrides: map[string]any{"state": "CLOSED"},
			wantState: model.StateClosed,
			wantLabel: "Closed",
			wantHead:  "feature/retry-timeouts",
			wantBy:    "mira-okafor",
		},
		{
			name:      "from a fork",
			overrides: map[string]any{"isCrossRepository": true, "headRepositoryOwner": map[string]any{"login": "octo-fork"}},
			wantState: model.StateOpen,
			wantLabel: "Open",
			wantHead:  "octo-fork:feature/retry-timeouts",
			wantBy:    "mira-okafor",
		},
		{
			name:      "from a deleted fork and a deleted account",
			overrides: map[string]any{"isCrossRepository": true, "headRepositoryOwner": nil, "author": nil},
			wantState: model.StateOpen,
			wantLabel: "Open",
			wantHead:  "feature/retry-timeouts",
			wantBy:    "ghost",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, _ := newTestClient(t, map[string]answer{
				graphqlRoute: {status: 200, body: pullRequestJSON(tt.overrides)},
				diffRoute:    {status: 200, contentType: "text/plain", body: smallDiff},
			})

			pr, err := client.PullRequest(context.Background(), testRef)
			if err != nil {
				t.Fatalf("PullRequest() error = %v", err)
			}
			if pr.State != tt.wantState || pr.StatusLabel() != tt.wantLabel || pr.HeadRef != tt.wantHead || pr.Author != tt.wantBy {
				t.Errorf("got state %v (%s), head %q, author %q", pr.State, pr.StatusLabel(), pr.HeadRef, pr.Author)
			}
		})
	}
}

func TestPullRequestWithoutChanges(t *testing.T) {
	client, _ := newTestClient(t, map[string]answer{
		graphqlRoute: {status: 200, body: pullRequestJSON(map[string]any{"changedFiles": 0})},
		diffRoute:    {status: 200, contentType: "text/plain", body: ""},
	})

	pr, err := client.PullRequest(context.Background(), testRef)
	if err != nil {
		t.Fatalf("PullRequest() error = %v", err)
	}
	if len(pr.Files) != 0 {
		t.Errorf("got %d files, want none", len(pr.Files))
	}
}

const tooLargeBody = `{"message":"Sorry, the diff exceeded the maximum number of files (300). Consider using 'List pull requests files' API or locally cloning the repository instead.","errors":[{"resource":"PullRequest","field":"diff","code":"too_large"}],"documentation_url":"https://docs.github.com/rest/pulls/pulls#list-pull-requests-files","status":"406"}`

func listing(files ...map[string]any) string {
	data, _ := json.Marshal(files)
	return string(data)
}

func TestPullRequestFallsBackToTheFileList(t *testing.T) {
	var firstPage []map[string]any
	for i := 0; i < 100; i++ {
		firstPage = append(firstPage, map[string]any{
			"filename":  fmt.Sprintf("gen/file%03d.txt", i),
			"status":    "modified",
			"additions": 1,
			"deletions": 1,
			"patch":     "@@ -1 +1 @@\n-old\n+new",
		})
	}
	secondPage := []map[string]any{
		{"filename": "added.txt", "status": "added", "additions": 2, "deletions": 0, "patch": "@@ -0,0 +1,2 @@\n+one\n+two"},
		{"filename": "removed.txt", "status": "removed", "additions": 0, "deletions": 1, "patch": "@@ -1 +0,0 @@\n-gone"},
		{"filename": "new/name.txt", "previous_filename": "old/name.txt", "status": "renamed", "additions": 0, "deletions": 0},
		{"filename": "copy.txt", "previous_filename": "original.txt", "status": "copied", "additions": 1, "deletions": 0, "patch": "@@ -1,0 +2 @@\n+more"},
		{"filename": "huge.sql", "status": "modified", "additions": 30000, "deletions": 12},
		{"filename": "logo.png", "status": "modified", "additions": 0, "deletions": 0},
	}

	client, fake := newTestClient(t, map[string]answer{
		graphqlRoute:  {status: 200, body: pullRequestJSON(map[string]any{"changedFiles": 106})},
		diffRoute:     {status: 406, body: tooLargeBody},
		filesRoute(1): {status: 200, body: listing(firstPage...)},
		filesRoute(2): {status: 200, body: listing(secondPage...)},
	})

	pr, err := client.PullRequest(context.Background(), testRef)
	if err != nil {
		t.Fatalf("PullRequest() error = %v", err)
	}
	if len(pr.Files) != 106 {
		t.Fatalf("got %d files, want 106", len(pr.Files))
	}
	if pr.Notice != "" {
		t.Errorf("Notice = %q, want none when every file is listed", pr.Notice)
	}
	if len(fake.requests) != 4 {
		t.Errorf("made %d requests, want 4", len(fake.requests))
	}
	if got := fake.requests[2].Header.Get("Accept"); got != "application/vnd.github+json" {
		t.Errorf("file list Accept = %q", got)
	}

	first := pr.Files[0]
	if first.Path() != "gen/file000.txt" || first.Status != diff.Modified || len(first.Hunks) != 1 || len(first.Hunks[0].Lines) != 2 {
		t.Errorf("first file = %+v", first)
	}

	byPath := map[string]*diff.File{}
	for _, f := range pr.Files {
		byPath[f.Path()] = f
	}
	if f := byPath["added.txt"]; f == nil || f.Status != diff.Added || f.OldPath != "" || f.Additions != 2 {
		t.Errorf("added.txt = %+v", f)
	}
	if f := byPath["removed.txt"]; f == nil || f.Status != diff.Deleted || f.NewPath != "" || f.Deletions != 1 {
		t.Errorf("removed.txt = %+v", f)
	}
	if f := byPath["new/name.txt"]; f == nil || f.Status != diff.Renamed || f.OldPath != "old/name.txt" || f.Note != "" || f.Missing {
		t.Errorf("new/name.txt = %+v", f)
	}
	if f := byPath["copy.txt"]; f == nil || f.Status != diff.Copied || f.OldPath != "original.txt" || len(f.Hunks) != 1 {
		t.Errorf("copy.txt = %+v", f)
	}
	if f := byPath["huge.sql"]; f == nil || !f.Missing || f.Additions != 30000 || f.Deletions != 12 {
		t.Errorf("huge.sql = %+v", f)
	}
	if f := byPath["logo.png"]; f == nil || f.Missing || f.Note == "" {
		t.Errorf("logo.png = %+v", f)
	}
}

func TestPullRequestSaysWhenTheFileListIsCut(t *testing.T) {
	page := make([]map[string]any, 100)
	for i := range page {
		page[i] = map[string]any{"filename": fmt.Sprintf("f%d", i), "status": "added", "additions": 1, "deletions": 0, "patch": "@@ -0,0 +1 @@\n+x"}
	}
	routes := map[string]answer{
		graphqlRoute: {status: 200, body: pullRequestJSON(map[string]any{"changedFiles": 4200})},
		diffRoute:    {status: 406, body: tooLargeBody},
	}
	for p := 1; p <= 30; p++ {
		routes[filesRoute(p)] = answer{status: 200, body: listing(page...)}
	}
	client, fake := newTestClient(t, routes)

	pr, err := client.PullRequest(context.Background(), testRef)
	if err != nil {
		t.Fatalf("PullRequest() error = %v", err)
	}
	if len(pr.Files) != 3000 {
		t.Errorf("got %d files, want 3000", len(pr.Files))
	}
	if want := "Showing 3000 of 4200 files: GitHub lists at most 3000 files of a pull request."; pr.Notice != want {
		t.Errorf("Notice = %q, want %q", pr.Notice, want)
	}
	if len(fake.requests) != 32 {
		t.Errorf("made %d requests, want 32", len(fake.requests))
	}
}

func TestPullRequestErrors(t *testing.T) {
	notFoundBody := `{"data":{"repository":null},"errors":[{"type":"NOT_FOUND","path":["repository"],"message":"Could not resolve to a Repository with the name 'acme/payments-api'."}]}`
	noPullBody := `{"data":{"repository":{"pullRequest":null}},"errors":[{"type":"NOT_FOUND","path":["repository","pullRequest"],"message":"Could not resolve to a PullRequest with the number of 482."}]}`
	nullPullBody := `{"data":{"repository":{"pullRequest":null}}}`
	forbiddenBody := `{"data":{"repository":null},"errors":[{"type":"FORBIDDEN","path":["repository"],"message":"Resource protected by organization SAML enforcement."}]}`

	tests := []struct {
		name   string
		routes map[string]answer
		want   string
	}{
		{
			name:   "repository not found",
			routes: map[string]answer{graphqlRoute: {status: 200, body: notFoundBody}},
			want:   "could not find pull request acme/payments-api #482: check the name and number, and that your token can read the repository",
		},
		{
			name:   "pull request not found",
			routes: map[string]answer{graphqlRoute: {status: 200, body: noPullBody}},
			want:   "could not find pull request acme/payments-api #482",
		},
		{
			name:   "pull request missing without an error",
			routes: map[string]answer{graphqlRoute: {status: 200, body: nullPullBody}},
			want:   "could not find pull request acme/payments-api #482",
		},
		{
			name:   "bad token",
			routes: map[string]answer{graphqlRoute: {status: 401, body: `{"message":"Bad credentials"}`}},
			want:   "GitHub rejected the token from the GH_TOKEN environment variable",
		},
		{
			name:   "access refused",
			routes: map[string]answer{graphqlRoute: {status: 200, body: forbiddenBody}},
			want:   "Resource protected by organization SAML enforcement",
		},
		{
			name:   "rate limited",
			routes: map[string]answer{graphqlRoute: {status: 403, body: `{"message":"API rate limit exceeded for user ID 1."}`}},
			want:   "GitHub refused the request for acme/payments-api #482: API rate limit exceeded for user ID 1.",
		},
		{
			name:   "server error",
			routes: map[string]answer{graphqlRoute: {status: 502, body: `{"message":"Bad Gateway"}`}},
			want:   "GitHub answered the request for acme/payments-api #482 with an error",
		},
		{
			name:   "network down",
			routes: map[string]answer{graphqlRoute: {err: errors.New("dial tcp: no such host")}},
			want:   "could not reach github.com",
		},
		{
			name: "diff not found",
			routes: map[string]answer{
				graphqlRoute: {status: 200, body: pullRequestJSON(nil)},
				diffRoute:    {status: 404, body: `{"message":"Not Found"}`},
			},
			want: "could not find pull request acme/payments-api #482",
		},
		{
			name: "diff that cannot be parsed",
			routes: map[string]answer{
				graphqlRoute: {status: 200, body: pullRequestJSON(nil)},
				diffRoute:    {status: 200, contentType: "text/plain", body: "diff --git a/x b/x\n--- a/x\n+++ b/x\n@@ nonsense @@\n"},
			},
			want: "understand the diff of acme/payments-api #482",
		},
		{
			name: "file list fails",
			routes: map[string]answer{
				graphqlRoute:  {status: 200, body: pullRequestJSON(nil)},
				diffRoute:     {status: 406, body: tooLargeBody},
				filesRoute(1): {status: 500, body: `{"message":"Server Error"}`},
			},
			want: "GitHub answered the request for acme/payments-api #482 with an error",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, _ := newTestClient(t, tt.routes)

			pr, err := client.PullRequest(context.Background(), testRef)

			if err == nil {
				t.Fatalf("PullRequest() = %+v, want an error", pr)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("PullRequest() error = %q, want it to contain %q", err, tt.want)
			}
			if strings.Contains(err.Error(), "secret") {
				t.Errorf("error leaks the token: %q", err)
			}
		})
	}
}

func TestPullRequestStopsWhenCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	client, _ := newTestClient(t, map[string]answer{graphqlRoute: {err: context.Canceled}})

	_, err := client.PullRequest(ctx, testRef)

	if !errors.Is(err, context.Canceled) {
		t.Errorf("PullRequest() error = %v, want context.Canceled", err)
	}
}

func TestEnterpriseHostUsesItsOwnAPI(t *testing.T) {
	fake := &fakeGitHub{t: t, routes: map[string]answer{
		"POST /api/graphql":                             {status: 200, body: pullRequestJSON(nil)},
		"GET /api/v3/repos/acme/payments-api/pulls/482": {status: 200, contentType: "text/plain", body: smallDiff},
	}}
	client, err := New(Options{Host: "github.example.com", Token: "secret", Transport: fake})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ref := testRef
	ref.Host = "github.example.com"
	pr, err := client.PullRequest(context.Background(), ref)
	if err != nil {
		t.Fatalf("PullRequest() error = %v", err)
	}
	if pr.Host != "github.example.com" {
		t.Errorf("Host = %q", pr.Host)
	}
	for _, req := range fake.requests {
		if req.URL.Host != "github.example.com" {
			t.Errorf("request went to %s", req.URL.Host)
		}
	}
}

func TestNewNeedsHostAndToken(t *testing.T) {
	if _, err := New(Options{Token: "secret"}); err == nil {
		t.Error("New() without a host succeeded")
	}
	if _, err := New(Options{Host: "github.com"}); err == nil {
		t.Error("New() without a token succeeded")
	}
}

func TestEmbeddedQueryIsOneDocument(t *testing.T) {
	if !strings.HasPrefix(pullRequestQuery, "query PullRequest(") {
		t.Errorf("query starts with %q", pullRequestQuery[:20])
	}
	if open, closed := bytes.Count([]byte(pullRequestQuery), []byte("{")), bytes.Count([]byte(pullRequestQuery), []byte("}")); open != closed {
		t.Errorf("query has %d opening and %d closing braces", open, closed)
	}
}
