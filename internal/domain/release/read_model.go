package release

import "time"

// EntryView is the read model for a single release event.
type EntryView struct {
	PortalRef string
	Day       string // YYYY-MM-DD (logical day of the Release CR)
	Type      string
	Version   string
	Origin    string
	Date      time.Time
	Author    string
	Message   string
	Link      string
}

// DayView groups release entries by day.
type DayView struct {
	Day     string // "2026-03-25"
	Entries []EntryView
}
