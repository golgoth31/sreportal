package image

// TagType classifies the image tag format.
type TagType string

const (
	TagTypeSemver TagType = "semver"
	TagTypeCommit TagType = "commit"
	TagTypeDigest TagType = "digest"
	TagTypeLatest TagType = "latest"
)

// WorkloadRef identifies a workload/container using an image.
type WorkloadRef struct {
	Kind      string
	Namespace string
	Name      string
	Container string
}

// ImageView is the read-side projection of images in a portal scope.
type ImageView struct {
	PortalRef  string
	Registry   string
	Repository string
	Tag        string
	TagType    TagType
	Workloads  []WorkloadRef
}

// ImageFilters are the criteria for listing images.
type ImageFilters struct {
	Portal   string
	Search   string // substring on repository
	Registry string
	TagType  string
}
