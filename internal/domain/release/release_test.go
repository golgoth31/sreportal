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

func TestNewEntry_Valid(t *testing.T) {
	date := time.Date(2026, 3, 21, 14, 30, 0, 0, time.UTC)

	entry, err := release.NewEntry("deployment", "v1.2.3", "ci/cd", date)

	require.NoError(t, err)
	assert.Equal(t, "deployment", entry.Type)
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
		{"empty type", "", "v1.0.0", "ci", validDate, release.ErrInvalidType},
		{"empty version", "deploy", "", "ci", validDate, release.ErrInvalidVersion},
		{"empty origin", "deploy", "v1.0.0", "", validDate, release.ErrInvalidOrigin},
		{"zero date", "deploy", "v1.0.0", "ci", time.Time{}, release.ErrInvalidDate},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := release.NewEntry(tc.typ, tc.version, tc.origin, tc.date)
			require.ErrorIs(t, err, tc.wantErr)
		})
	}
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
			entry, err := release.NewEntry("deploy", "v1.0.0", "ci", tc.date)
			require.NoError(t, err)
			assert.Equal(t, tc.want, entry.DateKey())
		})
	}
}

func TestEntry_CRName(t *testing.T) {
	date := time.Date(2026, 3, 21, 14, 30, 0, 0, time.UTC)

	entry, err := release.NewEntry("deploy", "v1.0.0", "ci", date)
	require.NoError(t, err)

	assert.Equal(t, "release-2026-03-21", entry.CRName())
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
