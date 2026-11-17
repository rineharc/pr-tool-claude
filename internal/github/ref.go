package github

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// Ref names a pull request. Host, Owner and Repo are empty when the user gave
// only a number.
type Ref struct {
	Host   string
	Owner  string
	Repo   string
	Number int
}

func (r Ref) String() string {
	return fmt.Sprintf("%s/%s #%d", r.Owner, r.Repo, r.Number)
}

var (
	numberRE    = regexp.MustCompile(`^#?(\d+)$`)
	shorthandRE = regexp.MustCompile(`^(?:([^/#\s]+)/)?([^/#\s]+)/([^/#\s]+)#(\d+)$`)
)

// ParseRef understands the ways a pull request can be named on the command
// line: "482", "#482", "owner/repo#482", "host/owner/repo#482", and the
// pull request's URL, with or without the scheme and with anything after the
// number.
func ParseRef(arg string) (Ref, error) {
	arg = strings.TrimSpace(arg)
	if arg == "" {
		return Ref{}, fmt.Errorf("no pull request given")
	}

	if m := numberRE.FindStringSubmatch(arg); m != nil {
		return withNumber(Ref{}, m[1], arg)
	}
	if m := shorthandRE.FindStringSubmatch(arg); m != nil {
		return withNumber(Ref{Host: strings.ToLower(m[1]), Owner: m[2], Repo: m[3]}, m[4], arg)
	}
	if ref, ok := parseURL(arg); ok {
		return ref, nil
	}
	return Ref{}, fmt.Errorf("%q is not a pull request: give a number, owner/repo#number, or the pull request's URL", arg)
}

func parseURL(arg string) (Ref, bool) {
	raw := arg
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") {
		return Ref{}, false
	}

	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 4 || parts[2] != "pull" || parts[0] == "" || parts[1] == "" {
		return Ref{}, false
	}
	number, err := strconv.Atoi(parts[3])
	if err != nil || number <= 0 {
		return Ref{}, false
	}
	host := strings.TrimPrefix(strings.ToLower(u.Host), "www.")
	return Ref{Host: host, Owner: parts[0], Repo: parts[1], Number: number}, true
}

func withNumber(ref Ref, digits, arg string) (Ref, error) {
	number, err := strconv.Atoi(digits)
	if err != nil || number <= 0 {
		return Ref{}, fmt.Errorf("%q is not a valid pull request number", arg)
	}
	ref.Number = number
	return ref, nil
}
