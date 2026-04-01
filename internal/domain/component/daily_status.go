package component

import "time"

// DailyStatus records the worst observed status for a single UTC calendar day.
type DailyStatus struct {
	Date        string          // UTC date in YYYY-MM-DD format
	WorstStatus ComponentStatus // worst status observed during the day
}

// BackfillMissingDays ensures the history contains an entry for every day in the sliding window
// [today - (windowDays-1) .. today]. Missing days are filled with StatusOperational.
// Existing entries are preserved unchanged. The returned slice is sorted ascending by date
// and pruned to the window.
func BackfillMissingDays(history []DailyStatus, today string, windowDays int) []DailyStatus {
	todayTime, _ := time.Parse("2006-01-02", today)
	start := todayTime.AddDate(0, 0, -(windowDays - 1))

	// Index existing entries within the window
	existing := make(map[string]DailyStatus, len(history))
	for _, entry := range history {
		if entry.Date >= start.Format("2006-01-02") && entry.Date <= today {
			existing[entry.Date] = entry
		}
	}

	result := make([]DailyStatus, 0, windowDays)
	for d := start; !d.After(todayTime); d = d.AddDate(0, 0, 1) {
		dateStr := d.Format("2006-01-02")
		if entry, ok := existing[dateStr]; ok {
			result = append(result, entry)
		} else {
			result = append(result, DailyStatus{Date: dateStr, WorstStatus: StatusOperational})
		}
	}

	return result
}

// MergeDailyWorst merges the current status into the daily history for the given UTC date,
// keeping only the worst status per day, and prunes entries outside the sliding window.
// The returned slice is sorted in ascending date order.
func MergeDailyWorst(history []DailyStatus, today string, current ComponentStatus, windowDays int) []DailyStatus {
	// Copy to avoid mutating the caller's slice
	result := make([]DailyStatus, len(history))
	copy(result, history)

	// Merge today's status
	found := false
	for i := range result {
		if result[i].Date == today {
			if StatusSeverityRank(string(current)) > StatusSeverityRank(string(result[i].WorstStatus)) {
				result[i].WorstStatus = current
			}
			found = true
			break
		}
	}
	if !found {
		result = append(result, DailyStatus{Date: today, WorstStatus: current})
	}

	// Prune: compute the cutoff date (today - windowDays + 1)
	todayTime, _ := time.Parse("2006-01-02", today)
	cutoff := todayTime.AddDate(0, 0, -(windowDays - 1)).Format("2006-01-02")

	pruned := result[:0]
	for _, entry := range result {
		if entry.Date >= cutoff {
			pruned = append(pruned, entry)
		}
	}

	return pruned
}

// DurationUntilNextMidnightUTC returns the duration from now until the next UTC midnight.
// Returns at least 1s to avoid zero-duration requeue.
func DurationUntilNextMidnightUTC(now time.Time) time.Duration {
	utc := now.UTC()
	nextMidnight := time.Date(utc.Year(), utc.Month(), utc.Day()+1, 0, 0, 0, 0, time.UTC)
	d := nextMidnight.Sub(utc)
	if d <= 0 {
		return 1 * time.Second
	}
	// Clamp: if exactly 24h (i.e. exactly midnight), return 1s
	if d == 24*time.Hour {
		return 1 * time.Second
	}
	return d
}
