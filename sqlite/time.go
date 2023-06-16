package sqlite

import "time"

// toUnix converts the given time.Time to a unix timestamp, mapping the zero value to zero.
func toUnix(t time.Time) int64 {
	if t.IsZero() {
		return 0
	} else {
		return t.Unix()
	}
}

// toTime converts the given unix timestamp to a time.Time, mapping zero to the zero value.
func toTime(unix int64) time.Time {
	if unix == 0 {
		return time.Time{}
	} else {
		return time.Unix(unix, 0)
	}
}
