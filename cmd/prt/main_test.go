package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/rineharc/pr-tool-claude/internal/github"
)

// isolate hides the developer's own GitHub setup from a test: no tokens, an
// empty gh configuration, no gh executable, and no terminal.
func isolate(t *testing.T) {
	t.Helper()
	for _, name := range []string{"GH_TOKEN", "GITHUB_TOKEN", "GH_ENTERPRISE_TOKEN", "GITHUB_ENTERPRISE_TOKEN", "GH_HOST", "GH_REPO", "GH_PATH"} {
		t.Setenv(name, "")
	}
	t.Setenv("GH_CONFIG_DIR", t.TempDir())
	t.Setenv("PATH", t.TempDir())
	t.Chdir(t.TempDir())

	previous := interactive
	interactive = func() bool { return false }
	t.Cleanup(func() { interactive = previous })
}

func TestParseArgs(t *testing.T) {
	tests := []struct {
		args string
		want config
	}{
		{"482", config{target: "482"}},
		{"#482", config{target: "#482"}},
		{"acme/api#482", config{target: "acme/api#482"}},
		{"https://github.com/acme/api/pull/482", config{target: "https://github.com/acme/api/pull/482"}},
		{"482 --repo acme/api", config{target: "482", repo: "acme/api"}},
		{"--repo acme/api 482", config{target: "482", repo: "acme/api"}},
		{"-R acme/api 482", config{target: "482", repo: "acme/api"}},
		{"482 --repo=acme/api", config{target: "482", repo: "acme/api"}},
		{"482 -R=github.example.com/acme/api", config{target: "482", repo: "github.example.com/acme/api"}},
		{"--demo", config{demo: true}},
		{"--help", config{help: true}},
		{"-h", config{help: true}},
		{"help", config{help: true}},
		{"--version", config{version: true}},
		{"version", config{version: true}},
		{"--help --nonsense-before-help-is-still-an-error", config{}},
		{"-3", config{target: "-3"}},
	}
	for _, tt := range tests {
		got, err := parseArgs(strings.Fields(tt.args))
		if tt.want == (config{}) {
			if err == nil {
				t.Errorf("parseArgs(%q) = %+v, want an error", tt.args, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseArgs(%q) error = %v", tt.args, err)
			continue
		}
		if got != tt.want {
			t.Errorf("parseArgs(%q) = %+v, want %+v", tt.args, got, tt.want)
		}
	}
}

func TestParseArgsRejects(t *testing.T) {
	tests := []struct {
		args string
		want string
	}{
		{"", "no pull request given"},
		{"--repo acme/api", "no pull request given"},
		{"482 483", `expected one pull request, got "482" and "483"`},
		{"482 --repo", "--repo needs a repository"},
		{"482 -R", "-R needs a repository"},
		{"482 --repo=", "--repo needs a repository"},
		{"482 --split", "unknown flag --split"},
		{"-x", "unknown flag -x"},
		{"--demo 482", "--demo shows a bundled pull request and takes no other arguments"},
		{"--demo --repo acme/api", "--demo shows a bundled pull request"},
	}
	for _, tt := range tests {
		_, err := parseArgs(strings.Fields(tt.args))
		if err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("parseArgs(%q) error = %v, want it to contain %q", tt.args, err, tt.want)
		}
	}
}

func TestResolve(t *testing.T) {
	tests := []struct {
		name string
		cfg  config
		env  map[string]string
		want github.Ref
	}{
		{
			name: "a URL names everything",
			cfg:  config{target: "https://github.example.com/acme/api/pull/482/files"},
			want: github.Ref{Host: "github.example.com", Owner: "acme", Repo: "api", Number: 482},
		},
		{
			name: "the short form gets the default host",
			cfg:  config{target: "acme/api#482"},
			want: github.Ref{Host: "github.com", Owner: "acme", Repo: "api", Number: 482},
		},
		{
			name: "the default host follows GH_HOST",
			cfg:  config{target: "acme/api#482"},
			env:  map[string]string{"GH_HOST": "github.example.com"},
			want: github.Ref{Host: "github.example.com", Owner: "acme", Repo: "api", Number: 482},
		},
		{
			name: "a number with --repo",
			cfg:  config{target: "482", repo: "acme/api"},
			want: github.Ref{Host: "github.com", Owner: "acme", Repo: "api", Number: 482},
		},
		{
			name: "a number with --repo on another host",
			cfg:  config{target: "#482", repo: "github.example.com/acme/api"},
			want: github.Ref{Host: "github.example.com", Owner: "acme", Repo: "api", Number: 482},
		},
		{
			name: "a number with --repo as a URL",
			cfg:  config{target: "482", repo: "https://github.com/acme/api"},
			want: github.Ref{Host: "github.com", Owner: "acme", Repo: "api", Number: 482},
		},
		{
			name: "a number in a directory whose repository GH_REPO names",
			cfg:  config{target: "482"},
			env:  map[string]string{"GH_REPO": "acme/api"},
			want: github.Ref{Host: "github.com", Owner: "acme", Repo: "api", Number: 482},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isolate(t)
			for name, value := range tt.env {
				t.Setenv(name, value)
			}

			got, err := resolve(tt.cfg)
			if err != nil {
				t.Fatalf("resolve() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("resolve() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestResolveErrors(t *testing.T) {
	tests := []struct {
		name string
		cfg  config
		want string
	}{
		{"not a pull request", config{target: "nonsense"}, `"nonsense" is not a pull request`},
		{"a negative number", config{target: "-3"}, `"-3" is not a pull request`},
		{"--repo with a full name", config{target: "acme/api#482", repo: "acme/other"}, "already names its repository; leave out --repo"},
		{"--repo that is not a repository", config{target: "482", repo: "acme"}, "--repo: expected the"},
		{"a number outside a repository", config{target: "482"}, "could not tell which repository #482 belongs to"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isolate(t)

			got, err := resolve(tt.cfg)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("resolve() = %+v, %v; want an error containing %q", got, err, tt.want)
			}
		})
	}
}

func TestRun(t *testing.T) {
	tests := []struct {
		name       string
		args       string
		env        map[string]string
		wantCode   int
		wantStdout string
		wantStderr string
	}{
		{name: "help", args: "--help", wantCode: 0, wantStdout: "Usage:\n  prt <pull request> [--repo OWNER/REPO]\n  prt --demo"},
		{name: "version", args: "--version", wantCode: 0, wantStdout: "prt "},
		{name: "no arguments", args: "", wantCode: 2, wantStderr: "prt: no pull request given\nRun `prt --help` for usage.\n"},
		{name: "unknown flag", args: "482 --split", wantCode: 2, wantStderr: "prt: unknown flag --split"},
		{name: "not a pull request", args: "nonsense", wantCode: 2, wantStderr: `prt: "nonsense" is not a pull request: give a number, owner/repo#number, or the pull request's URL`},
		{name: "no token", args: "acme/api#482", wantCode: 1, wantStderr: "prt: no GitHub token found for github.com: run `gh auth login`, or set GH_TOKEN\n"},
		{name: "number outside a repository", args: "482", wantCode: 1, wantStderr: "prt: could not tell which repository #482 belongs to"},
		{
			name:       "no terminal",
			args:       "acme/api#482",
			env:        map[string]string{"GH_TOKEN": "secret"},
			wantCode:   1,
			wantStderr: "prt: this is an interactive program and needs a terminal for its input and output\n",
		},
		{name: "demo without a terminal", args: "--demo", wantCode: 1, wantStderr: "needs a terminal"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isolate(t)
			for name, value := range tt.env {
				t.Setenv(name, value)
			}
			var stdout, stderr bytes.Buffer

			code := run(strings.Fields(tt.args), &stdout, &stderr)

			if code != tt.wantCode {
				t.Errorf("exit code = %d, want %d (stderr: %s)", code, tt.wantCode, stderr.String())
			}
			if !strings.Contains(stdout.String(), tt.wantStdout) {
				t.Errorf("stdout = %q, want it to contain %q", stdout.String(), tt.wantStdout)
			}
			if !strings.Contains(stderr.String(), tt.wantStderr) {
				t.Errorf("stderr = %q, want it to contain %q", stderr.String(), tt.wantStderr)
			}
			if tt.wantStderr == "" && stderr.Len() > 0 {
				t.Errorf("unexpected stderr: %q", stderr.String())
			}
			if strings.Contains(stdout.String()+stderr.String(), "secret") {
				t.Error("the output leaks the token")
			}
		})
	}
}

func TestDemoLoadsWithoutNetworkOrToken(t *testing.T) {
	isolate(t)

	opts, err := prepare(config{demo: true})
	if err != nil {
		t.Fatalf("prepare() error = %v", err)
	}
	if opts.Title != "acme/payments-api #482" {
		t.Errorf("Title = %q", opts.Title)
	}
	pr, err := opts.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(pr.Files) != 10 {
		t.Errorf("the demo pull request has %d files, want 10", len(pr.Files))
	}
	if got := opts.Now().Sub(pr.UpdatedAt).Hours(); got != 2 {
		t.Errorf("the demo clock is %v hours after the last update, want 2", got)
	}
}

func TestBuildVersion(t *testing.T) {
	previous := version
	t.Cleanup(func() { version = previous })

	version = "1.2.3"
	if got := buildVersion(); got != "1.2.3" {
		t.Errorf("buildVersion() = %q", got)
	}
	version = ""
	if got := buildVersion(); got == "" {
		t.Error("buildVersion() is empty without a release version")
	}
}
