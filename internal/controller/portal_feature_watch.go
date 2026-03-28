/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"

	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	"github.com/golgoth31/sreportal/internal/log"
)

// PortalFeatureWakeupPredicate matches Portal create/update when a feature becomes
// effectively enabled (create with feature on, or update from disabled to enabled).
// Delete and Generic events are ignored.
func PortalFeatureWakeupPredicate(isFeatureOn func(*sreportalv1alpha1.Portal) bool) predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			p, ok := e.Object.(*sreportalv1alpha1.Portal)
			if !ok || p == nil {
				return false
			}
			return isFeatureOn(p)
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldP, ok1 := e.ObjectOld.(*sreportalv1alpha1.Portal)
			newP, ok2 := e.ObjectNew.(*sreportalv1alpha1.Portal)
			if !ok1 || !ok2 || oldP == nil || newP == nil {
				return false
			}
			return !isFeatureOn(oldP) && isFeatureOn(newP)
		},
		DeleteFunc: func(event.DeleteEvent) bool {
			return false
		},
		GenericFunc: func(event.GenericEvent) bool {
			return false
		},
	}
}

// PortalReleasesFeatureWakeupPredicate matches Portal events that require
// re-projecting Release CRs when the releases feature turns on.
func PortalReleasesFeatureWakeupPredicate() predicate.Predicate {
	return PortalFeatureWakeupPredicate(func(p *sreportalv1alpha1.Portal) bool {
		return p.Spec.Features.IsReleasesEnabled()
	})
}

// PortalDNSEnabledWakeupPredicate matches Portal events that require reconciling
// DNS / DNSRecord resources when the DNS feature turns on.
func PortalDNSEnabledWakeupPredicate() predicate.Predicate {
	return PortalFeatureWakeupPredicate(func(p *sreportalv1alpha1.Portal) bool {
		return p.Spec.Features.IsDNSEnabled()
	})
}

// PortalAlertsEnabledWakeupPredicate matches Portal events that require reconciling
// Alertmanager resources when the alerts feature turns on.
func PortalAlertsEnabledWakeupPredicate() predicate.Predicate {
	return PortalFeatureWakeupPredicate(func(p *sreportalv1alpha1.Portal) bool {
		return p.Spec.Features.IsAlertsEnabled()
	})
}

// PortalNetworkPolicyEnabledWakeupPredicate matches Portal events that require
// reconciling NetworkFlowDiscovery when the networkPolicy feature turns on.
func PortalNetworkPolicyEnabledWakeupPredicate() predicate.Predicate {
	return PortalFeatureWakeupPredicate(func(p *sreportalv1alpha1.Portal) bool {
		return p.Spec.Features.IsNetworkPolicyEnabled()
	})
}

// PortalStatusPageEnabledWakeupPredicate matches Portal events that require
// reconciling status-page CRs when the statusPage feature turns on.
func PortalStatusPageEnabledWakeupPredicate() predicate.Predicate {
	return PortalFeatureWakeupPredicate(func(p *sreportalv1alpha1.Portal) bool {
		return p.Spec.Features.IsStatusPageEnabled()
	})
}

func enqueueOnPortalFeature(
	ctx context.Context,
	c client.Client,
	portal *sreportalv1alpha1.Portal,
	featureOn func(*sreportalv1alpha1.Portal) bool,
	listKind string,
	listByPortal func(context.Context, client.Client, string, string) ([]ctrl.Request, error),
) []ctrl.Request {
	if portal == nil || !featureOn(portal) {
		return nil
	}
	reqs, err := listByPortal(ctx, c, portal.Namespace, portal.Name)
	if err != nil {
		log.FromContext(ctx).Error(err, "list resources for Portal watch", "portal", portal.Name, "kind", listKind)
		return nil
	}
	return reqs
}

func listReleasesForPortal(ctx context.Context, c client.Client, ns, portalName string) ([]ctrl.Request, error) {
	var list sreportalv1alpha1.ReleaseList
	if err := c.List(ctx, &list,
		client.InNamespace(ns),
		client.MatchingFields{FieldIndexPortalRef: portalName},
	); err != nil {
		return nil, err
	}
	out := make([]ctrl.Request, 0, len(list.Items))
	for i := range list.Items {
		out = append(out, ctrl.Request{NamespacedName: types.NamespacedName{
			Namespace: list.Items[i].Namespace,
			Name:      list.Items[i].Name,
		}})
	}
	return out, nil
}

func listDNSForPortal(ctx context.Context, c client.Client, ns, portalName string) ([]ctrl.Request, error) {
	var list sreportalv1alpha1.DNSList
	if err := c.List(ctx, &list,
		client.InNamespace(ns),
		client.MatchingFields{FieldIndexPortalRef: portalName},
	); err != nil {
		return nil, err
	}
	out := make([]ctrl.Request, 0, len(list.Items))
	for i := range list.Items {
		out = append(out, ctrl.Request{NamespacedName: types.NamespacedName{
			Namespace: list.Items[i].Namespace,
			Name:      list.Items[i].Name,
		}})
	}
	return out, nil
}

func listDNSRecordsForPortal(ctx context.Context, c client.Client, ns, portalName string) ([]ctrl.Request, error) {
	var list sreportalv1alpha1.DNSRecordList
	if err := c.List(ctx, &list,
		client.InNamespace(ns),
		client.MatchingFields{FieldIndexPortalRef: portalName},
	); err != nil {
		return nil, err
	}
	out := make([]ctrl.Request, 0, len(list.Items))
	for i := range list.Items {
		out = append(out, ctrl.Request{NamespacedName: types.NamespacedName{
			Namespace: list.Items[i].Namespace,
			Name:      list.Items[i].Name,
		}})
	}
	return out, nil
}

func listAlertmanagersForPortal(ctx context.Context, c client.Client, ns, portalName string) ([]ctrl.Request, error) {
	var list sreportalv1alpha1.AlertmanagerList
	if err := c.List(ctx, &list,
		client.InNamespace(ns),
		client.MatchingFields{FieldIndexPortalRef: portalName},
	); err != nil {
		return nil, err
	}
	out := make([]ctrl.Request, 0, len(list.Items))
	for i := range list.Items {
		out = append(out, ctrl.Request{NamespacedName: types.NamespacedName{
			Namespace: list.Items[i].Namespace,
			Name:      list.Items[i].Name,
		}})
	}
	return out, nil
}

func listNetworkFlowDiscoveriesForPortal(ctx context.Context, c client.Client, ns, portalName string) ([]ctrl.Request, error) {
	var list sreportalv1alpha1.NetworkFlowDiscoveryList
	if err := c.List(ctx, &list,
		client.InNamespace(ns),
		client.MatchingFields{FieldIndexPortalRef: portalName},
	); err != nil {
		return nil, err
	}
	out := make([]ctrl.Request, 0, len(list.Items))
	for i := range list.Items {
		out = append(out, ctrl.Request{NamespacedName: types.NamespacedName{
			Namespace: list.Items[i].Namespace,
			Name:      list.Items[i].Name,
		}})
	}
	return out, nil
}

func listComponentsForPortal(ctx context.Context, c client.Client, ns, portalName string) ([]ctrl.Request, error) {
	var list sreportalv1alpha1.ComponentList
	if err := c.List(ctx, &list,
		client.InNamespace(ns),
		client.MatchingFields{FieldIndexPortalRef: portalName},
	); err != nil {
		return nil, err
	}
	out := make([]ctrl.Request, 0, len(list.Items))
	for i := range list.Items {
		out = append(out, ctrl.Request{NamespacedName: types.NamespacedName{
			Namespace: list.Items[i].Namespace,
			Name:      list.Items[i].Name,
		}})
	}
	return out, nil
}

func listIncidentsForPortal(ctx context.Context, c client.Client, ns, portalName string) ([]ctrl.Request, error) {
	var list sreportalv1alpha1.IncidentList
	if err := c.List(ctx, &list,
		client.InNamespace(ns),
		client.MatchingFields{FieldIndexPortalRef: portalName},
	); err != nil {
		return nil, err
	}
	out := make([]ctrl.Request, 0, len(list.Items))
	for i := range list.Items {
		out = append(out, ctrl.Request{NamespacedName: types.NamespacedName{
			Namespace: list.Items[i].Namespace,
			Name:      list.Items[i].Name,
		}})
	}
	return out, nil
}

func listMaintenancesForPortal(ctx context.Context, c client.Client, ns, portalName string) ([]ctrl.Request, error) {
	var list sreportalv1alpha1.MaintenanceList
	if err := c.List(ctx, &list,
		client.InNamespace(ns),
		client.MatchingFields{FieldIndexPortalRef: portalName},
	); err != nil {
		return nil, err
	}
	out := make([]ctrl.Request, 0, len(list.Items))
	for i := range list.Items {
		out = append(out, ctrl.Request{NamespacedName: types.NamespacedName{
			Namespace: list.Items[i].Namespace,
			Name:      list.Items[i].Name,
		}})
	}
	return out, nil
}

// releaseReconcileRequestsForPortal enqueues all Release CRs referencing the portal
// when the releases feature is enabled on that portal.
func releaseReconcileRequestsForPortal(ctx context.Context, c client.Client, portal *sreportalv1alpha1.Portal) []ctrl.Request {
	return enqueueOnPortalFeature(ctx, c, portal, func(p *sreportalv1alpha1.Portal) bool {
		return p.Spec.Features.IsReleasesEnabled()
	}, "Release", listReleasesForPortal)
}

// dnsReconcileRequestsForPortal enqueues DNS CRs for the portal when DNS is enabled.
func dnsReconcileRequestsForPortal(ctx context.Context, c client.Client, portal *sreportalv1alpha1.Portal) []ctrl.Request {
	return enqueueOnPortalFeature(ctx, c, portal, func(p *sreportalv1alpha1.Portal) bool {
		return p.Spec.Features.IsDNSEnabled()
	}, "DNS", listDNSForPortal)
}

// dnsRecordReconcileRequestsForPortal enqueues DNSRecord CRs for the portal when DNS is enabled.
func dnsRecordReconcileRequestsForPortal(ctx context.Context, c client.Client, portal *sreportalv1alpha1.Portal) []ctrl.Request {
	return enqueueOnPortalFeature(ctx, c, portal, func(p *sreportalv1alpha1.Portal) bool {
		return p.Spec.Features.IsDNSEnabled()
	}, "DNSRecord", listDNSRecordsForPortal)
}

// alertmanagerReconcileRequestsForPortal enqueues Alertmanager CRs when alerts are enabled.
func alertmanagerReconcileRequestsForPortal(ctx context.Context, c client.Client, portal *sreportalv1alpha1.Portal) []ctrl.Request {
	return enqueueOnPortalFeature(ctx, c, portal, func(p *sreportalv1alpha1.Portal) bool {
		return p.Spec.Features.IsAlertsEnabled()
	}, "Alertmanager", listAlertmanagersForPortal)
}

// networkFlowDiscoveryReconcileRequestsForPortal enqueues NFD CRs when networkPolicy is enabled.
func networkFlowDiscoveryReconcileRequestsForPortal(ctx context.Context, c client.Client, portal *sreportalv1alpha1.Portal) []ctrl.Request {
	return enqueueOnPortalFeature(ctx, c, portal, func(p *sreportalv1alpha1.Portal) bool {
		return p.Spec.Features.IsNetworkPolicyEnabled()
	}, "NetworkFlowDiscovery", listNetworkFlowDiscoveriesForPortal)
}

// componentReconcileRequestsForPortal enqueues Component CRs when statusPage is enabled.
func componentReconcileRequestsForPortal(ctx context.Context, c client.Client, portal *sreportalv1alpha1.Portal) []ctrl.Request {
	return enqueueOnPortalFeature(ctx, c, portal, func(p *sreportalv1alpha1.Portal) bool {
		return p.Spec.Features.IsStatusPageEnabled()
	}, "Component", listComponentsForPortal)
}

// incidentReconcileRequestsForPortal enqueues Incident CRs when statusPage is enabled.
func incidentReconcileRequestsForPortal(ctx context.Context, c client.Client, portal *sreportalv1alpha1.Portal) []ctrl.Request {
	return enqueueOnPortalFeature(ctx, c, portal, func(p *sreportalv1alpha1.Portal) bool {
		return p.Spec.Features.IsStatusPageEnabled()
	}, "Incident", listIncidentsForPortal)
}

// maintenanceReconcileRequestsForPortal enqueues Maintenance CRs when statusPage is enabled.
func maintenanceReconcileRequestsForPortal(ctx context.Context, c client.Client, portal *sreportalv1alpha1.Portal) []ctrl.Request {
	return enqueueOnPortalFeature(ctx, c, portal, func(p *sreportalv1alpha1.Portal) bool {
		return p.Spec.Features.IsStatusPageEnabled()
	}, "Maintenance", listMaintenancesForPortal)
}
