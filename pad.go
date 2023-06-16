package shiftpad

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/binary"
	"time"
)

const idBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ123456789"

type Pad struct {
	ID string

	Description string
	ICalOverlay string
	LastUpdated string         // yyyy-mm-dd, update on take and edit (updating on view would cost some performance)
	Location    *time.Location // must not be nil
	Name        string
	PrivateKey  ed25519.PrivateKey
	ShiftNames  []string
}

func NewPad() (Pad, error) {
	var id = make([]byte, 20)
	for i := range id {
		id[i] = newDigit()
	}

	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return Pad{}, err
	}

	return Pad{
		ID:          string(id),
		LastUpdated: time.Now().Format(time.DateOnly),
		Location:    SystemLocation,
		PrivateKey:  privateKey,
	}, nil
}

func newDigit() byte {
	b := make([]byte, 8)
	n, err := rand.Read(b)
	if n != 8 {
		panic(n)
	} else if err != nil {
		panic(err)
	}
	return idBytes[uint(binary.BigEndian.Uint64(b))%uint(len(idBytes))]
}
