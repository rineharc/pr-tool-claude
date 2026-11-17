// Command prt reviews GitHub pull requests in the terminal.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/term"
	"github.com/cli/go-gh/v2/pkg/repository"

	"github.com/rineharc/pr-tool-claude/internal/auth"
	"github.com/rineharc/pr-tool-claude/internal/demo"
	"github.com/rineharc/pr-tool-claude/internal/github"
	"github.com/rineharc/pr-tool-claude/internal/model"
	"github.com/rineharc/pr-tool-claude/internal/ui"
)

// version is set by the release build.
var version = ""

const usage = `prt shows a GitHub pull request in the terminal.

Usage:
  prt <pull request> [--repo OWNER/REPO]
  prt --demo

A pull request is one of:
  482                                     a number, in the repository of the current directory
  owner/repo#482                          a repository and a number
  https://github.com/owner/repo/pull/482  the pull request's URL

Flags:
  -R, --repo [HOST/]OWNER/REPO   the repository a bare number refers to
      --demo                     show a bundled pull request; needs no network and no token
      --version                  print the version
  -h, --help                     print this help

prt reads your GitHub token from GH_TOKEN or GITHUB_TOKEN, or from the GitHub
CLI when you are signed in there (gh auth login). Press ? inside prt for the keys.
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// interactive reports whether prt has a terminal to draw on and read from.
// Tests replace it.
var interactive = func() bool {
	return term.IsTerminal(os.Stdin.Fd()) && term.IsTerminal(os.Stdout.Fd())
}

type config struct {
	target  string
	repo    string
	demo    bool
	help    bool
	version bool
}

// usageError is a mistake on the command line, as opposed to a failure while
// doing what was asked.
type usageError struct{ msg string }

func (e usageError) Error() string { return e.msg }

func parseArgs(args []string) (config, error) {
	var cfg config
	for i := 0; i < len(args); i++ {
		arg := args[i]
		name, value, hasValue := strings.Cut(arg, "=")
		switch {
		case arg == "-h" || arg == "--help" || arg == "help":
			cfg.help = true
		case arg == "--version" || arg == "version":
			cfg.version = true
		case arg == "--demo":
			cfg.demo = true
		case name == "-R" || name == "--repo":
			if !hasValue {
				if i+1 >= len(args) {
					return cfg, usageError{name + " needs a repository, such as owner/repo"}
				}
				i++
				value = args[i]
			}
			if value == "" {
				return cfg, usageError{name + " needs a repository, such as owner/repo"}
			}
			cfg.repo = value
		case strings.HasPrefix(arg, "-") && !isNegativeNumber(arg):
			return cfg, usageError{"unknown flag " + arg}
		case cfg.target != "":
			return cfg, usageError{fmt.Sprintf("expected one pull request, got %q and %q", cfg.target, arg)}
		default:
			cfg.target = arg
		}
	}

	switch {
	case cfg.help || cfg.version:
		return cfg, nil
	case cfg.demo && (cfg.target != "" || cfg.repo != ""):
		return cfg, usageError{"--demo shows a bundled pull request and takes no other arguments"}
	case !cfg.demo && cfg.target == "":
		return cfg, usageError{"no pull request given"}
	}
	return cfg, nil
}

// isNegativeNumber lets "-3" reach the pull request parser, which explains
// what is wrong with it better than "unknown flag" would.
func isNegativeNumber(arg string) bool {
	return len(arg) > 1 && strings.Trim(arg[1:], "0123456789") == ""
}

func run(args []string, stdout, stderr io.Writer) int {
	cfg, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "prt: %v\nRun `prt --help` for usage.\n", err)
		return 2
	}
	if cfg.help {
		fmt.Fprint(stdout, usage)
		return 0
	}
	if cfg.version {
		fmt.Fprintf(stdout, "prt %s\n", buildVersion())
		return 0
	}

	opts, err := prepare(cfg)
	if err != nil {
		fmt.Fprintf(stderr, "prt: %v\n", err)
		if errors.As(err, &usageError{}) {
			return 2
		}
		return 1
	}
	if !interactive() {
		fmt.Fprintln(stderr, "prt: this is an interactive program and needs a terminal for its input and output")
		return 1
	}

	app := ui.New(opts)
	if _, err := tea.NewProgram(app).Run(); err != nil {
		fmt.Fprintf(stderr, "prt: %v\n", err)
		return 1
	}
	if err := app.Err(); err != nil && !errors.Is(err, context.Canceled) {
		fmt.Fprintf(stderr, "prt: %v\n", err)
		return 1
	}
	return 0
}

// prepare works out which pull request to show and how to fetch it. It does
// not touch the network.
func prepare(cfg config) (ui.Options, error) {
	if cfg.demo {
		return options(demo.Ref, github.Options{Host: demo.Ref.Host, Token: demo.Token, Transport: demo.Transport()}, demo.Now)
	}

	ref, err := resolve(cfg)
	if err != nil {
		return ui.Options{}, err
	}
	token, err := auth.ForHost(ref.Host)
	if err != nil {
		return ui.Options{}, err
	}
	return options(ref, github.Options{Host: ref.Host, Token: token.Value, TokenSource: token.Source}, time.Now)
}

func options(ref github.Ref, clientOptions github.Options, now func() time.Time) (ui.Options, error) {
	client, err := github.New(clientOptions)
	if err != nil {
		return ui.Options{}, err
	}
	return ui.Options{
		Title: ref.String(),
		Now:   now,
		Load: func(ctx context.Context) (*model.PullRequest, error) {
			return client.PullRequest(ctx, ref)
		},
	}, nil
}

// resolve fills in the parts of the pull request's name that the command
// line left out: the repository for a bare number, and the host.
func resolve(cfg config) (github.Ref, error) {
	ref, err := github.ParseRef(cfg.target)
	if err != nil {
		return ref, usageError{err.Error()}
	}

	switch {
	case ref.Owner != "" && cfg.repo != "":
		return ref, usageError{fmt.Sprintf("%q already names its repository; leave out --repo", cfg.target)}
	case ref.Owner != "":
		// Named in full.
	case cfg.repo != "":
		repo, err := repository.Parse(cfg.repo)
		if err != nil {
			return ref, usageError{fmt.Sprintf("--repo: %v", err)}
		}
		ref.Host, ref.Owner, ref.Repo = repo.Host, repo.Owner, repo.Name
	default:
		repo, err := repository.Current()
		if err != nil {
			// The reason can carry git's own multi-line output.
			reason := strings.Join(strings.Fields(err.Error()), " ")
			return ref, fmt.Errorf("could not tell which repository #%d belongs to (%s): run prt inside a clone, or name the repository with owner/repo#%d, a URL, or --repo", ref.Number, reason, ref.Number)
		}
		ref.Host, ref.Owner, ref.Repo = repo.Host, repo.Owner, repo.Name
	}

	if ref.Host == "" {
		ref.Host = auth.DefaultHost()
	}
	return ref, nil
}

func buildVersion() string {
	if version != "" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return "dev"
}
