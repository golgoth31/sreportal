package deploystatus

// Reader is the read side consumed by gRPC/MCP.
type Reader interface {
	// List returns all entries for a portal (deduplicated across contributing namespaces).
	List(portalRef string) []Entry
	// Count returns the number of entries for a portal.
	Count(portalRef string) int
	// Subscribe returns a channel that is closed on the next mutation and an
	// unsubscribe function the caller must invoke when it is done listening.
	Subscribe() (<-chan struct{}, func())
}
