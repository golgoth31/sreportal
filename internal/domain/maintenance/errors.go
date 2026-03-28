package maintenance

import "errors"

var (
	ErrInvalidSchedule = errors.New("scheduledEnd must be after scheduledStart")
)
