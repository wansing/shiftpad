package shiftpad

import (
	"fmt"
	"slices"
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
	Paid     bool
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
	var approved = 0
	for _, take := range shift.Takes {
		if take.Approved {
			approved++
		}
	}
	return approved >= shift.Quantity
}

func (shift Shift) HasPayouts() bool {
	for _, take := range shift.Takes {
		if take.PaidOut {
			return true
		}
	}
	return false
}

func (shift Shift) Hours() float64 {
	return shift.End.Sub(shift.Begin).Hours()
}

func (shift Shift) String() string {
	var s = shift.Name
	if shift.Note != "" {
		s = s + " " + shift.Note
	}
	return s
}

// TakeViews returns shift.Takes with auth.ViewTakerName and auth.ViewTakerContact applied.
// Anonymous takes are summarized to "n × anonymous".
func (shift Shift) TakeViews(auth Auth) []Take {
	var takers []Take
	var anonymousApplied int
	var anonymousApproved int
	for _, take := range shift.Takes {
		// copy authorized data to local variables
		var takerName string
		var takerContact string
		if auth.ViewTakerName || slices.Contains(auth.Edit, shift.Name) || slices.Contains(auth.TakerName, take.Name) {
			takerName = take.Name
		}
		if auth.ViewTakerContact || slices.Contains(auth.Edit, shift.Name) {
			takerContact = take.Contact
		}

		if takerName == "" && takerContact == "" {
			if take.Approved {
				anonymousApproved++
			} else {
				anonymousApplied++
			}
			continue
		}

		if takerName == "" {
			takerName = "anonymous"
		}
		takers = append(takers, Take{
			ID:       take.ID,
			Name:     takerName,
			Contact:  takerContact,
			Approved: take.Approved,
			PaidOut:  take.PaidOut,
		})
	}
	if anonymousApproved > 0 {
		takers = append(takers, Take{
			Name:     fmt.Sprintf("%d × anonymous", anonymousApproved),
			Approved: true,
		})
	}
	if anonymousApplied > 0 {
		takers = append(takers, Take{
			Name:     fmt.Sprintf("%d × anonymous", anonymousApplied),
			Approved: false,
		})
	}
	return takers
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
	ID       int
	Name     string
	Contact  string
	Approved bool
	PaidOut  bool // not PaymentDue (although the zero value would be a good default) because its meaning would change if shift.Paid is changed, and because keeping track of payments is important
}

func (take Take) String() string {
	var s = take.Name
	if take.Contact != "" {
		s = s + " (" + take.Contact + ")"
	}
	if !take.Approved {
		s = s + " (applied)"
	}
	return s
}
