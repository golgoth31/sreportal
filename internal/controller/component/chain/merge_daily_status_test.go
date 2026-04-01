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

package component_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	componentchain "github.com/golgoth31/sreportal/internal/controller/component/chain"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

var _ = Describe("MergeDailyStatusHandler", func() {
	fixedNow := time.Date(2026, 4, 1, 10, 0, 0, 0, time.UTC)

	newRC := func(comp *sreportalv1alpha1.Component, status sreportalv1alpha1.ComputedComponentStatus) *reconciler.ReconcileContext[*sreportalv1alpha1.Component, componentchain.ChainData] {
		return &reconciler.ReconcileContext[*sreportalv1alpha1.Component, componentchain.ChainData]{
			Resource: comp,
			Data:     componentchain.ChainData{ComputedStatus: status},
		}
	}

	Context("when history is empty", func() {
		It("should backfill 30 days and set today's status", func() {
			comp := &sreportalv1alpha1.Component{}
			rc := newRC(comp, sreportalv1alpha1.ComputedStatusDegraded)
			handler := componentchain.NewMergeDailyStatusHandler(func() time.Time { return fixedNow })

			Expect(handler.Handle(context.Background(), rc)).To(Succeed())

			Expect(comp.Status.DailyWorstStatus).To(HaveLen(30))
			// First entry is 29 days ago
			Expect(comp.Status.DailyWorstStatus[0].Date).To(Equal("2026-03-03"))
			// Last entry is today with computed status
			last := comp.Status.DailyWorstStatus[29]
			Expect(last.Date).To(Equal("2026-04-01"))
			Expect(last.WorstStatus).To(Equal(sreportalv1alpha1.ComputedStatusDegraded))
			// All backfilled days are operational
			for _, entry := range comp.Status.DailyWorstStatus[:29] {
				Expect(entry.WorstStatus).To(Equal(sreportalv1alpha1.ComputedStatusOperational))
			}
		})
	})

	Context("when merging worst status for the same day", func() {
		It("should keep the worst status", func() {
			comp := &sreportalv1alpha1.Component{}
			comp.Status.DailyWorstStatus = []sreportalv1alpha1.DailyComponentStatus{
				{Date: "2026-04-01", WorstStatus: sreportalv1alpha1.ComputedStatusDegraded},
			}
			rc := newRC(comp, sreportalv1alpha1.ComputedStatusMajorOutage)
			handler := componentchain.NewMergeDailyStatusHandler(func() time.Time { return fixedNow })

			Expect(handler.Handle(context.Background(), rc)).To(Succeed())

			Expect(comp.Status.DailyWorstStatus).To(HaveLen(30))
			last := comp.Status.DailyWorstStatus[29]
			Expect(last.WorstStatus).To(Equal(sreportalv1alpha1.ComputedStatusMajorOutage))
		})
	})

	Context("when handler finishes", func() {
		It("should not set RequeueAfter", func() {
			comp := &sreportalv1alpha1.Component{}
			rc := newRC(comp, sreportalv1alpha1.ComputedStatusOperational)
			handler := componentchain.NewMergeDailyStatusHandler(func() time.Time { return fixedNow })

			_ = handler.Handle(context.Background(), rc)

			Expect(rc.Result.RequeueAfter).To(BeZero())
		})
	})
})
