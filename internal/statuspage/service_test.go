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

package statuspage_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	"github.com/golgoth31/sreportal/internal/statuspage"
)

const testNamespace = "default"

func testScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = sreportalv1alpha1.AddToScheme(s)
	return s
}

func newTestService(objects ...runtime.Object) *statuspage.Service {
	c := fake.NewClientBuilder().
		WithScheme(testScheme()).
		WithRuntimeObjects(objects...).
		Build()
	return statuspage.NewService(c, testNamespace)
}

func makeIncidentCR(updates []sreportalv1alpha1.IncidentUpdate) *sreportalv1alpha1.Incident {
	return &sreportalv1alpha1.Incident{
		ObjectMeta: metav1.ObjectMeta{Name: "inc-1", Namespace: testNamespace},
		Spec: sreportalv1alpha1.IncidentSpec{
			Title:     "existing incident",
			PortalRef: "main",
			Severity:  sreportalv1alpha1.IncidentSeverityCritical,
			Updates:   updates,
		},
	}
}

// --- CreateComponent ---

func makeComponentCR() *sreportalv1alpha1.Component {
	return &sreportalv1alpha1.Component{
		ObjectMeta: metav1.ObjectMeta{Name: "comp-1", Namespace: testNamespace},
		Spec: sreportalv1alpha1.ComponentSpec{
			DisplayName: "API Gateway",
			Description: "original desc",
			Group:       "Infrastructure",
			Link:        "https://example.com",
			PortalRef:   "main",
			Status:      sreportalv1alpha1.ComponentStatusOperational,
		},
	}
}

func TestCreateComponent_Success(t *testing.T) {
	c := fake.NewClientBuilder().WithScheme(testScheme()).Build()
	svc := statuspage.NewService(c, testNamespace)

	in := statuspage.CreateComponentInput{
		DisplayName: "API Gateway",
		Description: "Main API",
		Group:       "Infrastructure",
		Link:        "https://example.com",
		PortalRef:   "main",
		Status:      sreportalv1alpha1.ComponentStatusOperational,
	}

	name, err := svc.CreateComponent(context.Background(), in)
	require.NoError(t, err)
	assert.NotEmpty(t, name)

	var comp sreportalv1alpha1.Component
	err = c.Get(context.Background(), types.NamespacedName{Name: name, Namespace: testNamespace}, &comp)
	require.NoError(t, err)
	assert.Equal(t, "API Gateway", comp.Spec.DisplayName)
	assert.Equal(t, "Infrastructure", comp.Spec.Group)
	assert.Equal(t, "main", comp.Spec.PortalRef)
}

func TestCreateComponent_AutoGeneratesName(t *testing.T) {
	c := fake.NewClientBuilder().WithScheme(testScheme()).Build()
	svc := statuspage.NewService(c, testNamespace)

	in := statuspage.CreateComponentInput{
		DisplayName: "API Gateway",
		Group:       "Infrastructure",
		PortalRef:   "main",
		Status:      sreportalv1alpha1.ComponentStatusOperational,
	}

	name, err := svc.CreateComponent(context.Background(), in)
	require.NoError(t, err)
	assert.NotEmpty(t, name)
	assert.LessOrEqual(t, len(name), 63)

	// Verify CR exists with the generated name
	var comp sreportalv1alpha1.Component
	err = c.Get(context.Background(), types.NamespacedName{Name: name, Namespace: testNamespace}, &comp)
	require.NoError(t, err)
	assert.Equal(t, "API Gateway", comp.Spec.DisplayName)
}

func TestCreateComponent_AlreadyExists(t *testing.T) {
	// Pre-create a CR whose name matches GenerateCRName("main", "API Gateway")
	generatedName := statuspage.GenerateCRName("main", "API Gateway")
	existing := &sreportalv1alpha1.Component{
		ObjectMeta: metav1.ObjectMeta{Name: generatedName, Namespace: testNamespace},
		Spec: sreportalv1alpha1.ComponentSpec{
			DisplayName: "API Gateway",
			Group:       "Infrastructure",
			PortalRef:   "main",
			Status:      sreportalv1alpha1.ComponentStatusOperational,
		},
	}
	svc := newTestService(existing)

	in := statuspage.CreateComponentInput{
		DisplayName: "API Gateway",
		Group:       "Infrastructure",
		PortalRef:   "main",
		Status:      sreportalv1alpha1.ComponentStatusOperational,
	}

	_, err := svc.CreateComponent(context.Background(), in)
	require.ErrorIs(t, err, statuspage.ErrAlreadyExists)
}

func TestCreateComponent_MissingPortalRef(t *testing.T) {
	svc := newTestService()
	in := statuspage.CreateComponentInput{DisplayName: "x", Group: "x"}
	_, err := svc.CreateComponent(context.Background(), in)
	require.ErrorIs(t, err, statuspage.ErrPortalRefRequired)
}

func TestCreateComponent_MissingGroup(t *testing.T) {
	svc := newTestService()
	in := statuspage.CreateComponentInput{DisplayName: "x", PortalRef: "main"}
	_, err := svc.CreateComponent(context.Background(), in)
	require.ErrorIs(t, err, statuspage.ErrGroupRequired)
}

// --- UpdateComponent ---

func TestUpdateComponent_UpdatesDisplayName(t *testing.T) {
	existing := makeComponentCR()
	c := fake.NewClientBuilder().WithScheme(testScheme()).WithRuntimeObjects(existing).Build()
	svc := statuspage.NewService(c, testNamespace)

	newName := "Updated Gateway"
	in := statuspage.UpdateComponentInput{
		Name:        "comp-1",
		DisplayName: &newName,
	}

	_, err := svc.UpdateComponent(context.Background(), in)
	require.NoError(t, err)

	var comp sreportalv1alpha1.Component
	err = c.Get(context.Background(), types.NamespacedName{Name: "comp-1", Namespace: testNamespace}, &comp)
	require.NoError(t, err)
	assert.Equal(t, "Updated Gateway", comp.Spec.DisplayName)
}

func TestUpdateComponent_UpdatesStatus(t *testing.T) {
	existing := makeComponentCR()
	c := fake.NewClientBuilder().WithScheme(testScheme()).WithRuntimeObjects(existing).Build()
	svc := statuspage.NewService(c, testNamespace)

	newStatus := sreportalv1alpha1.ComponentStatusDegraded
	in := statuspage.UpdateComponentInput{
		Name:   "comp-1",
		Status: &newStatus,
	}

	_, err := svc.UpdateComponent(context.Background(), in)
	require.NoError(t, err)

	var comp sreportalv1alpha1.Component
	err = c.Get(context.Background(), types.NamespacedName{Name: "comp-1", Namespace: testNamespace}, &comp)
	require.NoError(t, err)
	assert.Equal(t, sreportalv1alpha1.ComponentStatusDegraded, comp.Spec.Status)
}

func TestUpdateComponent_LeavesFieldsUnchanged(t *testing.T) {
	existing := makeComponentCR()
	c := fake.NewClientBuilder().WithScheme(testScheme()).WithRuntimeObjects(existing).Build()
	svc := statuspage.NewService(c, testNamespace)

	newDesc := "updated desc"
	in := statuspage.UpdateComponentInput{
		Name:        "comp-1",
		Description: &newDesc,
	}

	_, err := svc.UpdateComponent(context.Background(), in)
	require.NoError(t, err)

	var comp sreportalv1alpha1.Component
	err = c.Get(context.Background(), types.NamespacedName{Name: "comp-1", Namespace: testNamespace}, &comp)
	require.NoError(t, err)
	assert.Equal(t, "API Gateway", comp.Spec.DisplayName)
	assert.Equal(t, "updated desc", comp.Spec.Description)
	assert.Equal(t, "Infrastructure", comp.Spec.Group)
	assert.Equal(t, "main", comp.Spec.PortalRef)
	assert.Equal(t, sreportalv1alpha1.ComponentStatusOperational, comp.Spec.Status)
}

func TestUpdateComponent_NotFound(t *testing.T) {
	svc := newTestService()
	in := statuspage.UpdateComponentInput{Name: "nonexistent"}
	_, err := svc.UpdateComponent(context.Background(), in)
	require.ErrorIs(t, err, statuspage.ErrNotFound)
}

func TestUpdateComponent_MissingName(t *testing.T) {
	svc := newTestService()
	in := statuspage.UpdateComponentInput{}
	_, err := svc.UpdateComponent(context.Background(), in)
	require.ErrorIs(t, err, statuspage.ErrNameRequired)
}

// --- Auto-generated names ---

func TestCreateMaintenance_AutoGeneratesName(t *testing.T) {
	c := fake.NewClientBuilder().WithScheme(testScheme()).Build()
	svc := statuspage.NewService(c, testNamespace)

	in := statuspage.CreateMaintenanceInput{
		Title:          "DB upgrade",
		PortalRef:      "main",
		ScheduledStart: metav1.NewTime(time.Date(2026, 4, 1, 6, 0, 0, 0, time.UTC)),
		ScheduledEnd:   metav1.NewTime(time.Date(2026, 4, 1, 8, 0, 0, 0, time.UTC)),
	}

	name, err := svc.CreateMaintenance(context.Background(), in)
	require.NoError(t, err)
	assert.NotEmpty(t, name)
	assert.LessOrEqual(t, len(name), 63)

	var maint sreportalv1alpha1.Maintenance
	err = c.Get(context.Background(), types.NamespacedName{Name: name, Namespace: testNamespace}, &maint)
	require.NoError(t, err)
	assert.Equal(t, "DB upgrade", maint.Spec.Title)
}

func TestCreateIncident_AutoGeneratesName(t *testing.T) {
	c := fake.NewClientBuilder().WithScheme(testScheme()).Build()
	svc := statuspage.NewService(c, testNamespace)

	in := statuspage.CreateIncidentInput{
		Title:     "API down",
		PortalRef: "main",
		Severity:  sreportalv1alpha1.IncidentSeverityCritical,
		InitialUpdate: sreportalv1alpha1.IncidentUpdate{
			Timestamp: metav1.Now(),
			Phase:     sreportalv1alpha1.IncidentPhaseInvestigating,
			Message:   "Investigating",
		},
	}

	name, err := svc.CreateIncident(context.Background(), in)
	require.NoError(t, err)
	assert.NotEmpty(t, name)
	assert.LessOrEqual(t, len(name), 63)

	var inc sreportalv1alpha1.Incident
	err = c.Get(context.Background(), types.NamespacedName{Name: name, Namespace: testNamespace}, &inc)
	require.NoError(t, err)
	assert.Equal(t, "API down", inc.Spec.Title)
}

// --- CreateMaintenance ---

func makeMaintenanceCR() *sreportalv1alpha1.Maintenance {
	return &sreportalv1alpha1.Maintenance{
		ObjectMeta: metav1.ObjectMeta{Name: "maint-1", Namespace: testNamespace},
		Spec: sreportalv1alpha1.MaintenanceSpec{
			Title:          "existing maintenance",
			PortalRef:      "main",
			Description:    "original desc",
			Components:     []string{"api"},
			ScheduledStart: metav1.NewTime(time.Date(2026, 4, 1, 6, 0, 0, 0, time.UTC)),
			ScheduledEnd:   metav1.NewTime(time.Date(2026, 4, 1, 8, 0, 0, 0, time.UTC)),
			AffectedStatus: sreportalv1alpha1.MaintenanceAffectedMaintenance,
		},
	}
}

func TestCreateMaintenance_Success(t *testing.T) {
	c := fake.NewClientBuilder().WithScheme(testScheme()).Build()
	svc := statuspage.NewService(c, testNamespace)

	in := statuspage.CreateMaintenanceInput{
		Title:          "DB upgrade",
		Description:    "Upgrading PostgreSQL",
		PortalRef:      "main",
		Components:     []string{"db"},
		ScheduledStart: metav1.NewTime(time.Date(2026, 4, 1, 6, 0, 0, 0, time.UTC)),
		ScheduledEnd:   metav1.NewTime(time.Date(2026, 4, 1, 8, 0, 0, 0, time.UTC)),
		AffectedStatus: sreportalv1alpha1.MaintenanceAffectedMaintenance,
	}

	name, err := svc.CreateMaintenance(context.Background(), in)
	require.NoError(t, err)
	assert.NotEmpty(t, name)

	var maint sreportalv1alpha1.Maintenance
	err = c.Get(context.Background(), types.NamespacedName{Name: name, Namespace: testNamespace}, &maint)
	require.NoError(t, err)
	assert.Equal(t, "DB upgrade", maint.Spec.Title)
	assert.Equal(t, "Upgrading PostgreSQL", maint.Spec.Description)
	assert.Equal(t, "main", maint.Spec.PortalRef)
	assert.Equal(t, []string{"db"}, maint.Spec.Components)
	assert.Equal(t, sreportalv1alpha1.MaintenanceAffectedMaintenance, maint.Spec.AffectedStatus)
}

func TestCreateMaintenance_DefaultAffectedStatus(t *testing.T) {
	c := fake.NewClientBuilder().WithScheme(testScheme()).Build()
	svc := statuspage.NewService(c, testNamespace)

	in := statuspage.CreateMaintenanceInput{
		Title:          "DB upgrade",
		PortalRef:      "main",
		ScheduledStart: metav1.NewTime(time.Date(2026, 4, 1, 6, 0, 0, 0, time.UTC)),
		ScheduledEnd:   metav1.NewTime(time.Date(2026, 4, 1, 8, 0, 0, 0, time.UTC)),
	}

	name, err := svc.CreateMaintenance(context.Background(), in)
	require.NoError(t, err)

	var maint sreportalv1alpha1.Maintenance
	err = c.Get(context.Background(), types.NamespacedName{Name: name, Namespace: testNamespace}, &maint)
	require.NoError(t, err)
	assert.Equal(t, sreportalv1alpha1.MaintenanceAffectedMaintenance, maint.Spec.AffectedStatus)
}

func TestCreateMaintenance_AlreadyExists(t *testing.T) {
	generatedName := statuspage.GenerateCRName("main", "existing maintenance")
	existing := &sreportalv1alpha1.Maintenance{
		ObjectMeta: metav1.ObjectMeta{Name: generatedName, Namespace: testNamespace},
		Spec: sreportalv1alpha1.MaintenanceSpec{
			Title:     "existing maintenance",
			PortalRef: "main",
		},
	}
	svc := newTestService(existing)

	in := statuspage.CreateMaintenanceInput{
		Title:     "existing maintenance",
		PortalRef: "main",
	}

	_, err := svc.CreateMaintenance(context.Background(), in)
	require.ErrorIs(t, err, statuspage.ErrAlreadyExists)
}

func TestCreateMaintenance_MissingPortalRef(t *testing.T) {
	svc := newTestService()
	in := statuspage.CreateMaintenanceInput{Title: "x"}
	_, err := svc.CreateMaintenance(context.Background(), in)
	require.ErrorIs(t, err, statuspage.ErrPortalRefRequired)
}

func TestCreateMaintenance_MissingTitle(t *testing.T) {
	svc := newTestService()
	in := statuspage.CreateMaintenanceInput{PortalRef: "main"}
	_, err := svc.CreateMaintenance(context.Background(), in)
	require.ErrorIs(t, err, statuspage.ErrTitleRequired)
}

// --- UpdateMaintenance ---

func TestUpdateMaintenance_UpdatesTitle(t *testing.T) {
	existing := makeMaintenanceCR()
	c := fake.NewClientBuilder().WithScheme(testScheme()).WithRuntimeObjects(existing).Build()
	svc := statuspage.NewService(c, testNamespace)

	newTitle := "Updated maintenance"
	in := statuspage.UpdateMaintenanceInput{
		Name:  "maint-1",
		Title: &newTitle,
	}

	_, err := svc.UpdateMaintenance(context.Background(), in)
	require.NoError(t, err)

	var maint sreportalv1alpha1.Maintenance
	err = c.Get(context.Background(), types.NamespacedName{Name: "maint-1", Namespace: testNamespace}, &maint)
	require.NoError(t, err)
	assert.Equal(t, "Updated maintenance", maint.Spec.Title)
}

func TestUpdateMaintenance_UpdatesDescription(t *testing.T) {
	existing := makeMaintenanceCR()
	c := fake.NewClientBuilder().WithScheme(testScheme()).WithRuntimeObjects(existing).Build()
	svc := statuspage.NewService(c, testNamespace)

	newDesc := "new description"
	in := statuspage.UpdateMaintenanceInput{
		Name:        "maint-1",
		Description: &newDesc,
	}

	_, err := svc.UpdateMaintenance(context.Background(), in)
	require.NoError(t, err)

	var maint sreportalv1alpha1.Maintenance
	err = c.Get(context.Background(), types.NamespacedName{Name: "maint-1", Namespace: testNamespace}, &maint)
	require.NoError(t, err)
	assert.Equal(t, "new description", maint.Spec.Description)
}

func TestUpdateMaintenance_UpdatesComponents(t *testing.T) {
	existing := makeMaintenanceCR()
	c := fake.NewClientBuilder().WithScheme(testScheme()).WithRuntimeObjects(existing).Build()
	svc := statuspage.NewService(c, testNamespace)

	in := statuspage.UpdateMaintenanceInput{
		Name:       "maint-1",
		Components: []string{"api", "db"},
	}

	_, err := svc.UpdateMaintenance(context.Background(), in)
	require.NoError(t, err)

	var maint sreportalv1alpha1.Maintenance
	err = c.Get(context.Background(), types.NamespacedName{Name: "maint-1", Namespace: testNamespace}, &maint)
	require.NoError(t, err)
	assert.Equal(t, []string{"api", "db"}, maint.Spec.Components)
}

func TestUpdateMaintenance_UpdatesSchedule(t *testing.T) {
	existing := makeMaintenanceCR()
	c := fake.NewClientBuilder().WithScheme(testScheme()).WithRuntimeObjects(existing).Build()
	svc := statuspage.NewService(c, testNamespace)

	newStart := metav1.NewTime(time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC))
	newEnd := metav1.NewTime(time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC))
	in := statuspage.UpdateMaintenanceInput{
		Name:           "maint-1",
		ScheduledStart: &newStart,
		ScheduledEnd:   &newEnd,
	}

	_, err := svc.UpdateMaintenance(context.Background(), in)
	require.NoError(t, err)

	var maint sreportalv1alpha1.Maintenance
	err = c.Get(context.Background(), types.NamespacedName{Name: "maint-1", Namespace: testNamespace}, &maint)
	require.NoError(t, err)
	assert.True(t, newStart.Time.Equal(maint.Spec.ScheduledStart.Time), "scheduled_start mismatch")
	assert.True(t, newEnd.Time.Equal(maint.Spec.ScheduledEnd.Time), "scheduled_end mismatch")
}

func TestUpdateMaintenance_UpdatesAffectedStatus(t *testing.T) {
	existing := makeMaintenanceCR()
	c := fake.NewClientBuilder().WithScheme(testScheme()).WithRuntimeObjects(existing).Build()
	svc := statuspage.NewService(c, testNamespace)

	newStatus := sreportalv1alpha1.MaintenanceAffectedDegraded
	in := statuspage.UpdateMaintenanceInput{
		Name:           "maint-1",
		AffectedStatus: &newStatus,
	}

	_, err := svc.UpdateMaintenance(context.Background(), in)
	require.NoError(t, err)

	var maint sreportalv1alpha1.Maintenance
	err = c.Get(context.Background(), types.NamespacedName{Name: "maint-1", Namespace: testNamespace}, &maint)
	require.NoError(t, err)
	assert.Equal(t, sreportalv1alpha1.MaintenanceAffectedDegraded, maint.Spec.AffectedStatus)
}

func TestUpdateMaintenance_LeavesFieldsUnchanged(t *testing.T) {
	existing := makeMaintenanceCR()
	c := fake.NewClientBuilder().WithScheme(testScheme()).WithRuntimeObjects(existing).Build()
	svc := statuspage.NewService(c, testNamespace)

	newTitle := "changed title"
	in := statuspage.UpdateMaintenanceInput{
		Name:  "maint-1",
		Title: &newTitle,
	}

	_, err := svc.UpdateMaintenance(context.Background(), in)
	require.NoError(t, err)

	var maint sreportalv1alpha1.Maintenance
	err = c.Get(context.Background(), types.NamespacedName{Name: "maint-1", Namespace: testNamespace}, &maint)
	require.NoError(t, err)
	assert.Equal(t, "changed title", maint.Spec.Title)
	assert.Equal(t, "original desc", maint.Spec.Description)
	assert.Equal(t, "main", maint.Spec.PortalRef)
	assert.Equal(t, []string{"api"}, maint.Spec.Components)
	assert.Equal(t, sreportalv1alpha1.MaintenanceAffectedMaintenance, maint.Spec.AffectedStatus)
}

func TestUpdateMaintenance_NotFound(t *testing.T) {
	svc := newTestService()
	in := statuspage.UpdateMaintenanceInput{Name: "nonexistent"}
	_, err := svc.UpdateMaintenance(context.Background(), in)
	require.ErrorIs(t, err, statuspage.ErrNotFound)
}

func TestUpdateMaintenance_MissingName(t *testing.T) {
	svc := newTestService()
	in := statuspage.UpdateMaintenanceInput{}
	_, err := svc.UpdateMaintenance(context.Background(), in)
	require.ErrorIs(t, err, statuspage.ErrNameRequired)
}

// --- CreateIncident ---

func TestCreateIncident_Success(t *testing.T) {
	c := fake.NewClientBuilder().WithScheme(testScheme()).Build()
	svc := statuspage.NewService(c, testNamespace)

	ts := metav1.NewTime(time.Date(2026, 3, 28, 10, 0, 0, 0, time.UTC))
	in := statuspage.CreateIncidentInput{
		Title:      "API down",
		PortalRef:  "main",
		Components: []string{"api"},
		Severity:   sreportalv1alpha1.IncidentSeverityCritical,
		InitialUpdate: sreportalv1alpha1.IncidentUpdate{
			Timestamp: ts,
			Phase:     sreportalv1alpha1.IncidentPhaseInvestigating,
			Message:   "Investigating API errors",
		},
	}

	name, err := svc.CreateIncident(context.Background(), in)
	require.NoError(t, err)
	assert.NotEmpty(t, name)

	var inc sreportalv1alpha1.Incident
	err = c.Get(context.Background(), types.NamespacedName{Name: name, Namespace: testNamespace}, &inc)
	require.NoError(t, err)
	assert.Equal(t, "API down", inc.Spec.Title)
	assert.Equal(t, "main", inc.Spec.PortalRef)
	assert.Equal(t, []string{"api"}, inc.Spec.Components)
	assert.Equal(t, sreportalv1alpha1.IncidentSeverityCritical, inc.Spec.Severity)
	require.Len(t, inc.Spec.Updates, 1)
	assert.Equal(t, sreportalv1alpha1.IncidentPhaseInvestigating, inc.Spec.Updates[0].Phase)
	assert.Equal(t, "Investigating API errors", inc.Spec.Updates[0].Message)
}

func TestCreateIncident_AlreadyExists(t *testing.T) {
	generatedName := statuspage.GenerateCRName("main", "existing incident")
	existing := &sreportalv1alpha1.Incident{
		ObjectMeta: metav1.ObjectMeta{Name: generatedName, Namespace: testNamespace},
		Spec: sreportalv1alpha1.IncidentSpec{
			Title:     "existing incident",
			PortalRef: "main",
			Severity:  sreportalv1alpha1.IncidentSeverityCritical,
		},
	}
	svc := newTestService(existing)

	in := statuspage.CreateIncidentInput{
		Title:     "existing incident",
		PortalRef: "main",
		Severity:  sreportalv1alpha1.IncidentSeverityCritical,
		InitialUpdate: sreportalv1alpha1.IncidentUpdate{
			Timestamp: metav1.Now(),
			Phase:     sreportalv1alpha1.IncidentPhaseInvestigating,
			Message:   "Investigating",
		},
	}

	_, err := svc.CreateIncident(context.Background(), in)
	require.ErrorIs(t, err, statuspage.ErrAlreadyExists)
}

func TestCreateIncident_MissingPortalRef(t *testing.T) {
	svc := newTestService()
	in := statuspage.CreateIncidentInput{Title: "x", Severity: sreportalv1alpha1.IncidentSeverityMinor}
	_, err := svc.CreateIncident(context.Background(), in)
	require.ErrorIs(t, err, statuspage.ErrPortalRefRequired)
}

func TestCreateIncident_MissingTitle(t *testing.T) {
	svc := newTestService()
	in := statuspage.CreateIncidentInput{PortalRef: "main", Severity: sreportalv1alpha1.IncidentSeverityMinor}
	_, err := svc.CreateIncident(context.Background(), in)
	require.ErrorIs(t, err, statuspage.ErrTitleRequired)
}

func TestCreateIncident_MissingSeverity(t *testing.T) {
	svc := newTestService()
	in := statuspage.CreateIncidentInput{Title: "x", PortalRef: "main"}
	_, err := svc.CreateIncident(context.Background(), in)
	require.ErrorIs(t, err, statuspage.ErrSeverityRequired)
}

// --- UpdateIncident ---

func TestUpdateIncident_AppendsUpdate(t *testing.T) {
	ts1 := metav1.NewTime(time.Date(2026, 3, 28, 10, 0, 0, 0, time.UTC))
	existing := makeIncidentCR([]sreportalv1alpha1.IncidentUpdate{
		{Timestamp: ts1, Phase: sreportalv1alpha1.IncidentPhaseInvestigating, Message: "Looking into it"},
	})
	c := fake.NewClientBuilder().WithScheme(testScheme()).WithRuntimeObjects(existing).Build()
	svc := statuspage.NewService(c, testNamespace)

	ts2 := metav1.NewTime(time.Date(2026, 3, 28, 11, 0, 0, 0, time.UTC))
	in := statuspage.UpdateIncidentInput{
		Name: "inc-1",
		Update: sreportalv1alpha1.IncidentUpdate{
			Timestamp: ts2,
			Phase:     sreportalv1alpha1.IncidentPhaseIdentified,
			Message:   "Root cause found",
		},
	}

	name, err := svc.UpdateIncident(context.Background(), in)
	require.NoError(t, err)
	assert.Equal(t, "inc-1", name)

	var inc sreportalv1alpha1.Incident
	err = c.Get(context.Background(), types.NamespacedName{Name: "inc-1", Namespace: testNamespace}, &inc)
	require.NoError(t, err)
	require.Len(t, inc.Spec.Updates, 2)
	assert.Equal(t, sreportalv1alpha1.IncidentPhaseInvestigating, inc.Spec.Updates[0].Phase)
	assert.Equal(t, sreportalv1alpha1.IncidentPhaseIdentified, inc.Spec.Updates[1].Phase)
	assert.Equal(t, "Root cause found", inc.Spec.Updates[1].Message)
}

func TestUpdateIncident_UpdatesTitle(t *testing.T) {
	existing := makeIncidentCR(nil)
	c := fake.NewClientBuilder().WithScheme(testScheme()).WithRuntimeObjects(existing).Build()
	svc := statuspage.NewService(c, testNamespace)

	newTitle := "Updated title"
	in := statuspage.UpdateIncidentInput{
		Name:  "inc-1",
		Title: &newTitle,
		Update: sreportalv1alpha1.IncidentUpdate{
			Timestamp: metav1.Now(),
			Phase:     sreportalv1alpha1.IncidentPhaseMonitoring,
			Message:   "monitoring",
		},
	}

	_, err := svc.UpdateIncident(context.Background(), in)
	require.NoError(t, err)

	var inc sreportalv1alpha1.Incident
	err = c.Get(context.Background(), types.NamespacedName{Name: "inc-1", Namespace: testNamespace}, &inc)
	require.NoError(t, err)
	assert.Equal(t, "Updated title", inc.Spec.Title)
}

func TestUpdateIncident_UpdatesSeverity(t *testing.T) {
	existing := makeIncidentCR(nil)
	c := fake.NewClientBuilder().WithScheme(testScheme()).WithRuntimeObjects(existing).Build()
	svc := statuspage.NewService(c, testNamespace)

	sev := sreportalv1alpha1.IncidentSeverityMinor
	in := statuspage.UpdateIncidentInput{
		Name:     "inc-1",
		Severity: &sev,
		Update: sreportalv1alpha1.IncidentUpdate{
			Timestamp: metav1.Now(),
			Phase:     sreportalv1alpha1.IncidentPhaseMonitoring,
			Message:   "downgraded",
		},
	}

	_, err := svc.UpdateIncident(context.Background(), in)
	require.NoError(t, err)

	var inc sreportalv1alpha1.Incident
	err = c.Get(context.Background(), types.NamespacedName{Name: "inc-1", Namespace: testNamespace}, &inc)
	require.NoError(t, err)
	assert.Equal(t, sreportalv1alpha1.IncidentSeverityMinor, inc.Spec.Severity)
}

func TestUpdateIncident_UpdatesComponents(t *testing.T) {
	existing := makeIncidentCR(nil)
	c := fake.NewClientBuilder().WithScheme(testScheme()).WithRuntimeObjects(existing).Build()
	svc := statuspage.NewService(c, testNamespace)

	in := statuspage.UpdateIncidentInput{
		Name:       "inc-1",
		Components: []string{"api", "db"},
		Update: sreportalv1alpha1.IncidentUpdate{
			Timestamp: metav1.Now(),
			Phase:     sreportalv1alpha1.IncidentPhaseIdentified,
			Message:   "also affects db",
		},
	}

	_, err := svc.UpdateIncident(context.Background(), in)
	require.NoError(t, err)

	var inc sreportalv1alpha1.Incident
	err = c.Get(context.Background(), types.NamespacedName{Name: "inc-1", Namespace: testNamespace}, &inc)
	require.NoError(t, err)
	assert.Equal(t, []string{"api", "db"}, inc.Spec.Components)
}

func TestUpdateIncident_LeavesFieldsUnchanged(t *testing.T) {
	existing := makeIncidentCR(nil)
	c := fake.NewClientBuilder().WithScheme(testScheme()).WithRuntimeObjects(existing).Build()
	svc := statuspage.NewService(c, testNamespace)

	in := statuspage.UpdateIncidentInput{
		Name: "inc-1",
		Update: sreportalv1alpha1.IncidentUpdate{
			Timestamp: metav1.Now(),
			Phase:     sreportalv1alpha1.IncidentPhaseMonitoring,
			Message:   "still monitoring",
		},
	}

	_, err := svc.UpdateIncident(context.Background(), in)
	require.NoError(t, err)

	var inc sreportalv1alpha1.Incident
	err = c.Get(context.Background(), types.NamespacedName{Name: "inc-1", Namespace: testNamespace}, &inc)
	require.NoError(t, err)
	assert.Equal(t, "existing incident", inc.Spec.Title)
	assert.Equal(t, "main", inc.Spec.PortalRef)
	assert.Equal(t, sreportalv1alpha1.IncidentSeverityCritical, inc.Spec.Severity)
}

func TestUpdateIncident_NotFound(t *testing.T) {
	svc := newTestService()

	in := statuspage.UpdateIncidentInput{
		Name: "nonexistent",
		Update: sreportalv1alpha1.IncidentUpdate{
			Timestamp: metav1.Now(),
			Phase:     sreportalv1alpha1.IncidentPhaseInvestigating,
			Message:   "test",
		},
	}

	_, err := svc.UpdateIncident(context.Background(), in)
	require.ErrorIs(t, err, statuspage.ErrNotFound)
}

func TestUpdateIncident_MissingName(t *testing.T) {
	svc := newTestService()
	in := statuspage.UpdateIncidentInput{
		Update: sreportalv1alpha1.IncidentUpdate{
			Timestamp: metav1.Now(),
			Phase:     sreportalv1alpha1.IncidentPhaseInvestigating,
			Message:   "test",
		},
	}
	_, err := svc.UpdateIncident(context.Background(), in)
	require.ErrorIs(t, err, statuspage.ErrNameRequired)
}
