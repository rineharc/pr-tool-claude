package github

import "testing"

func TestParseRef(t *testing.T) {
	tests := []struct {
		arg  string
		want Ref
	}{
		{"482", Ref{Number: 482}},
		{"#482", Ref{Number: 482}},
		{" 482 ", Ref{Number: 482}},
		{"acme/payments-api#482", Ref{Owner: "acme", Repo: "payments-api", Number: 482}},
		{"my.org/repo.name#7", Ref{Owner: "my.org", Repo: "repo.name", Number: 7}},
		{"github.example.com/acme/api#482", Ref{Host: "github.example.com", Owner: "acme", Repo: "api", Number: 482}},
		{"https://github.com/acme/payments-api/pull/482", Ref{Host: "github.com", Owner: "acme", Repo: "payments-api", Number: 482}},
		{"https://github.com/acme/payments-api/pull/482/", Ref{Host: "github.com", Owner: "acme", Repo: "payments-api", Number: 482}},
		{"https://github.com/acme/payments-api/pull/482/files", Ref{Host: "github.com", Owner: "acme", Repo: "payments-api", Number: 482}},
		{"https://github.com/acme/payments-api/pull/482/files?w=1#diff-abc", Ref{Host: "github.com", Owner: "acme", Repo: "payments-api", Number: 482}},
		{"https://github.com/acme/payments-api/pull/482#issuecomment-1", Ref{Host: "github.com", Owner: "acme", Repo: "payments-api", Number: 482}},
		{"http://GitHub.com/acme/payments-api/pull/482", Ref{Host: "github.com", Owner: "acme", Repo: "payments-api", Number: 482}},
		{"https://www.github.com/acme/payments-api/pull/482", Ref{Host: "github.com", Owner: "acme", Repo: "payments-api", Number: 482}},
		{"github.com/acme/payments-api/pull/482", Ref{Host: "github.com", Owner: "acme", Repo: "payments-api", Number: 482}},
		{"https://github.example.com/acme/api/pull/9", Ref{Host: "github.example.com", Owner: "acme", Repo: "api", Number: 9}},
	}
	for _, tt := range tests {
		got, err := ParseRef(tt.arg)
		if err != nil {
			t.Errorf("ParseRef(%q) error = %v", tt.arg, err)
			continue
		}
		if got != tt.want {
			t.Errorf("ParseRef(%q) = %+v, want %+v", tt.arg, got, tt.want)
		}
	}
}

func TestParseRefRejects(t *testing.T) {
	for _, arg := range []string{
		"",
		"   ",
		"0",
		"#0",
		"-3",
		"abc",
		"acme/payments-api",
		"acme/payments-api#",
		"acme/payments-api#x",
		"acme#482",
		"a/b/c/d#482",
		"https://github.com/acme/payments-api",
		"https://github.com/acme/payments-api/issues/482",
		"https://github.com/acme/payments-api/pull/",
		"https://github.com/acme/payments-api/pull/abc",
		"ftp://github.com/acme/payments-api/pull/482",
		"99999999999999999999999",
	} {
		if got, err := ParseRef(arg); err == nil {
			t.Errorf("ParseRef(%q) = %+v, want an error", arg, got)
		}
	}
}

func TestRefString(t *testing.T) {
	ref := Ref{Host: "github.com", Owner: "acme", Repo: "payments-api", Number: 482}
	if got := ref.String(); got != "acme/payments-api #482" {
		t.Errorf("String() = %q", got)
	}
}
