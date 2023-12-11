package shiftpad

import (
	"time"

	"github.com/wansing/shiftpad/datefmt"
	"github.com/wansing/shiftpad/ical"
	"golang.org/x/exp/slices"
)

type Event struct {
	ical.Event
	Shifts []Shift
}

// Group is used for displaying. It can represent an event or a bunch of independent shifts.
type Group struct {
	*ical.Event // can be nil
	Shifts      []Shift
}

type Day struct {
	Begin  time.Time // inclusive
	End    time.Time // exclusive
	Events []Event   // both with and without shifts
	Shifts []Shift   // without an event
}

type Repository interface {
	GetICalFeedCache(url string) *ical.FeedCache
	GetShifts(pad *Pad, from, to int64) ([]Shift, error) // begin: from inclusive, to exclusive
	GetShiftsByEvent(pad *Pad, eventUID string) ([]Shift, error)
}

// date must be any time in the day
func GetDay(repo Repository, pad *Pad, date time.Time, location *time.Location) (Day, error) {
	from := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, location)
	to := from.AddDate(0, 0, 1)

	events, shifts, err := GetInterval(repo, pad, from, to, location)
	if err != nil {
		return Day{}, err
	}

	return Day{
		Begin:  from,
		End:    to,
		Events: events,
		Shifts: shifts,
	}, nil
}

// Groups returns the events plus an event with empty UID, which contains all shifts without an event.
func (day Day) Groups() []Group {
	var groups []Group
	if len(day.Shifts) > 0 {
		groups = append(groups, Group{
			Shifts: day.Shifts,
		})
	}
	for _, event := range day.Events {
		event := event // see https://github.com/golang/go/discussions/56010
		groups = append(groups, Group{
			Event:  &event.Event,
			Shifts: event.Shifts,
		})
	}
	return groups
}

func (day Day) FmtDateTime(t time.Time) string {
	return datefmt.DateTime(t, day.Begin)
}

func (day Day) FmtDateTimeRange(begin, end time.Time) string {
	return datefmt.DateTimeRange(begin, end, day.Begin)
}

func (day Day) FmtEventTime(event ical.Event) string {
	return datefmt.DateTimeRange(event.Start, event.End, day.Begin)
}

func (day Day) FmtShiftTime(shift Shift) string {
	return datefmt.DateTimeRange(shift.Begin, shift.End, day.Begin)
}

type Week struct {
	Begin time.Time
	End   time.Time
	Days  [7]*Day
}

// date must be any time in the week
func GetWeek(repo Repository, pad *Pad, year, week int, location *time.Location) (Week, error) {
	// get start of week
	// "Jan 01 to Jan 03 of year n might belong to week 52 or 53 of year n-1", so January 4th is always in week one
	begin := time.Date(year, time.January, 4, 0, 0, 0, 0, location)
	begin = begin.AddDate(0, 0, 7*(week-1))
	// go back to monday
	for begin.Weekday() != time.Monday {
		begin = begin.AddDate(0, 0, -1)
	}
	to := begin.AddDate(0, 0, 7)

	events, shifts, err := GetInterval(repo, pad, begin, to, location)
	if err != nil {
		return Week{}, err
	}

	// create days
	var days = [7]*Day{}
	for i := range days {
		days[i] = &Day{
			Begin: begin.AddDate(0, 0, i),
			End:   begin.AddDate(0, 0, i+1),
		}
	}

	// move independent shifts into their individual day
	for _, shift := range shifts {
		shiftDay := closestDay(days, shift.Begin)
		shiftDay.Shifts = append(shiftDay.Shifts, shift)
	}

	// move events
	for _, event := range events {
		eventDay := closestDay(days, event.Start)
		eventDay.Events = append(eventDay.Events, event)
	}

	return Week{
		Begin: begin,
		End:   to,
		Days:  days,
	}, nil
}

func closestDay(days [7]*Day, t time.Time) *Day {
	if t.Before(days[0].Begin) {
		return days[0]
	}
	for _, day := range days {
		if t.After(day.Begin) && t.Before(day.End) {
			return day
		}
	}
	return days[6]
}

// weeks can't be split into days because shifts and events may be contained in multiple days then, but we want them just once in a week (or any other interval)
func GetInterval(repo Repository, pad *Pad, from, to time.Time, location *time.Location) ([]Event, []Shift, error) {

	// get shifts from time interval
	var eventUIDs = make(map[string]interface{})
	var independentShifts = []Shift{}
	shifts, err := repo.GetShifts(pad, from.Unix(), to.Unix())
	if err != nil {
		return nil, nil, err
	}
	for _, shift := range shifts {
		if shift.EventUID == "" {
			independentShifts = append(independentShifts, shift)
		} else {
			eventUIDs[shift.EventUID] = struct{}{}
		}
	}

	// get every event which is referenced in a shift or overlaps with the time interval
	var events = []Event{}
	icalEvents, _ := repo.GetICalFeedCache(pad.ICalOverlay).Get(location)
	for _, icalEvent := range icalEvents {
		if _, ok := eventUIDs[icalEvent.UID]; ok || overlaps(icalEvent.Start, icalEvent.End, from, to) {
			events = append(events, Event{
				Event: icalEvent,
			})
			delete(eventUIDs, icalEvent.UID)
		}
	}

	// create dummies for remaining event uids
	for uid := range eventUIDs {
		events = append(events, Event{
			Event: ical.Event{
				UID:     uid,
				Summary: "Unknown event " + uid,
			},
		})
	}

	// get shifts of each event (including shifts that are not in the time interval)
	for i, event := range events {
		shifts, err := repo.GetShiftsByEvent(pad, event.UID)
		if err != nil {
			return nil, nil, err
		}
		events[i].Shifts = shifts
	}

	// adjust start and end times of dummy events
	for i, event := range events {
		if event.Start.IsZero() && event.End.IsZero() {
			events[i].Start = minStart(event.Shifts)
			events[i].End = maxEnd(event.Shifts)
		}
	}

	// sort events by begin
	slices.SortFunc(events, func(a, b Event) bool {
		if a.Event.Start.Equal(b.Event.Start) {
			return a.Event.Summary < b.Event.Summary
		}
		return a.Event.Start.Before(b.Event.Start)
	})

	return events, independentShifts, nil
}

func maxEnd(shifts []Shift) time.Time {
	var max time.Time
	for _, shift := range shifts {
		if max.IsZero() || max.Before(shift.End) {
			max = shift.End
		}
	}
	return max
}

func minStart(shifts []Shift) time.Time {
	var min time.Time
	for _, shift := range shifts {
		if min.IsZero() || min.After(shift.Begin) {
			min = shift.Begin
		}
	}
	return min
}

func overlaps(begin1, end1, begin2, end2 time.Time) bool {
	switch {
	case begin1.IsZero() && end1.IsZero():
		return false
	case begin1.IsZero():
		begin1 = end1
	case end1.IsZero():
		end1 = begin1
	}

	switch {
	case begin2.IsZero() && end2.IsZero():
		return false
	case begin2.IsZero():
		begin2 = end2
	case end2.IsZero():
		end2 = begin2
	}

	if end1.Before(begin2) || end1.Equal(begin2) {
		return false
	}
	if begin1.After(end2) || begin1.Equal(end2) {
		return false
	}

	return true
}
