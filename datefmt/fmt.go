package datefmt

import (
	"fmt"
	"time"
)

// DateTime returns "2. Jan 2006 15:04", or "15:04" if t and reference have the same day.
func DateTime(t, reference time.Time) string {
	if t.Format(time.DateOnly) == reference.Format(time.DateOnly) {
		return t.Format("15:04")
	}
	return t.Format("2. Jan 2006 15:04")
}

// DateTimeRange formats a date range using DateTime.
func DateTimeRange(begin, end, reference time.Time) string {
	return fmt.Sprintf("%s – %s", DateTime(begin, reference), DateTime(end, reference))
}
