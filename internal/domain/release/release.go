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

package release

import (
	"fmt"
	"slices"
	"strings"
	"time"
)

const (
	dateLayout = "2006-01-02"
	crPrefix   = "release-"
)

// Entry represents a single release event (pure domain, no K8s deps).
type Entry struct {
	Type    string
	Version string
	Origin  string
	Date    time.Time
	Author  string // optional
	Message string // optional
	Link    string // optional
}

// NewEntry creates a validated Entry. Date is normalized to UTC.
func NewEntry(typ, version, origin string, date time.Time) (Entry, error) {
	if typ == "" {
		return Entry{}, ErrInvalidType
	}
	if origin == "" {
		return Entry{}, ErrInvalidOrigin
	}
	if date.IsZero() {
		return Entry{}, ErrInvalidDate
	}
	return Entry{
		Type:    typ,
		Version: version,
		Origin:  origin,
		Date:    date.UTC(),
	}, nil
}

// DateKey returns the YYYY-MM-DD string for this entry's date (UTC).
func (e Entry) DateKey() string {
	return e.Date.Format(dateLayout)
}

// CRName returns the K8s CR name for this entry's day.
func (e Entry) CRName() string {
	return crPrefix + e.DateKey()
}

// ValidateType checks that typ is in the allowedTypes list.
// If allowedTypes is empty, all types are accepted (no restriction).
func ValidateType(typ string, allowedTypes []string) error {
	if len(allowedTypes) == 0 {
		return nil
	}
	if slices.Contains(allowedTypes, typ) {
		return nil
	}
	return fmt.Errorf("%w: %q (allowed: %v)", ErrTypeNotAllowed, typ, allowedTypes)
}

// ParseDateFromCRName extracts the YYYY-MM-DD date string from a CR name like "release-2026-03-21".
func ParseDateFromCRName(name string) (string, error) {
	if !strings.HasPrefix(name, crPrefix) {
		return "", fmt.Errorf("%w: missing prefix %q", ErrInvalidCRName, crPrefix)
	}
	dateStr := strings.TrimPrefix(name, crPrefix)
	if _, err := time.Parse(dateLayout, dateStr); err != nil {
		return "", fmt.Errorf("%w: %v", ErrInvalidCRName, err)
	}
	return dateStr, nil
}
