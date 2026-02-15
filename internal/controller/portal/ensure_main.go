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

package portal

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
)

const (
	// MainPortalName is the name of the default main portal
	MainPortalName = "main"
	// MainPortalTitle is the display title of the default main portal
	MainPortalTitle = "Main Portal"
)

// EnsureMainPortalRunnable creates a manager.Runnable that ensures a main portal
// exists at startup. If no portal has spec.main=true, it creates one.
type EnsureMainPortalRunnable struct {
	client    client.Client
	namespace string
}

// NewEnsureMainPortalRunnable creates a new EnsureMainPortalRunnable.
func NewEnsureMainPortalRunnable(c client.Client, namespace string) *EnsureMainPortalRunnable {
	return &EnsureMainPortalRunnable{
		client:    c,
		namespace: namespace,
	}
}

// Start implements manager.Runnable. It runs once at startup to ensure the main portal exists.
func (r *EnsureMainPortalRunnable) Start(ctx context.Context) error {
	log := ctrl.Log.WithName("ensure-main-portal")

	// List all portals
	var portalList sreportalv1alpha1.PortalList
	if err := r.client.List(ctx, &portalList, client.InNamespace(r.namespace)); err != nil {
		log.Error(err, "failed to list portals")
		return err
	}

	// Check if any portal has main=true
	for _, p := range portalList.Items {
		if p.Spec.Main {
			log.Info("main portal already exists", "name", p.Name, "namespace", p.Namespace)
			return nil
		}
	}

	// No main portal found, create one
	mainPortal := &sreportalv1alpha1.Portal{
		ObjectMeta: metav1.ObjectMeta{
			Name:      MainPortalName,
			Namespace: r.namespace,
		},
		Spec: sreportalv1alpha1.PortalSpec{
			Title: MainPortalTitle,
			Main:  true,
		},
	}

	if err := r.client.Create(ctx, mainPortal); err != nil {
		log.Error(err, "failed to create main portal")
		return err
	}

	log.Info("created main portal", "name", MainPortalName, "namespace", r.namespace)
	return nil
}

// NeedLeaderElection returns true so this only runs on the leader.
func (r *EnsureMainPortalRunnable) NeedLeaderElection() bool {
	return true
}
