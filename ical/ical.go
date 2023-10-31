package ical

import "time"

type Event struct {
	Start   time.Time
	End     time.Time
	UID     string
	URL     string
	Summary string
}
