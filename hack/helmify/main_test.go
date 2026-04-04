package main

import "testing"

func TestRunHelmify_NoPanic(t *testing.T) {
	// Smoke: package compiles; full helmify requires bin/kustomize and bin/helmify.
	t.Parallel()
}
