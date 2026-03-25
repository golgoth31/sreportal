package dns

import "errors"

// ErrFQDNNotFound is returned when a requested FQDN does not exist in the store.
var ErrFQDNNotFound = errors.New("fqdn not found")
