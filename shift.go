package shiftpad

import (
	"time"

	"github.com/gorhill/cronexpr"
)

// MaxFuture specifies how far in the future shifts can be created and edited, and the expiry time of pads.
const MaxFuture = 180 * 24 * time.Hour

type Shift struct {
	ID           int
	Modified     time.Time // used in ical export
	Name         string    // matched against Pad.ShiftNames
	Note         string
	EventUID     string
	Begin        time.Time // required
	End          time.Time // required
	TakerName    string
	TakerContact string
}

func (shift Shift) AfterDeadline(deadline string) bool {
	if deadline == "" {
		return true // no deadline
	}
	nextDeadline := cronexpr.MustParse(deadline).Next(time.Now())
	return shift.Begin.After(nextDeadline)
}

func (shift Shift) Over() bool {
	return time.Now().After(shift.End)
}

func (shift Shift) Taken() bool {
	return shift.TakerName != "" || shift.TakerContact != ""
}
