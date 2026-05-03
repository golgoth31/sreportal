package image

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
)

var (
	semverRE = regexp.MustCompile(`^v?\d+\.\d+\.\d+(?:[-+][0-9A-Za-z\.-]+)?$`)
	commitRE = regexp.MustCompile(`^[a-f0-9]{7,40}$`)
)

// ParsedReference is a normalized image reference.
type ParsedReference struct {
	Registry   string
	Repository string
	Tag        string
	TagType    TagType
}

// ParseReference parses an OCI image reference into normalized parts.
func ParseReference(ref string) (ParsedReference, error) {
	r, err := name.ParseReference(ref)
	if err != nil {
		return ParsedReference{}, fmt.Errorf("parse image reference %q: %w", ref, err)
	}
	switch typed := r.(type) {
	case name.Digest:
		return ParsedReference{
			Registry:   typed.Context().RegistryStr(),
			Repository: typed.Context().RepositoryStr(),
			Tag:        typed.DigestStr(),
			TagType:    TagTypeDigest,
		}, nil
	case name.Tag:
		tag := typed.TagStr()
		return ParsedReference{
			Registry:   typed.Context().RegistryStr(),
			Repository: typed.Context().RepositoryStr(),
			Tag:        tag,
			TagType:    ClassifyTag(tag),
		}, nil
	default:
		return ParsedReference{}, fmt.Errorf("unsupported reference %q", ref)
	}
}

// ClassifyTag classifies a Docker tag.
func ClassifyTag(tag string) TagType {
	switch {
	case strings.HasPrefix(tag, "sha256:"):
		return TagTypeDigest
	case tag == string(TagTypeLatest):
		return TagTypeLatest
	case semverRE.MatchString(tag):
		return TagTypeSemver
	case commitRE.MatchString(tag):
		return TagTypeCommit
	default:
		return TagTypeOther
	}
}
