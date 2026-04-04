package auth

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	portalchain "github.com/golgoth31/sreportal/internal/controller/portal/chain"
	releasev1 "github.com/golgoth31/sreportal/internal/grpc/gen/sreportal/v1"
)

func normalizePortalRef(ref string) string {
	if ref == "" {
		return portalchain.MainPortalName
	}
	return ref
}

func (r *PortalResolver) extractPortalRef(ctx context.Context, proc string, msg any) (string, error) {
	switch proc {
	case "/sreportal.v1.ReleaseService/AddRelease":
		m, ok := msg.(*releasev1.ReleaseEntry)
		if !ok {
			return portalchain.MainPortalName, nil
		}
		return normalizePortalRef(m.GetPortal()), nil

	case "/sreportal.v1.StatusService/CreateComponent":
		m, ok := msg.(*releasev1.CreateComponentRequest)
		if !ok {
			return portalchain.MainPortalName, nil
		}
		return normalizePortalRef(m.GetPortalRef()), nil

	case "/sreportal.v1.StatusService/UpdateComponent":
		m, ok := msg.(*releasev1.UpdateComponentRequest)
		if !ok {
			return portalchain.MainPortalName, nil
		}
		return r.portalRefFromComponent(ctx, m.GetName())

	case "/sreportal.v1.StatusService/DeleteComponent":
		m, ok := msg.(*releasev1.DeleteComponentRequest)
		if !ok {
			return portalchain.MainPortalName, nil
		}
		return r.portalRefFromComponent(ctx, m.GetName())

	case "/sreportal.v1.StatusService/CreateMaintenance":
		m, ok := msg.(*releasev1.CreateMaintenanceRequest)
		if !ok {
			return portalchain.MainPortalName, nil
		}
		return normalizePortalRef(m.GetPortalRef()), nil

	case "/sreportal.v1.StatusService/UpdateMaintenance":
		m, ok := msg.(*releasev1.UpdateMaintenanceRequest)
		if !ok {
			return portalchain.MainPortalName, nil
		}
		return r.portalRefFromMaintenance(ctx, m.GetName())

	case "/sreportal.v1.StatusService/DeleteMaintenance":
		m, ok := msg.(*releasev1.DeleteMaintenanceRequest)
		if !ok {
			return portalchain.MainPortalName, nil
		}
		return r.portalRefFromMaintenance(ctx, m.GetName())

	case "/sreportal.v1.StatusService/CreateIncident":
		m, ok := msg.(*releasev1.CreateIncidentRequest)
		if !ok {
			return portalchain.MainPortalName, nil
		}
		return normalizePortalRef(m.GetPortalRef()), nil

	case "/sreportal.v1.StatusService/UpdateIncident":
		m, ok := msg.(*releasev1.UpdateIncidentRequest)
		if !ok {
			return portalchain.MainPortalName, nil
		}
		return r.portalRefFromIncident(ctx, m.GetName())

	case "/sreportal.v1.StatusService/DeleteIncident":
		m, ok := msg.(*releasev1.DeleteIncidentRequest)
		if !ok {
			return portalchain.MainPortalName, nil
		}
		return r.portalRefFromIncident(ctx, m.GetName())

	default:
		return portalchain.MainPortalName, nil
	}
}

func (r *PortalResolver) portalRefFromComponent(ctx context.Context, name string) (string, error) {
	var comp sreportalv1alpha1.Component
	err := r.client.Get(ctx, types.NamespacedName{Namespace: r.portalNamespace, Name: name}, &comp)
	if apierrors.IsNotFound(err) {
		return portalchain.MainPortalName, nil
	}
	if err != nil {
		return "", fmt.Errorf("auth: get component: %w", err)
	}
	return normalizePortalRef(comp.Spec.PortalRef), nil
}

func (r *PortalResolver) portalRefFromMaintenance(ctx context.Context, name string) (string, error) {
	var m sreportalv1alpha1.Maintenance
	err := r.client.Get(ctx, types.NamespacedName{Namespace: r.portalNamespace, Name: name}, &m)
	if apierrors.IsNotFound(err) {
		return portalchain.MainPortalName, nil
	}
	if err != nil {
		return "", fmt.Errorf("auth: get maintenance: %w", err)
	}
	return normalizePortalRef(m.Spec.PortalRef), nil
}

func (r *PortalResolver) portalRefFromIncident(ctx context.Context, name string) (string, error) {
	var inc sreportalv1alpha1.Incident
	err := r.client.Get(ctx, types.NamespacedName{Namespace: r.portalNamespace, Name: name}, &inc)
	if apierrors.IsNotFound(err) {
		return portalchain.MainPortalName, nil
	}
	if err != nil {
		return "", fmt.Errorf("auth: get incident: %w", err)
	}
	return normalizePortalRef(inc.Spec.PortalRef), nil
}
