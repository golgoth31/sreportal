package image

import "testing"

func TestClassifyTag(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		tag  string
		want TagType
	}{
		{name: "digest", tag: "sha256:012345", want: TagTypeDigest},
		{name: "latest", tag: "latest", want: TagTypeLatest},
		{name: "semver plain", tag: "1.2.3", want: TagTypeSemver},
		{name: "semver with v", tag: "v1.2.3", want: TagTypeSemver},
		{name: "commit", tag: "abcdef12", want: TagTypeCommit},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := ClassifyTag(tc.tag)
			if got != tc.want {
				t.Fatalf("ClassifyTag(%q)=%q want %q", tc.tag, got, tc.want)
			}
		})
	}
}

func TestParseReference(t *testing.T) {
	t.Parallel()

	got, err := ParseReference("europe-docker.pkg.dev/my-project/my-repo/vault:1.20.1")
	if err != nil {
		t.Fatalf("ParseReference() error = %v", err)
	}
	if got.Registry != "europe-docker.pkg.dev" {
		t.Fatalf("registry=%q", got.Registry)
	}
	if got.Repository != "my-project/my-repo/vault" {
		t.Fatalf("repository=%q", got.Repository)
	}
	if got.Tag != "1.20.1" {
		t.Fatalf("tag=%q", got.Tag)
	}
	if got.TagType != TagTypeSemver {
		t.Fatalf("tagType=%q", got.TagType)
	}
}
