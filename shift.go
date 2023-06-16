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
	Note         string    // free text, not matched
	EventUID     string
	Begin        time.Time // Begin.IsZero() means undefined
	End          time.Time // End.IsZero() means undefined
	TakerName    string
	TakerContact string
}

func (shift Shift) AfterDeadline(deadline string) bool {
	if deadline == "" {
		return true // no deadline
	}
	nextDeadline := cronexpr.MustParse(deadline).Next(time.Now())
	return shift.BeginTime().After(nextDeadline)
}

// BeginTime returns shift.Begin if it is not zero, or shift.End else.
func (shift Shift) BeginTime() time.Time {
	var t = shift.Begin
	if t.IsZero() {
		t = shift.End
	}
	return t
}

// EndTime returns shift.End if it is not zero, or shift.Begin else.
func (shift Shift) EndTime() time.Time {
	var t = shift.End
	if t.IsZero() {
		t = shift.Begin
	}
	return t
}

func (shift Shift) Over() bool {
	return time.Now().After(shift.EndTime())
}

func (shift Shift) Taken() bool {
	return shift.TakerName != "" || shift.TakerContact != ""
}
