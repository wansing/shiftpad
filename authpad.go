package shiftpad

import (
	"errors"
	"fmt"
)

var ErrUnauthorized = errors.New("unauthorized")

type AuthPad struct {
	*Pad
	Share
}

// EditShiftNames returns the shift names that may be edited explicitly.
// You must check EditAll separately.
func (ap AuthPad) EditShiftNames() []string {
	return Intersect(ap.Auth.Edit, ap.Pad.ShiftNames)
}

func (ap AuthPad) Link() string {
	return fmt.Sprintf("/p/%s/%s", ap.Pad.ID, ap.Share.Secret)
}

// useful for ical link
func (ap AuthPad) Readonly() AuthPad {
	return AuthPad{
		Pad: ap.Pad,
		Share: Share{
			Auth: ap.Restrict(
				Auth{
					ViewTakerContact: true,
					ViewTakerName:    true,
				},
			),
			Secret: ap.Secret,
		},
	}
}
