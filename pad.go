package shiftpad

import (
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
	ShiftNames  []string
}

func NewPad() *Pad {
	return &Pad{
		ID:          randStr(16),
		LastUpdated: time.Now().Format(time.DateOnly),
		Location:    SystemLocation,
	}
}

func NewShareID() string {
	return randStr(20)
}

func randStr(length int) string {
	var bs = make([]byte, length)
	for i := range bs {
		bs[i] = newDigit()
	}
	return string(bs)
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
