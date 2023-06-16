package shiftpad

import (
	"errors"
	"fmt"
	"net/url"
	"time"

	"golang.org/x/exp/slices"
)

const encodeEmptyAuth = "-" // instead of empty string, so slashes are not collapsed

func Intersect(as, bs []string) []string {
	if len(as) < len(bs) {
		as, bs = bs, as
	}
	// now a is longer
	bmap := make(map[string]any)
	for _, b := range bs {
		bmap[b] = struct{}{}
	}
	var result []string
	for _, a := range as {
		if _, ok := bmap[a]; ok {
			result = append(result, a)
		}
	}
	return result
}

type Auth struct {
	Admin            bool
	Edit             []string
	EditAll          bool
	EditRetroAlways  bool
	Expires          string // yyyy-mm-dd
	Note             string
	Take             []string
	TakeAll          bool
	TakeDeadline     string // cronexpr
	TakerName        []string
	TakerNameAll     bool
	ViewTakerContact bool
	ViewTakerName    bool // also visible if contained in Auth.TakerName
}

func DecodeAuth(s string) (Auth, error) {
	if s == encodeEmptyAuth {
		return Auth{}, nil
	}

	var auth Auth
	values, err := url.ParseQuery(s)
	if err != nil {
		return Auth{}, err
	}
	if values.Get("admin") != "" {
		auth.Admin = true
		auth.EditAll = true
		auth.EditRetroAlways = true
		auth.TakeAll = true
		auth.TakerNameAll = true
		auth.ViewTakerContact = true
		auth.ViewTakerName = true
	} else {
		if values.Get("edit-all") != "" {
			auth.EditAll = true
			auth.TakeAll = true
			auth.TakerNameAll = true
			auth.ViewTakerContact = true
			auth.ViewTakerName = true
		} else {
			auth.Edit = values["edit"]
		}
		if values.Get("edit-retro-always") != "" {
			auth.EditRetroAlways = true
		}
		if values.Get("take-all") != "" {
			auth.TakeAll = true
		} else {
			auth.Take = values["take"]
		}
		if dl := values.Get("take-deadline"); dl != "" {
			auth.TakeDeadline = dl
		}
		if values.Get("taker-name-all") != "" {
			auth.TakerNameAll = true
		} else {
			auth.TakerName = values["taker-name"]
		}
		if values.Get("view-taker-contact") != "" {
			auth.ViewTakerContact = true
		}
		if values.Get("view-taker-name") != "" {
			auth.ViewTakerName = true
		}
	}
	if exp := values.Get("expires"); exp != "" {
		auth.Expires = exp
	}
	if note := values.Get("note"); note != "" {
		auth.Note = note
	}
	return auth, nil
}

// Active returns true if an expiry date is set and is not in the past.
func (auth Auth) Active() bool {
	if auth.Expires == "" {
		return true
	}

	exp, err := time.Parse("2006-01-02", auth.Expires)
	if err != nil {
		return false
	}
	exp = exp.AddDate(0, 0, 1)
	return time.Now().Before(exp)
}

func (auth Auth) CanEdit(shiftname string) bool {
	return auth.EditAll || slices.Contains(auth.Edit, shiftname)
}

// When editing a shift, CanEditShift must be called on the original and on the modified shift.
func (auth Auth) CanEditShift(shift Shift) bool {
	return auth.CanEdit(shift.Name) && CheckBeginEnd(shift.Begin, shift.End, auth.EditRetroAlways, MaxFuture) == nil
}

func (auth Auth) CanEditSomeShift() bool {
	return auth.EditAll || len(auth.Edit) > 0
}

func (auth Auth) CanTake(shiftname string) bool {
	return auth.TakeAll || slices.Contains(auth.Take, shiftname)
}

func (auth Auth) CanTakeShift(shift Shift) bool {
	return auth.CanTake(shift.Name) && !shift.Taken() && !shift.Over() && shift.AfterDeadline(auth.TakeDeadline)
}

func (auth Auth) CanTakerName(shift Shift, name string) bool {
	return auth.CanTakeShift(shift) && (auth.TakerNameAll || slices.Contains(auth.TakerName, name))
}

// CheckBeginEnd checks that begin or end are non-zero, that end is after begin (or end is zero), and that begin and end are not too far in the past and future.
func CheckBeginEnd(begin, end time.Time, pastAlways bool, future time.Duration) error {
	// basics
	if begin.IsZero() && end.IsZero() {
		return errors.New("begin or end must be non-zero")
	}
	if !end.IsZero() && end.Before(begin) {
		return errors.New("end must be after begin")
	}
	// past
	if !pastAlways {
		past := time.Now()
		if !begin.IsZero() && begin.Before(past) {
			return errors.New("begin is too far in the past")
		}
		if !end.IsZero() && end.Before(past) {
			return errors.New("end is too far in the past")
		}
	}
	// future
	if !begin.IsZero() && time.Until(begin) > future {
		return errors.New("begin is too far in the future")
	}
	if !end.IsZero() && time.Until(end) > future {
		return errors.New("end is too far in the future")
	}
	return nil
}

// Encode copies the contents of auth into url.Values and encodes them.
// Note that url.Values are designed for url queries, not url path elements.
// The only difference is the representation of the space character.
func (auth Auth) Encode() ([]byte, error) {
	var values = make(url.Values)
	if auth.Admin {
		values.Set("admin", "1")
	} else {
		if auth.EditAll {
			values.Set("edit-all", "1")
		} else {
			values["edit"] = auth.Edit
		}
		if auth.EditRetroAlways {
			values.Set("edit-retro-always", "1")
		}
		if auth.TakeAll {
			values.Set("take-all", "1")
		} else {
			values["take"] = auth.Take
		}
		if auth.TakeDeadline != "" {
			values.Set("take-deadline", auth.TakeDeadline)
		}
		if auth.TakerNameAll {
			values.Set("taker-name-all", "1")
		} else {
			values["taker-name"] = auth.TakerName
		}
		if auth.ViewTakerContact {
			values.Set("view-taker-contact", "1")
		}
		if auth.ViewTakerName {
			values.Set("view-taker-name", "1")
		}
	}
	if auth.Expires != "" {
		values.Set("expires", auth.Expires)
	}
	if auth.Note != "" {
		values.Set("note", auth.Note)
	}
	encoded := values.Encode()
	if encoded == "" {
		encoded = encodeEmptyAuth
	}
	return []byte(encoded), nil
}

// Restricts returns a copy of input which is restricted to a reference Auth.
// Note that this function is not symmetric and thus not an intersection.
func (ref Auth) Restrict(input Auth) Auth {
	// && bool
	input.Admin = input.Admin && ref.Admin
	input.EditAll = input.EditAll && ref.EditAll
	input.EditRetroAlways = input.EditRetroAlways && ref.EditRetroAlways
	input.TakeAll = input.TakeAll && ref.TakeAll
	input.TakerNameAll = input.TakerNameAll && ref.TakerNameAll
	input.ViewTakerContact = input.ViewTakerContact && ref.ViewTakerContact
	input.ViewTakerName = input.ViewTakerName && ref.ViewTakerName
	// intersect []string
	if !ref.EditAll && !input.EditAll {
		input.Edit = Intersect(input.Edit, ref.Edit)
	}
	if !ref.TakeAll && !input.TakeAll {
		input.Take = Intersect(input.Take, ref.Take)
	}
	if !ref.TakerNameAll && !input.TakerNameAll {
		input.TakerName = Intersect(input.TakerName, ref.TakerName)
	}
	// min
	if ref.Expires != "" {
		if input.Expires > ref.Expires {
			input.Expires = ref.Expires
		}
	}
	// overwrite
	if ref.TakeDeadline != "" {
		input.TakeDeadline = ref.TakeDeadline
	}
	return input
}

// TakerString returns a string containing the taker name and taker contact of
// the given shift, or "taken" if auth.ViewTakerContact and auth.ViewTakerName
// are false. If the shift is not taken, an empty string is returned.
func (auth Auth) TakerString(shift Shift) string {
	if !shift.Taken() {
		return ""
	}

	// copy authorized data to local variables
	var takerContact string
	var takerName string
	if auth.ViewTakerContact {
		takerContact = shift.TakerContact
	}
	if auth.ViewTakerName || slices.Contains(auth.TakerName, shift.TakerName) {
		takerName = shift.TakerName
	}

	switch {
	case takerName != "" && takerContact != "":
		return fmt.Sprintf("%s (%s)", takerName, takerContact)
	case takerName != "":
		return takerName
	case takerContact != "":
		return takerContact
	default:
		return "taken"
	}
}
