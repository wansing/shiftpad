package datefmt

import (
	"fmt"
	"time"
)

// Date returns "Monday 2. Jan 2006".
func Date(t time.Time) string {
	return t.Format("Monday 2. Jan 2006")
}

// DateTime returns "2. Jan 2006 15:04", or "15:04" if t and reference have the same day.
func DateTime(t, reference time.Time) string {
	if t.IsZero() {
		return ""
	}
	if t.Format("2006-01-02") == reference.Format("2006-01-02") {
		return t.Format("15:04")
	}
	return t.Format("2. Jan 2006 15:04")
}

// DateTimeRange formats a date range using DateTime.
func DateTimeRange(begin, end, reference time.Time) string {
	beginStr := DateTime(begin, reference)
	endStr := DateTime(end, reference)
	switch {
	case beginStr != "" && endStr != "":
		return fmt.Sprintf("%s – %s", beginStr, endStr)
	case beginStr != "" && endStr == "":
		return beginStr
	case beginStr == "" && endStr != "":
		return fmt.Sprintf("bis %s", endStr)
	default:
		return ""
	}
}

func ISODate(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02")
}

func ISOTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("15:04")
}
