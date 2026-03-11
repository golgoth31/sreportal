// Package alertmanager contains pure domain types for Alertmanager alert management.
package alertmanager

import "errors"

var (
	ErrFetchAlerts     = errors.New("failed to fetch alerts from alertmanager")
	ErrFetchReceivers  = errors.New("failed to fetch receivers from alertmanager")
	ErrFetchSilences   = errors.New("failed to fetch silences from alertmanager")
	ErrInvalidURL      = errors.New("invalid alertmanager URL")
	ErrInvalidState    = errors.New("invalid alert state")
	ErrInvalidReceiver = errors.New("invalid receiver name")
	ErrInvalidSilence  = errors.New("invalid silence")
)
