package image

import "time"

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
//
// The new image-registry pipeline enriches this view with:
//   - the original (template) and mutated (runtime) image references,
//   - the change_type classification computed by the aggregator,
//   - the result of the registry lookup (latest semver tag and timestamp).
type ImageView struct {
	PortalRef  string
	Registry   string
	Repository string
	Tag        string
	TagType    TagType

	// OriginalImage is the image declared in the workload's PodSpec template.
	// Empty when ChangeType=injected.
	OriginalImage string
	// MutatedImage is the image observed in the running Pod.
	MutatedImage string
	// ChangeType classifies the relationship between OriginalImage and MutatedImage.
	// One of: none, mutated, injected.
	ChangeType string

	// LatestVersion is the highest semver tag found on the registry-of-origin.
	// Empty when the lookup is not applicable (non-semver tag) or after a failed lookup.
	LatestVersion string
	// LatestCheckedAt is the timestamp of the most recent successful registry lookup.
	LatestCheckedAt *time.Time
	// LatestError carries the last lookup error, if any.
	LatestError string
	// UpgradeAvailable is true when LatestVersion is strictly greater than Tag (semver).
	UpgradeAvailable bool

	Workloads []WorkloadRef
}

// ImageFilters are the criteria for listing images.
type ImageFilters struct {
	Portal   string
	Search   string // substring on repository
	Registry string
	TagType  string
}
