package shiftpad

import (
	"time"

	"github.com/gorhill/cronexpr"
)

// MaxFuture specifies how far in the future shifts can be created and edited, and the expiry time of pads.
const MaxFuture = 180 * 24 * time.Hour

type Shift struct {
	ID       int
	Modified time.Time // used in ical export
	Name     string    // matched against Pad.ShiftNames
	Note     string
	EventUID string
	Quantity int
	Begin    time.Time // required
	End      time.Time // required
	Takes    []Take
}

func (shift Shift) AfterDeadline(deadline string) bool {
	if deadline == "" {
		return true // no deadline
	}
	nextDeadline := cronexpr.MustParse(deadline).Next(time.Now())
	return shift.Begin.After(nextDeadline)
}

func (shift Shift) FullyTaken() bool {
	return len(shift.Takes) == shift.Quantity
}

func (shift Shift) Untaken() []struct{} {
	var untaken = shift.Quantity - len(shift.Takes)
	if untaken < 0 {
		untaken = 0
	}
	return make([]struct{}, untaken)
}

func (shift Shift) Over() bool {
	return time.Now().After(shift.End)
}

type Take struct {
	ID      int
	Name    string
	Contact string
}
