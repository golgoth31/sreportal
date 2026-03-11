package alertmanager

// Receiver represents an Alertmanager notification integration.
// Value object: no ID, equality by name.
type Receiver struct {
	name string
}

// NewReceiver creates a Receiver. Name must not be empty.
func NewReceiver(name string) (Receiver, error) {
	if name == "" {
		return Receiver{}, ErrInvalidReceiver
	}
	return Receiver{name: name}, nil
}

// Name returns the receiver name.
func (r Receiver) Name() string {
	return r.name
}
