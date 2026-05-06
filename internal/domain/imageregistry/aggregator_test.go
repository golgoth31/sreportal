/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
*/

package imageregistry

import (
	"sort"
	"testing"

	domainimage "github.com/golgoth31/sreportal/internal/domain/image"
)

func TestAggregateForCRs_NoneMutatedInjected(t *testing.T) {
	t.Parallel()

	scan := []ContainerObservation{
		// none: same image in template & pod (semver tag for parse coverage).
		{
			WorkloadKind: tKindDeploy, WorkloadName: tWorkloadWeb, WorkloadNamespace: tNsDefault,
			ContainerName: tContainerApp,
			TemplateImage: tImgNginxDocker,
			PodImage:      tImgNginxDocker,
		},
		// mutated: template & pod differ. Lookup target = template (docker.io)
		// so this groups under docker.io.
		{
			WorkloadKind: tKindDeploy, WorkloadName: tContainerAPI, WorkloadNamespace: tNsDefault,
			ContainerName: tContainerApp,
			TemplateImage: tImgRedisDocker,
			PodImage:      tImgRedisMirror,
		},
		// injected: container absent from template. Lookup target = pod
		// (docker.io for istio image), groups under docker.io.
		{
			WorkloadKind: tKindDeploy, WorkloadName: tContainerAPI, WorkloadNamespace: tNsDefault,
			ContainerName: "istio-proxy",
			TemplateImage: "",
			PodImage:      "docker.io/istio/proxy:1.20.0",
		},
	}

	got := AggregateForCRs(tPortalMain, scan)

	g := Group{Host: tHostIndexDocker, Namespace: tNsDefault}
	if len(got) != 1 {
		t.Fatalf("expected only index.docker.io/default group, got %d groups: %+v", len(got), got)
	}
	if len(got[g]) != 3 {
		t.Fatalf("index.docker.io/default group should have 3 entries (none + mutated + injected), got %d: %+v", len(got[g]), got[g])
	}

	var nonE, mutE, injE *Entry
	for i := range got[g] {
		e := &got[g][i]
		switch e.ChangeType {
		case ChangeTypeNone:
			nonE = e
		case ChangeTypeMutated:
			mutE = e
		case ChangeTypeInjected:
			injE = e
		}
	}
	if nonE == nil || mutE == nil || injE == nil {
		t.Fatalf("missing one of (none/mutated/injected): %+v", got[g])
	}

	if nonE.OriginalImage != tImgNginxDocker || nonE.MutatedImage != tImgNginxDocker {
		t.Fatalf("none entry images: %+v", nonE)
	}
	if nonE.Repository != "library/nginx" || nonE.OriginalTag != "1.25.0" || nonE.TagType != domainimage.TagTypeSemver {
		t.Fatalf("none parsed: repo=%q tag=%q tagType=%q", nonE.Repository, nonE.OriginalTag, nonE.TagType)
	}

	if mutE.OriginalImage != tImgRedisDocker || mutE.MutatedImage != tImgRedisMirror {
		t.Fatalf("mutated images: %+v", mutE)
	}
	if mutE.Repository != "library/redis" || mutE.OriginalTag != "7.0.0" || mutE.TagType != domainimage.TagTypeSemver {
		t.Fatalf("mutated parsed: repo=%q tag=%q tagType=%q", mutE.Repository, mutE.OriginalTag, mutE.TagType)
	}

	if injE.OriginalImage != "" {
		t.Fatalf("injected must have empty OriginalImage, got %q", injE.OriginalImage)
	}
	if injE.MutatedImage != "docker.io/istio/proxy:1.20.0" {
		t.Fatalf("injected MutatedImage: %q", injE.MutatedImage)
	}
	if injE.Repository != "istio/proxy" || injE.OriginalTag != "1.20.0" {
		t.Fatalf("injected parsed: repo=%q tag=%q", injE.Repository, injE.OriginalTag)
	}
}

func TestAggregateForCRs_GroupingByLookupHost(t *testing.T) {
	t.Parallel()

	scan := []ContainerObservation{
		// mutated: original = docker.io, mutated = mirror.io
		// Lookup target = original → group host = docker.io
		{
			WorkloadKind: tKindDeploy, WorkloadName: tContainerAPI, WorkloadNamespace: tNsDefault,
			ContainerName: tContainerApp,
			TemplateImage: tImgRedisDocker,
			PodImage:      tImgRedisMirror,
		},
		// injected: only podImage = ghcr.io/foo
		// Lookup target = mutated → group host = ghcr.io
		{
			WorkloadKind: tKindDeploy, WorkloadName: tContainerAPI, WorkloadNamespace: tNsDefault,
			ContainerName: "sidecar",
			TemplateImage: "",
			PodImage:      "ghcr.io/foo/bar:1.0.0",
		},
	}

	got := AggregateForCRs(tPortalMain, scan)

	if len(got[Group{Host: tHostIndexDocker, Namespace: tNsDefault}]) != 1 {
		t.Fatalf("expected mutated grouped under docker.io: %+v", got)
	}
	if len(got[Group{Host: "ghcr.io", Namespace: tNsDefault}]) != 1 {
		t.Fatalf("expected injected grouped under ghcr.io: %+v", got)
	}
}

func TestAggregateForCRs_DedupMultiWorkload(t *testing.T) {
	t.Parallel()

	scan := []ContainerObservation{
		{
			WorkloadKind: tKindDeploy, WorkloadName: "web1", WorkloadNamespace: tNsDefault,
			ContainerName: tContainerApp,
			TemplateImage: tImgNginxDocker,
			PodImage:      tImgNginxDocker,
		},
		{
			WorkloadKind: tKindDeploy, WorkloadName: "web2", WorkloadNamespace: tNsDefault,
			ContainerName: tContainerApp,
			TemplateImage: tImgNginxDocker,
			PodImage:      tImgNginxDocker,
		},
	}

	got := AggregateForCRs(tPortalMain, scan)
	g := Group{Host: tHostIndexDocker, Namespace: tNsDefault}
	if len(got[g]) != 1 {
		t.Fatalf("expected 1 dedup entry, got %d: %+v", len(got[g]), got[g])
	}
	e := got[g][0]
	if len(e.Workloads) != 2 {
		t.Fatalf("expected 2 workloads, got %d: %+v", len(e.Workloads), e.Workloads)
	}
	// Sorted by name for stability
	names := []string{e.Workloads[0].Name, e.Workloads[1].Name}
	sort.Strings(names)
	if names[0] != "web1" || names[1] != "web2" {
		t.Fatalf("workloads names: %v", names)
	}
}

func TestAggregateForCRs_DiffContainerNotDeduped(t *testing.T) {
	t.Parallel()

	scan := []ContainerObservation{
		{
			WorkloadKind: tKindDeploy, WorkloadName: tWorkloadWeb, WorkloadNamespace: tNsDefault,
			ContainerName: tContainerApp,
			TemplateImage: tImgNginxDocker,
			PodImage:      tImgNginxDocker,
		},
		{
			WorkloadKind: tKindDeploy, WorkloadName: tWorkloadWeb, WorkloadNamespace: tNsDefault,
			ContainerName: "side", // different container, same image
			TemplateImage: tImgNginxDocker,
			PodImage:      tImgNginxDocker,
		},
	}
	got := AggregateForCRs(tPortalMain, scan)
	g := Group{Host: tHostIndexDocker, Namespace: tNsDefault}
	if len(got[g]) != 2 {
		t.Fatalf("different containers must produce distinct entries; got %d", len(got[g]))
	}
}

func TestAggregateForCRs_GroupByNamespace(t *testing.T) {
	t.Parallel()

	scan := []ContainerObservation{
		{
			WorkloadKind: tKindDeploy, WorkloadName: "a", WorkloadNamespace: "ns-a",
			ContainerName: tContainerApp,
			TemplateImage: tImgNginxDocker,
			PodImage:      tImgNginxDocker,
		},
		{
			WorkloadKind: tKindDeploy, WorkloadName: "b", WorkloadNamespace: "ns-b",
			ContainerName: tContainerApp,
			TemplateImage: tImgNginxDocker,
			PodImage:      tImgNginxDocker,
		},
	}
	got := AggregateForCRs(tPortalMain, scan)
	if len(got) != 2 {
		t.Fatalf("expected 2 namespace groups, got %d", len(got))
	}
	if len(got[Group{Host: tHostIndexDocker, Namespace: "ns-a"}]) != 1 {
		t.Fatalf("ns-a missing: %+v", got)
	}
	if len(got[Group{Host: tHostIndexDocker, Namespace: "ns-b"}]) != 1 {
		t.Fatalf("ns-b missing: %+v", got)
	}
}

func TestAggregateForCRs_KeyStableAcrossRuns(t *testing.T) {
	t.Parallel()

	scan := []ContainerObservation{
		{
			WorkloadKind: tKindDeploy, WorkloadName: "a", WorkloadNamespace: tNsDefault,
			ContainerName: tContainerApp,
			TemplateImage: tImgNginxDocker,
			PodImage:      tImgNginxDocker,
		},
	}
	g1 := AggregateForCRs(tPortalMain, scan)
	g2 := AggregateForCRs(tPortalMain, scan)
	g := Group{Host: tHostIndexDocker, Namespace: tNsDefault}
	if g1[g][0].Key != g2[g][0].Key {
		t.Fatalf("Key must be stable: %q vs %q", g1[g][0].Key, g2[g][0].Key)
	}
	if len(g1[g][0].Key) == 0 {
		t.Fatalf("Key must not be empty")
	}
}

func TestAggregateForCRs_SkipUnparsableImages(t *testing.T) {
	t.Parallel()

	scan := []ContainerObservation{
		{
			WorkloadKind: tKindDeploy, WorkloadName: "bad", WorkloadNamespace: tNsDefault,
			ContainerName: tContainerApp,
			TemplateImage: "::not-a-ref",
			PodImage:      "::not-a-ref",
		},
	}
	got := AggregateForCRs(tPortalMain, scan)
	if len(got) != 0 {
		t.Fatalf("unparsable images must be dropped, got %+v", got)
	}
}

func TestAggregateForCRs_EmptyPodImageDropped(t *testing.T) {
	t.Parallel()

	scan := []ContainerObservation{
		{
			WorkloadKind: tKindDeploy, WorkloadName: "x", WorkloadNamespace: tNsDefault,
			ContainerName: tContainerApp,
			TemplateImage: tImgNginxDocker,
			PodImage:      "", // missing pod image: drop
		},
	}
	got := AggregateForCRs(tPortalMain, scan)
	if len(got) != 0 {
		t.Fatalf("observation without PodImage must be dropped, got %+v", got)
	}
}
