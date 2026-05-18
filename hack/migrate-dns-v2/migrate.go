package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	v1alpha2 "github.com/golgoth31/sreportal/api/v1alpha2"
)

// Summary aggregates the outcome of a migration run so callers can decide
// whether to exit non-zero and operators can see what happened at a glance.
type Summary struct {
	DNSProcessed int
	Created      int
	AlreadyExist int
	Skipped      int
	Failures     int
}

// Migrate converts the v1alpha1 groups annotation on every DNS CR into
// origin=manual DNSRecord CRs. The v1alpha1 groups annotation is only stripped
// from a DNS when *every* non-empty group successfully materialised — a
// partial failure leaves the annotation in place so a retry can complete the
// migration. When dryRun is true, Create calls use client.DryRunAll for
// server-side validation without persisting and the annotation is never
// removed.
func Migrate(ctx context.Context, c client.Client, dryRun bool) (Summary, error) {
	var (
		sum     Summary
		dnsList v1alpha2.DNSList
		errAggr []error
	)
	if err := c.List(ctx, &dnsList); err != nil {
		return sum, fmt.Errorf("list DNS: %w", err)
	}

	for i := range dnsList.Items {
		sum.DNSProcessed++
		dns := &dnsList.Items[i]
		raw, ok := dns.Annotations[annotationV1Alpha1Groups]
		if !ok || raw == "" {
			sum.Skipped++
			fmt.Printf("DNS %s/%s: no v1alpha1 groups annotation, skipping\n", dns.Namespace, dns.Name)
			continue
		}
		var groups []v1alpha1.DNSGroup
		if err := json.Unmarshal([]byte(raw), &groups); err != nil {
			sum.Failures++
			errAggr = append(errAggr, fmt.Errorf("DNS %s/%s: parse groups: %w", dns.Namespace, dns.Name, err))
			continue
		}

		groupCount, perDNSCreated, perDNSFailures := 0, 0, 0
		for _, g := range groups {
			if len(g.Entries) == 0 {
				continue
			}
			groupCount++
			recordName := dns.Name + "-manual-" + slug(g.Name)
			entries := make([]v1alpha2.DNSRecordEntry, 0, len(g.Entries))
			for _, e := range g.Entries {
				entries = append(entries, v1alpha2.DNSRecordEntry{
					FQDN:        e.FQDN,
					Group:       g.Name,
					Description: e.Description,
					RecordType:  "A",
				})
			}
			record := &v1alpha2.DNSRecord{
				ObjectMeta: metav1.ObjectMeta{Name: recordName, Namespace: dns.Namespace},
				Spec: v1alpha2.DNSRecordSpec{
					Origin:    v1alpha2.DNSRecordOriginManual,
					PortalRef: dns.Spec.PortalRef,
					Entries:   entries,
				},
			}
			opts := []client.CreateOption{}
			if dryRun {
				opts = append(opts, client.DryRunAll)
			}
			if err := c.Create(ctx, record, opts...); err != nil {
				if apierrors.IsAlreadyExists(err) {
					sum.AlreadyExist++
					fmt.Printf("DNSRecord %s/%s already exists, leaving in place\n", dns.Namespace, recordName)
					continue
				}
				sum.Failures++
				perDNSFailures++
				errAggr = append(errAggr, fmt.Errorf("create %s/%s: %w", dns.Namespace, recordName, err))
				continue
			}
			perDNSCreated++
			sum.Created++
			if dryRun {
				fmt.Printf("[dry-run] would create DNSRecord %s/%s (%d entries)\n", dns.Namespace, recordName, len(entries))
			} else {
				fmt.Printf("created DNSRecord %s/%s\n", dns.Namespace, recordName)
			}
		}

		// Only strip the annotation when every non-empty group succeeded so a
		// retry can pick up any partial failures.
		if !dryRun && perDNSFailures == 0 && perDNSCreated == groupCount && groupCount > 0 {
			patch := client.MergeFrom(dns.DeepCopy())
			delete(dns.Annotations, annotationV1Alpha1Groups)
			if err := c.Patch(ctx, dns, patch); err != nil {
				sum.Failures++
				errAggr = append(errAggr, fmt.Errorf("remove annotation %s/%s: %w", dns.Namespace, dns.Name, err))
			}
		}
	}
	if len(errAggr) > 0 {
		return sum, errors.Join(errAggr...)
	}
	return sum, nil
}
