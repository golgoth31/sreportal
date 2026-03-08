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

package alertmanager_test

import (
	"context"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	alertmanagerctrl "github.com/golgoth31/sreportal/internal/controller/alertmanager"
	domainalertmanager "github.com/golgoth31/sreportal/internal/domain/alertmanager"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

type fakeFetcher struct {
	alerts []domainalertmanager.Alert
	err    error
}

func (f *fakeFetcher) GetActiveAlerts(_ context.Context, _ string) ([]domainalertmanager.Alert, error) {
	return f.alerts, f.err
}

var _ = Describe("FetchAlertsHandler", func() {
	newRC := func(isRemote bool) *reconciler.ReconcileContext[*sreportalv1alpha1.Alertmanager] {
		return &reconciler.ReconcileContext[*sreportalv1alpha1.Alertmanager]{
			Resource: &sreportalv1alpha1.Alertmanager{
				ObjectMeta: metav1.ObjectMeta{Name: "test-am", Namespace: "default"},
				Spec: sreportalv1alpha1.AlertmanagerSpec{
					PortalRef: "main",
					URL: sreportalv1alpha1.AlertmanagerURL{
						Local: "http://alertmanager:9093",
					},
					IsRemote: isRemote,
				},
			},
			Data: make(map[string]any),
		}
	}

	Context("when fetcher returns alerts (local)", func() {
		It("should store alerts in context data", func() {
			alerts := []domainalertmanager.Alert{
				{Fingerprint: "aaa", Labels: map[string]string{"alertname": "HighCPU"}, State: domainalertmanager.StateActive},
				{Fingerprint: "bbb", Labels: map[string]string{"alertname": "DiskFull"}, State: domainalertmanager.StateActive},
			}
			localFetcher := &fakeFetcher{alerts: alerts}
			remoteFetcher := &fakeFetcher{}
			handler := alertmanagerctrl.NewFetchAlertsHandler(localFetcher, remoteFetcher)
			rc := newRC(false)

			Expect(handler.Handle(context.Background(), rc)).To(Succeed())

			stored, ok := rc.Data[alertmanagerctrl.DataKeyAlerts].([]domainalertmanager.Alert)
			Expect(ok).To(BeTrue())
			Expect(stored).To(HaveLen(2))
			Expect(stored[0].Fingerprint).To(Equal("aaa"))
			Expect(stored[1].Fingerprint).To(Equal("bbb"))
		})
	})

	Context("when fetcher returns empty list", func() {
		It("should store empty slice in context data", func() {
			localFetcher := &fakeFetcher{alerts: []domainalertmanager.Alert{}}
			remoteFetcher := &fakeFetcher{}
			handler := alertmanagerctrl.NewFetchAlertsHandler(localFetcher, remoteFetcher)
			rc := newRC(false)

			Expect(handler.Handle(context.Background(), rc)).To(Succeed())

			stored, ok := rc.Data[alertmanagerctrl.DataKeyAlerts].([]domainalertmanager.Alert)
			Expect(ok).To(BeTrue())
			Expect(stored).To(BeEmpty())
		})
	})

	Context("when fetcher returns an error", func() {
		It("should propagate the error", func() {
			localFetcher := &fakeFetcher{err: errors.New("connection refused")}
			remoteFetcher := &fakeFetcher{}
			handler := alertmanagerctrl.NewFetchAlertsHandler(localFetcher, remoteFetcher)
			rc := newRC(false)

			err := handler.Handle(context.Background(), rc)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("connection refused"))
		})
	})

	Context("when isRemote is true", func() {
		It("should use remote fetcher instead of local", func() {
			remoteAlerts := []domainalertmanager.Alert{
				{Fingerprint: "remote-1", Labels: map[string]string{"alertname": "RemoteAlert"}, State: domainalertmanager.StateActive},
			}
			localFetcher := &fakeFetcher{err: errors.New("should not be called")}
			remoteFetcher := &fakeFetcher{alerts: remoteAlerts}
			handler := alertmanagerctrl.NewFetchAlertsHandler(localFetcher, remoteFetcher)
			rc := newRC(true)

			Expect(handler.Handle(context.Background(), rc)).To(Succeed())

			stored, ok := rc.Data[alertmanagerctrl.DataKeyAlerts].([]domainalertmanager.Alert)
			Expect(ok).To(BeTrue())
			Expect(stored).To(HaveLen(1))
			Expect(stored[0].Fingerprint).To(Equal("remote-1"))
		})

		It("should propagate remote fetcher errors", func() {
			localFetcher := &fakeFetcher{}
			remoteFetcher := &fakeFetcher{err: errors.New("remote unreachable")}
			handler := alertmanagerctrl.NewFetchAlertsHandler(localFetcher, remoteFetcher)
			rc := newRC(true)

			err := handler.Handle(context.Background(), rc)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("remote unreachable"))
		})
	})
})
