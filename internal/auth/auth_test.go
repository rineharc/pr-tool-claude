package auth

import (
	"strings"
	"testing"
)

// isolate makes the lookup see only what a test sets: no token variables, an
// empty gh configuration directory, and no gh executable.
func isolate(t *testing.T) {
	t.Helper()
	for _, name := range []string{"GH_TOKEN", "GITHUB_TOKEN", "GH_ENTERPRISE_TOKEN", "GITHUB_ENTERPRISE_TOKEN", "GH_HOST", "GH_PATH"} {
		t.Setenv(name, "")
	}
	t.Setenv("GH_CONFIG_DIR", t.TempDir())
	t.Setenv("PATH", t.TempDir())
}

func TestForHost(t *testing.T) {
	tests := []struct {
		name       string
		host       string
		env        map[string]string
		wantValue  string
		wantSource string
		wantErr    string
	}{
		{
			name:       "GH_TOKEN",
			host:       "github.com",
			env:        map[string]string{"GH_TOKEN": "from-gh-token"},
			wantValue:  "from-gh-token",
			wantSource: "the GH_TOKEN environment variable",
		},
		{
			name:       "GITHUB_TOKEN",
			host:       "github.com",
			env:        map[string]string{"GITHUB_TOKEN": "from-github-token"},
			wantValue:  "from-github-token",
			wantSource: "the GITHUB_TOKEN environment variable",
		},
		{
			name:       "GH_TOKEN wins over GITHUB_TOKEN",
			host:       "github.com",
			env:        map[string]string{"GH_TOKEN": "first", "GITHUB_TOKEN": "second"},
			wantValue:  "first",
			wantSource: "the GH_TOKEN environment variable",
		},
		{
			name:       "another host uses the enterprise variable",
			host:       "github.example.com",
			env:        map[string]string{"GH_TOKEN": "dotcom", "GH_ENTERPRISE_TOKEN": "enterprise"},
			wantValue:  "enterprise",
			wantSource: "the GH_ENTERPRISE_TOKEN environment variable",
		},
		{
			name:    "nothing configured",
			host:    "github.com",
			wantErr: "no GitHub token found for github.com: run `gh auth login`, or set GH_TOKEN",
		},
		{
			name:    "nothing configured for another host",
			host:    "github.example.com",
			env:     map[string]string{"GH_TOKEN": "dotcom"},
			wantErr: "set GH_ENTERPRISE_TOKEN",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isolate(t)
			for name, value := range tt.env {
				t.Setenv(name, value)
			}

			got, err := ForHost(tt.host)

			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("ForHost() error = %v, want it to contain %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ForHost() error = %v", err)
			}
			if got.Value != tt.wantValue || got.Source != tt.wantSource {
				t.Errorf("ForHost() = %+v, want value %q from %q", got, tt.wantValue, tt.wantSource)
			}
		})
	}
}

func TestDefaultHost(t *testing.T) {
	isolate(t)
	if got := DefaultHost(); got != "github.com" {
		t.Errorf("DefaultHost() = %q, want github.com", got)
	}

	t.Setenv("GH_HOST", "github.example.com")
	if got := DefaultHost(); got != "github.example.com" {
		t.Errorf("DefaultHost() with GH_HOST = %q, want github.example.com", got)
	}
}
