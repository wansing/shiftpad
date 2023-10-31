package main

import (
	"sync"
	"time"

	"github.com/wansing/shiftpad"
	"github.com/wansing/shiftpad/ical"
)

type DB interface {
	AddPad(shiftpad.Pad) error
	AddShift(*shiftpad.Pad, shiftpad.Shift) error
	DeletePad(shiftpad.Pad) error
	DeletePads(string) error
	DeleteShift(*shiftpad.Pad, *shiftpad.Shift) error
	GetPad(id string) (*shiftpad.Pad, error)
	GetShift(pad *shiftpad.Pad, shift int) (*shiftpad.Shift, error)
	GetShifts(pad *shiftpad.Pad, from, to int64) ([]shiftpad.Shift, error) // begin: from inclusive, to exclusive
	GetShiftsByEvent(pad *shiftpad.Pad, eventUID string) ([]shiftpad.Shift, error)
	TakeShift(*shiftpad.Pad, *shiftpad.Shift) error
	UpdatePad(*shiftpad.Pad) error
	UpdatePadLastUpdated(pad *shiftpad.Pad, lastUpdated string) error
	UpdateShift(*shiftpad.Pad, *shiftpad.Shift) error
}

type Server struct {
	CreateKeys     []string
	DB             DB
	icalCaches     map[string]*ical.FeedCache
	icalCachesLock sync.Mutex
}

func NewServer(db DB) *Server {
	return &Server{
		DB:         db,
		icalCaches: make(map[string]*ical.FeedCache),
	}
}

func (srv *Server) Cleanup() error {
	date := time.Now().Add(-shiftpad.MaxFuture).Format(time.DateOnly)
	return srv.DB.DeletePads(date)
}

func (srv *Server) GetICalFeedCache(url string) *ical.FeedCache {
	srv.icalCachesLock.Lock()
	defer srv.icalCachesLock.Unlock()

	feedCache, ok := srv.icalCaches[url]
	if !ok {
		feedCache = &ical.FeedCache{
			Config: ical.Config{
				URL: url,
			},
		}
		srv.icalCaches[url] = feedCache
	}
	return feedCache
}

func (srv *Server) GetShifts(pad *shiftpad.Pad, from, to int64) ([]shiftpad.Shift, error) {
	return srv.DB.GetShifts(pad, from, to)
}

func (srv *Server) GetShiftsByEvent(pad *shiftpad.Pad, eventUID string) ([]shiftpad.Shift, error) {
	return srv.DB.GetShiftsByEvent(pad, eventUID)
}

func (srv *Server) UpdatePadLastUpdated(pad *shiftpad.Pad) error {
	today := time.Now().Format(time.DateOnly)
	return srv.DB.UpdatePadLastUpdated(pad, today)
}
