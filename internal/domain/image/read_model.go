package image

// TagType classifies the image tag format.
type TagType string

const (
	TagTypeSemver TagType = "semver"
	TagTypeCommit TagType = "commit"
	TagTypeDigest TagType = "digest"
	TagTypeLatest TagType = "latest"
	TagTypeOther  TagType = "other"
)

// ContainerSource indicates whether a container/image was discovered from
// the workload template spec or only observed in the running pod (typically
// because a MutatingWebhook injected or mutated it).
type ContainerSource string

const (
	// ContainerSourceSpec marks containers declared in the workload's PodSpec template.
	ContainerSourceSpec ContainerSource = "spec"
	// ContainerSourcePod marks containers observed only in the running pod —
	// either added by a MutatingWebhook or whose image was mutated post-admission.
	ContainerSourcePod ContainerSource = "pod"
)

// WorkloadRef identifies a workload/container using an image.
type WorkloadRef struct {
	Kind      string
	Namespace string
	Name      string
	Container string
	Source    ContainerSource
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
