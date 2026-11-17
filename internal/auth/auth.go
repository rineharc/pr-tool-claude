// Package auth finds the GitHub token to use. It reuses the lookup that the
// GitHub CLI's own library provides, so a machine where gh is signed in needs
// no further setup.
package auth

import (
	"fmt"

	ghauth "github.com/cli/go-gh/v2/pkg/auth"
)

// Token is a GitHub token and where it came from.
type Token struct {
	Value string
	// Source describes the origin in words, for messages to the user.
	Source string
}

// DefaultHost is the GitHub host to use when the user names none: the one gh
// is configured for, or github.com.
func DefaultHost() string {
	host, _ := ghauth.DefaultHost()
	return host
}

// ForHost returns the token for host. It looks, in order, at the environment
// (GH_TOKEN, then GITHUB_TOKEN; GH_ENTERPRISE_TOKEN, then
// GITHUB_ENTERPRISE_TOKEN for other hosts), at gh's configuration file, and
// at gh's stored credentials.
func ForHost(host string) (Token, error) {
	value, source := ghauth.TokenForHost(host)
	if value == "" {
		return Token{}, fmt.Errorf("no GitHub token found for %s: run `gh auth login`, or set %s", host, envName(host))
	}
	return Token{Value: value, Source: describe(source)}, nil
}

func envName(host string) string {
	if ghauth.IsEnterprise(host) {
		return "GH_ENTERPRISE_TOKEN"
	}
	return "GH_TOKEN"
}

func describe(source string) string {
	switch source {
	case "oauth_token":
		return "gh's configuration file"
	case "gh":
		return "gh's stored credentials"
	default:
		return "the " + source + " environment variable"
	}
}
