// Package github is the only package that knows GitHub's wire format. It
// fetches a pull request and turns it into the types in package model.
package github

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/cli/go-gh/v2/pkg/api"

	"github.com/rineharc/pr-tool-claude/internal/diff"
	"github.com/rineharc/pr-tool-claude/internal/model"
)

//go:embed queries/pull_request.graphql
var pullRequestQuery string

const (
	diffMediaType = "application/vnd.github.diff"
	jsonMediaType = "application/vnd.github+json"

	filesPerPage = 100
	// GitHub lists at most this many files of a pull request.
	maxListedFiles = 3000
)

// Options configures a Client.
type Options struct {
	// Host is the GitHub host, such as "github.com".
	Host string
	// Token authenticates every request.
	Token string
	// TokenSource says in words where Token came from. It is used in the
	// message shown when GitHub rejects the token.
	TokenSource string
	// Transport replaces the network. Tests and demo mode set it.
	Transport http.RoundTripper
}

// Client reads pull requests from one GitHub host.
type Client struct {
	host        string
	tokenSource string
	graphql     *api.GraphQLClient
	rest        *api.RESTClient
	restDiff    *api.RESTClient
}

// New returns a Client for the host in opts.
func New(opts Options) (*Client, error) {
	if opts.Host == "" {
		return nil, errors.New("no GitHub host given")
	}
	if opts.Token == "" {
		return nil, fmt.Errorf("no GitHub token given for %s", opts.Host)
	}

	// The media type is fixed per client because go-gh sets headers on the
	// client, not on a request.
	clientOptions := func(accept string) api.ClientOptions {
		return api.ClientOptions{
			Host:      opts.Host,
			AuthToken: opts.Token,
			Headers:   map[string]string{"Accept": accept, "User-Agent": "prt"},
			Timeout:   30 * time.Second,
			Transport: opts.Transport,
		}
	}

	graphql, err := api.NewGraphQLClient(clientOptions(jsonMediaType))
	if err != nil {
		return nil, err
	}
	rest, err := api.NewRESTClient(clientOptions(jsonMediaType))
	if err != nil {
		return nil, err
	}
	restDiff, err := api.NewRESTClient(clientOptions(diffMediaType))
	if err != nil {
		return nil, err
	}
	return &Client{host: opts.Host, tokenSource: opts.TokenSource, graphql: graphql, rest: rest, restDiff: restDiff}, nil
}

type pullRequestData struct {
	Repository struct {
		PullRequest *struct {
			Number int    `json:"number"`
			Title  string `json:"title"`
			URL    string `json:"url"`
			State  string `json:"state"`
			Draft  bool   `json:"isDraft"`
			Author *struct {
				Login string `json:"login"`
			} `json:"author"`
			BaseRefName         string `json:"baseRefName"`
			HeadRefName         string `json:"headRefName"`
			IsCrossRepository   bool   `json:"isCrossRepository"`
			HeadRepositoryOwner *struct {
				Login string `json:"login"`
			} `json:"headRepositoryOwner"`
			UpdatedAt    time.Time `json:"updatedAt"`
			Additions    int       `json:"additions"`
			Deletions    int       `json:"deletions"`
			ChangedFiles int       `json:"changedFiles"`
			Commits      struct {
				TotalCount int `json:"totalCount"`
			} `json:"commits"`
		} `json:"pullRequest"`
	} `json:"repository"`
}

// PullRequest fetches a pull request and the diff of its files. It makes two
// calls: one GraphQL query for the description of the pull request and one
// REST request for its diff. When GitHub refuses to render the diff because
// it is too large, the files are listed page by page instead.
func (c *Client) PullRequest(ctx context.Context, ref Ref) (*model.PullRequest, error) {
	var data pullRequestData
	variables := map[string]interface{}{"owner": ref.Owner, "repo": ref.Repo, "number": ref.Number}
	if err := c.graphql.DoWithContext(ctx, pullRequestQuery, variables, &data); err != nil {
		return nil, c.explain(err, ref)
	}
	node := data.Repository.PullRequest
	if node == nil {
		return nil, notFound(ref)
	}

	pr := &model.PullRequest{
		Host:         c.host,
		Owner:        ref.Owner,
		Repo:         ref.Repo,
		Number:       node.Number,
		Title:        node.Title,
		URL:          node.URL,
		Author:       "ghost",
		Draft:        node.Draft,
		BaseRef:      node.BaseRefName,
		HeadRef:      node.HeadRefName,
		Commits:      node.Commits.TotalCount,
		ChangedFiles: node.ChangedFiles,
		Additions:    node.Additions,
		Deletions:    node.Deletions,
		UpdatedAt:    node.UpdatedAt,
	}
	if node.Author != nil {
		pr.Author = node.Author.Login
	}
	if node.IsCrossRepository && node.HeadRepositoryOwner != nil {
		pr.HeadRef = node.HeadRepositoryOwner.Login + ":" + node.HeadRefName
	}
	switch node.State {
	case "MERGED":
		pr.State = model.StateMerged
	case "CLOSED":
		pr.State = model.StateClosed
	default:
		pr.State = model.StateOpen
	}

	files, err := c.diff(ctx, ref)
	if errors.Is(err, errDiffTooLarge) {
		files, err = c.listFiles(ctx, ref)
		if err == nil && pr.ChangedFiles > len(files) {
			pr.Notice = fmt.Sprintf("Showing %d of %d files: GitHub lists at most %d files of a pull request.", len(files), pr.ChangedFiles, maxListedFiles)
		}
	}
	if err != nil {
		return nil, err
	}
	pr.Files = files
	return pr, nil
}

// errDiffTooLarge reports that GitHub would not render the diff in one piece,
// which it refuses above 300 files or 20,000 lines.
var errDiffTooLarge = errors.New("the diff is too large for GitHub to render at once")

func (c *Client) diff(ctx context.Context, ref Ref) ([]*diff.File, error) {
	resp, err := c.restDiff.RequestWithContext(ctx, http.MethodGet, pullPath(ref), nil)
	if err != nil {
		var httpErr *api.HTTPError
		if errors.As(err, &httpErr) && httpErr.StatusCode == http.StatusNotAcceptable {
			return nil, errDiffTooLarge
		}
		return nil, c.explain(err, ref)
	}
	defer resp.Body.Close()

	text, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read the diff of %s: %w", ref, err)
	}
	files, err := diff.Parse(string(text))
	if err != nil {
		return nil, fmt.Errorf("understand the diff of %s: %w", ref, err)
	}
	return files, nil
}

type listedFile struct {
	Filename         string  `json:"filename"`
	PreviousFilename string  `json:"previous_filename"`
	Status           string  `json:"status"`
	Additions        int     `json:"additions"`
	Deletions        int     `json:"deletions"`
	Patch            *string `json:"patch"`
}

func (c *Client) listFiles(ctx context.Context, ref Ref) ([]*diff.File, error) {
	var files []*diff.File
	for page := 1; len(files) < maxListedFiles; page++ {
		path := fmt.Sprintf("%s/files?per_page=%d&page=%d", pullPath(ref), filesPerPage, page)
		resp, err := c.rest.RequestWithContext(ctx, http.MethodGet, path, nil)
		if err != nil {
			return nil, c.explain(err, ref)
		}
		var listed []listedFile
		err = json.NewDecoder(resp.Body).Decode(&listed)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("read the file list of %s: %w", ref, err)
		}

		for _, l := range listed {
			f, err := fileFromListing(l)
			if err != nil {
				return nil, fmt.Errorf("understand the diff of %s in %s: %w", l.Filename, ref, err)
			}
			files = append(files, f)
		}
		if len(listed) < filesPerPage {
			break
		}
	}
	return files, nil
}

func fileFromListing(l listedFile) (*diff.File, error) {
	f := &diff.File{OldPath: l.Filename, NewPath: l.Filename, Additions: l.Additions, Deletions: l.Deletions}
	switch l.Status {
	case "added":
		f.Status = diff.Added
		f.OldPath = ""
	case "removed":
		f.Status = diff.Deleted
		f.NewPath = ""
	case "renamed":
		f.Status = diff.Renamed
		f.OldPath = l.PreviousFilename
	case "copied":
		f.Status = diff.Copied
		f.OldPath = l.PreviousFilename
	}

	switch {
	case l.Patch != nil:
		hunks, _, _, err := diff.ParsePatch(*l.Patch)
		if err != nil {
			return nil, err
		}
		f.Hunks = hunks
	case l.Additions+l.Deletions > 0:
		f.Missing = true
	case f.Status != diff.Renamed && f.Status != diff.Copied:
		// The listing does not say why there is no patch.
		f.Note = "No text diff: the file is binary, or only its mode changed."
	}
	return f, nil
}

func pullPath(ref Ref) string {
	return fmt.Sprintf("repos/%s/%s/pulls/%d", ref.Owner, ref.Repo, ref.Number)
}

func notFound(ref Ref) error {
	return fmt.Errorf("could not find pull request %s: check the name and number, and that your token can read the repository", ref)
}

// explain turns an API error into a message the user can act on.
func (c *Client) explain(err error, ref Ref) error {
	var gqlErr *api.GraphQLError
	if errors.As(err, &gqlErr) {
		for _, item := range gqlErr.Errors {
			if item.Type == "NOT_FOUND" {
				return notFound(ref)
			}
		}
		return fmt.Errorf("GitHub could not answer the request for %s: %w", ref, err)
	}

	var httpErr *api.HTTPError
	if errors.As(err, &httpErr) {
		switch httpErr.StatusCode {
		case http.StatusUnauthorized:
			source := c.tokenSource
			if source == "" {
				source = "your configuration"
			}
			return fmt.Errorf("GitHub rejected the token from %s: sign in again with `gh auth login`, or set a valid GH_TOKEN", source)
		case http.StatusNotFound:
			return notFound(ref)
		case http.StatusForbidden, http.StatusTooManyRequests:
			return fmt.Errorf("GitHub refused the request for %s: %s", ref, httpErr.Message)
		}
		return fmt.Errorf("GitHub answered the request for %s with an error: %w", ref, err)
	}

	if errors.Is(err, context.Canceled) {
		return err
	}
	return fmt.Errorf("could not reach %s: %w", c.host, err)
}
