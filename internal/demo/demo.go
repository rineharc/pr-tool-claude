// Package demo serves a bundled pull request without touching the network.
// It answers the same two requests the GitHub client makes, so demo mode
// exercises the real client, parser and user interface.
package demo

import (
	"bytes"
	_ "embed"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/rineharc/pr-tool-claude/internal/github"
)

//go:embed fixture/pull_request.json
var pullRequestJSON []byte

//go:embed fixture/pull_request.diff
var pullRequestDiff []byte

// Ref names the bundled pull request.
var Ref = github.Ref{Host: "github.com", Owner: "acme", Repo: "payments-api", Number: 482}

// Token is a placeholder; nothing checks it.
const Token = "demo"

// Now is the time demo mode pretends it is: two hours after the bundled pull
// request was last updated, so that screens are the same on every run.
func Now() time.Time {
	return time.Date(2026, time.November, 12, 11, 14, 0, 0, time.UTC)
}

// Transport answers GitHub API requests for the bundled pull request and
// reports everything else as not found.
func Transport() http.RoundTripper {
	return transport{}
}

type transport struct{}

func (transport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Body != nil {
		_, _ = io.Copy(io.Discard, req.Body)
		req.Body.Close()
	}

	switch {
	case req.Method == http.MethodPost && strings.HasSuffix(req.URL.Path, "/graphql"):
		return response(req, http.StatusOK, "application/json; charset=utf-8", pullRequestJSON), nil
	case req.Method == http.MethodGet && strings.HasSuffix(req.URL.Path, "/repos/acme/payments-api/pulls/482"):
		return response(req, http.StatusOK, "text/plain; charset=utf-8", pullRequestDiff), nil
	default:
		return response(req, http.StatusNotFound, "application/json; charset=utf-8", []byte(`{"message":"Not Found"}`)), nil
	}
}

func response(req *http.Request, status int, contentType string, body []byte) *http.Response {
	return &http.Response{
		StatusCode:    status,
		Status:        http.StatusText(status),
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Header:        http.Header{"Content-Type": []string{contentType}},
		Body:          io.NopCloser(bytes.NewReader(body)),
		ContentLength: int64(len(body)),
		Request:       req,
	}
}
