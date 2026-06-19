// Package deploystatus holds the read model and store interfaces for the deploy-status feature.
package deploystatus

import "time"

// Entry is the read-model projection of one service's deploy status.
type Entry struct {
	Key            string
	Workload       WorkloadRef
	Image          string
	SourceRepo     string
	DeployedRef    string
	DefaultBranch  string
	AheadBy        int
	PendingCommits   []Commit
	PendingTruncated bool
	DeployedAt       time.Time
	DeployRunURL   string
	State          string // ok | behind | unresolved | error
	Error          string
	LastCheckedAt  time.Time
}

// WorkloadRef identifies the workload that owns this deploy-status entry.
type WorkloadRef struct {
	Kind      string
	Namespace string
	Name      string
	Container string
}

// Commit is a pending (not-yet-deployed) commit ahead of the deployed ref.
type Commit struct {
	Sha     string
	Message string
	Author  string
	Date    time.Time
	URL     string
}
