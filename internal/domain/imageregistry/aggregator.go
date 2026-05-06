/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
*/

package imageregistry

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"

	domainimage "github.com/golgoth31/sreportal/internal/domain/image"
)

// entryKeyLen is the byte length of the truncated sha256 used as
// listMapKey on Spec.Images. 16 hex chars = 64 bits, well below collision
// risk for the cardinality of containers in one (portal, host, ns) tuple.
const entryKeyLen = 16

// AggregateForCRs takes raw scan results (containers from Pods + workload
// templates) and returns groups suitable for ImageRegistry CR Spec
// generation, keyed by (host, namespace).
//
// For each ContainerObservation:
//  1. Compute ChangeType (none/mutated/injected) based on TemplateImage vs
//     PodImage.
//  2. The lookup target image is `TemplateImage` if non-empty, otherwise
//     `PodImage` (i.e. the registry-of-origin image).
//  3. Parse the lookup target into (host, repository, tag, tagType).
//  4. Group by (host, namespace), dedup by (originalImage, mutatedImage,
//     containerName) — agreggating workloads from duplicates.
//
// Observations whose PodImage is empty, or whose lookup target is
// unparsable, are dropped.
func AggregateForCRs(_ string, scan []ContainerObservation) map[Group][]Entry {
	type bucket struct {
		entry     Entry
		workloads []WorkloadRef
	}
	groups := make(map[Group]map[string]*bucket)

	for _, obs := range scan {
		if obs.PodImage == "" {
			continue
		}
		original := obs.TemplateImage
		mutated := obs.PodImage

		var changeType ChangeType
		switch original {
		case "":
			changeType = ChangeTypeInjected
		case mutated:
			changeType = ChangeTypeNone
		default:
			changeType = ChangeTypeMutated
		}

		// Lookup target = OriginalImage when present, else MutatedImage.
		lookupTarget := original
		if lookupTarget == "" {
			lookupTarget = mutated
		}

		parsed, err := domainimage.ParseReference(lookupTarget)
		if err != nil {
			continue
		}

		// Dedup key inside a group: (originalImage, mutatedImage, container).
		dedupKey := entryKey(original, mutated, obs.ContainerName)
		groupKey := Group{Host: parsed.Registry, Namespace: obs.WorkloadNamespace}

		grp, ok := groups[groupKey]
		if !ok {
			grp = make(map[string]*bucket)
			groups[groupKey] = grp
		}

		b, ok := grp[dedupKey]
		if !ok {
			b = &bucket{
				entry: Entry{
					Key:           dedupKey,
					OriginalImage: original,
					MutatedImage:  mutated,
					ChangeType:    changeType,
					Repository:    parsed.Repository,
					OriginalTag:   parsed.Tag,
					TagType:       parsed.TagType,
				},
			}
			grp[dedupKey] = b
		}
		b.workloads = append(b.workloads, WorkloadRef{
			Kind:      obs.WorkloadKind,
			Namespace: obs.WorkloadNamespace,
			Name:      obs.WorkloadName,
			Container: obs.ContainerName,
		})
	}

	out := make(map[Group][]Entry, len(groups))
	for gk, buckets := range groups {
		entries := make([]Entry, 0, len(buckets))
		// Stable order per group: sort buckets by Key.
		keys := make([]string, 0, len(buckets))
		for k := range buckets {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			b := buckets[k]
			sort.SliceStable(b.workloads, func(i, j int) bool {
				if b.workloads[i].Namespace != b.workloads[j].Namespace {
					return b.workloads[i].Namespace < b.workloads[j].Namespace
				}
				if b.workloads[i].Kind != b.workloads[j].Kind {
					return b.workloads[i].Kind < b.workloads[j].Kind
				}
				if b.workloads[i].Name != b.workloads[j].Name {
					return b.workloads[i].Name < b.workloads[j].Name
				}
				return b.workloads[i].Container < b.workloads[j].Container
			})
			b.entry.Workloads = b.workloads
			entries = append(entries, b.entry)
		}
		out[gk] = entries
	}
	return out
}

// entryKey is the listMapKey used by Spec.Images.
// sha256(originalImage|mutatedImage|container)[:16] in lowercase hex.
func entryKey(original, mutated, container string) string {
	sum := sha256.Sum256([]byte(original + "|" + mutated + "|" + container))
	return hex.EncodeToString(sum[:])[:entryKeyLen]
}
