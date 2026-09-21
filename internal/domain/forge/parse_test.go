package forge_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/golgoth31/sreportal/internal/domain/forge"
)

const (
	testOwnerAcme  = "acme"
	testHostGitHub = "github.com"
	testRepoMyApp  = "myapp"
)

func TestParseSourceURL(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    forge.RepoRef
		wantErr bool
	}{
		{
			name: "https github.com",
			raw:  "https://github.com/acme/myapp",
			want: forge.RepoRef{Host: testHostGitHub, Owner: testOwnerAcme, Repo: testRepoMyApp},
		},
		{
			name: "https with .git suffix",
			raw:  "https://github.com/acme/myapp.git",
			want: forge.RepoRef{Host: testHostGitHub, Owner: testOwnerAcme, Repo: testRepoMyApp},
		},
		{
			name: "https GHES host",
			raw:  "https://ghe.example.com/org/repo",
			want: forge.RepoRef{Host: "ghe.example.com", Owner: "org", Repo: "repo"},
		},
		{
			name: "scp-style ssh git@github.com",
			raw:  "git@github.com:acme/myapp.git",
			want: forge.RepoRef{Host: testHostGitHub, Owner: testOwnerAcme, Repo: testRepoMyApp},
		},
		{
			name: "scp-style ssh GHES",
			raw:  "git@ghe.example.com:org/repo.git",
			want: forge.RepoRef{Host: "ghe.example.com", Owner: "org", Repo: "repo"},
		},
		{
			name:    "empty string",
			raw:     "",
			wantErr: true,
		},
		{
			name:    "no owner or repo",
			raw:     "https://github.com/",
			wantErr: true,
		},
		{
			name:    "only host no path",
			raw:     "https://github.com",
			wantErr: true,
		},
		{
			name:    "missing repo in path",
			raw:     "https://github.com/acme",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := forge.ParseSourceURL(tc.raw)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}
