package shiftpad

import (
	"testing"
	"time"
)

func TestAfterDeadline(t *testing.T) {
	shift := Shift{
		Begin: time.Date(2024, time.November, 26, 10, 0, 0, 0, time.UTC), // Tue 15:00
	}

	const deadline = "0 0 15 * * MON *" // any Mon 15:00

	tests := []struct {
		now  time.Time
		want bool
	}{
		{time.Date(2024, time.November, 20, 15, 1, 0, 0, time.UTC), true},
		{time.Date(2024, time.November, 21, 15, 1, 0, 0, time.UTC), true},
		{time.Date(2024, time.November, 22, 15, 1, 0, 0, time.UTC), true},
		{time.Date(2024, time.November, 23, 15, 1, 0, 0, time.UTC), true},
		{time.Date(2024, time.November, 24, 15, 1, 0, 0, time.UTC), true},
		{time.Date(2024, time.November, 25, 14, 59, 0, 0, time.UTC), true}, // Mon 14:59
		{time.Date(2024, time.November, 25, 15, 0, 0, 0, time.UTC), false}, // Mon 15:00
		{time.Date(2024, time.November, 25, 15, 1, 0, 0, time.UTC), false}, // Mon 15:01
		{time.Date(2024, time.November, 26, 14, 59, 0, 0, time.UTC), false},
	}

	for _, test := range tests {
		if got := shift.AfterDeadline(deadline, test.now); got != test.want {
			t.Fatalf("got %v, want %v", got, test.want)
		}
	}
}
