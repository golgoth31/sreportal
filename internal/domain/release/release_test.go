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

package release_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/golgoth31/sreportal/internal/domain/release"
)

const (
	versionV1      = "v1.0.0"
	typeDeploy     = "deploy"
	typeDeployment = "deployment"
)

func TestNewEntry_Valid(t *testing.T) {
	date := time.Date(2026, 3, 21, 14, 30, 0, 0, time.UTC)

	entry, err := release.NewEntry(typeDeployment, "v1.2.3", "ci/cd", date)

	require.NoError(t, err)
	assert.Equal(t, typeDeployment, entry.Type)
	assert.Equal(t, "v1.2.3", entry.Version)
	assert.Equal(t, "ci/cd", entry.Origin)
	assert.Equal(t, date, entry.Date)
}

func TestNewEntry_Validation(t *testing.T) {
	validDate := time.Date(2026, 3, 21, 14, 30, 0, 0, time.UTC)

	cases := []struct {
		name    string
		typ     string
		version string
		origin  string
		date    time.Time
		wantErr error
	}{
		{"empty type", "", versionV1, "ci", validDate, release.ErrInvalidType},
		{"empty origin", typeDeploy, versionV1, "", validDate, release.ErrInvalidOrigin},
		{"zero date", typeDeploy, versionV1, "ci", time.Time{}, release.ErrInvalidDate},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := release.NewEntry(tc.typ, tc.version, tc.origin, tc.date)
			require.ErrorIs(t, err, tc.wantErr)
		})
	}
}

func TestNewEntry_EmptyVersion_IsValid(t *testing.T) {
	date := time.Date(2026, 3, 21, 14, 30, 0, 0, time.UTC)

	entry, err := release.NewEntry(typeDeployment, "", "ci/cd", date)

	require.NoError(t, err)
	assert.Equal(t, typeDeployment, entry.Type)
	assert.Empty(t, entry.Version)
	assert.Equal(t, "ci/cd", entry.Origin)
}

func TestEntry_DateKey(t *testing.T) {
	cases := []struct {
		name string
		date time.Time
		want string
	}{
		{"utc date", time.Date(2026, 3, 21, 14, 30, 0, 0, time.UTC), "2026-03-21"},
		{"non-utc converts to utc", time.Date(2026, 3, 21, 23, 30, 0, 0, time.FixedZone("EST", -5*3600)), "2026-03-22"},
		{"start of year", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), "2026-01-01"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			entry, err := release.NewEntry(typeDeploy, versionV1, "ci", tc.date)
			require.NoError(t, err)
			assert.Equal(t, tc.want, entry.DateKey())
		})
	}
}

func TestEntry_CRName(t *testing.T) {
	date := time.Date(2026, 3, 21, 14, 30, 0, 0, time.UTC)

	entry, err := release.NewEntry(typeDeploy, versionV1, "ci", date)
	require.NoError(t, err)

	assert.Equal(t, "release-2026-03-21", entry.CRName())
}

func TestValidateType(t *testing.T) {
	cases := []struct {
		name         string
		typ          string
		allowedTypes []string
		wantErr      error
	}{
		{"empty allowed list accepts any type", typeDeployment, nil, nil},
		{"empty slice accepts any type", typeDeployment, []string{}, nil},
		{"type in allowed list", typeDeployment, []string{typeDeployment, "rollback"}, nil},
		{"type not in allowed list", "hotfix", []string{typeDeployment, "rollback"}, release.ErrTypeNotAllowed},
		{"exact match required", typeDeploy, []string{typeDeployment}, release.ErrTypeNotAllowed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := release.ValidateType(tc.typ, tc.allowedTypes)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestEntryView_PortalRefAndDay(t *testing.T) {
	v := release.EntryView{
		PortalRef: "main",
		Day:       "2026-03-25",
		Type:      typeDeployment,
	}
	assert.Equal(t, "main", v.PortalRef)
	assert.Equal(t, "2026-03-25", v.Day)
	assert.Equal(t, typeDeployment, v.Type)
}

func TestParseDateFromCRName(t *testing.T) {
	cases := []struct {
		name    string
		crName  string
		want    string
		wantErr bool
	}{
		{"valid name", "release-2026-03-21", "2026-03-21", false},
		{"invalid prefix", "foo-2026-03-21", "", true},
		{"invalid date", "release-not-a-date", "", true},
		{"too short", "release-", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := release.ParseDateFromCRName(tc.crName)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.want, got)
			}
		})
	}
}
