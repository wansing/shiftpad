package shiftpad

import (
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"fmt"
)

var ErrUnauthorized = errors.New("unauthorized")

// An AuthPad is a pad with verified authentication.
type AuthPad struct {
	Auth
	*Pad
}

// EditShiftNames returns the shift names that may be edited explicitly.
// You must check EditAll separately.
func (ap AuthPad) EditShiftNames() []string {
	return Intersect(ap.Auth.Edit, ap.Pad.ShiftNames)
}

func (ap AuthPad) Link() string {
	encodedAuth, err := ap.Auth.Encode()
	if err != nil {
		return ""
	}
	sig := ed25519.Sign(ap.Pad.PrivateKey, encodedAuth)
	sig64 := base64.RawURLEncoding.EncodeToString(sig)
	return fmt.Sprintf("/p/%s/%s/%s", ap.Pad.ID, string(encodedAuth), string(sig64))
}

// useful for ical link
func (ap AuthPad) Readonly() AuthPad {
	return AuthPad{
		Auth: ap.Restrict(
			Auth{
				ViewTakerContact: true,
				ViewTakerName:    true,
			},
		),
		Pad: ap.Pad,
	}
}

// Verify verifies the given base64-encoded signature of the given auth string,
// decodes the auth string and checks auth.Active().
func Verify(pad *Pad, authstr, sig string) (AuthPad, error) {
	sigBytes, err := base64.RawURLEncoding.DecodeString(sig)
	if err != nil {
		return AuthPad{}, err
	}
	if !ed25519.Verify(pad.PrivateKey.Public().(ed25519.PublicKey), []byte(authstr), []byte(sigBytes)) {
		return AuthPad{}, ErrUnauthorized
	}
	auth, err := DecodeAuth(authstr)
	if err != nil {
		return AuthPad{}, err
	}
	if !auth.Active() {
		return AuthPad{}, ErrUnauthorized
	}
	return AuthPad{
		Auth: auth,
		Pad:  pad,
	}, nil
}
