package deploystatus

// Writer is the write side the controller pushes projections into.
type Writer interface {
	// ReplaceForNamespace replaces all entries contributed by (portalRef, namespace)
	// with the provided slice. Contributions from other namespaces under the same
	// portalRef are preserved.
	ReplaceForNamespace(portalRef, namespace string, entries []Entry)
	// RemoveForNamespace drops a (portalRef, namespace) contribution (CR deletion).
	RemoveForNamespace(portalRef, namespace string)
}
