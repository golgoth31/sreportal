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

package config

import "errors"

var (
	// ErrInvalidInterval is returned when the reconciliation interval is not positive.
	ErrInvalidInterval = errors.New("reconciliation interval must be positive")

	// ErrEmptyDefaultGroup is returned when the group mapping default group is empty.
	ErrEmptyDefaultGroup = errors.New("group mapping defaultGroup must not be empty")
)
