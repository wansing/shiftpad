package main

import (
	"net/http"
	"sync"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/wansing/shiftpad"
	"github.com/wansing/shiftpad/ical"
)

type DB interface {
	AddPad(shiftpad.Pad) error
	AddShare(pad shiftpad.Pad, id string, auth shiftpad.Auth) error
	AddShift(*shiftpad.Pad, shiftpad.Shift) error
	ApproveTake(*shiftpad.Shift, shiftpad.Take) error
	DeletePad(shiftpad.Pad) error
	DeletePads(string) error
	DeleteShift(*shiftpad.Shift) error
	GetAuthPad(id, secret string) (shiftpad.AuthPad, error)
	GetShift(pad *shiftpad.Pad, shift int) (*shiftpad.Shift, error)
	GetShifts(pad *shiftpad.Pad, from, to int64) ([]shiftpad.Shift, error) // begin: from inclusive, to exclusive
	GetShiftsByEvent(pad *shiftpad.Pad, eventUID string) ([]shiftpad.Shift, error)
	GetTakerNames(*shiftpad.Pad) ([]string, error)
	GetTakesByTaker(pad *shiftpad.Pad, name string) ([]shiftpad.Shift, error)
	SetPaidOut([]shiftpad.Take) error
	TakeShift(*shiftpad.Pad, *shiftpad.Shift, shiftpad.Take) error
	UpdatePad(*shiftpad.Pad) error
	UpdatePadLastUpdated(pad *shiftpad.Pad, lastUpdated string) error
	UpdateShift(*shiftpad.Pad, *shiftpad.Shift) error
}

type Server struct {
	CreateKeys     []string
	DB             DB
	icalCaches     map[string]*ical.FeedCache
	icalCachesLock sync.Mutex
	sessionManager *scs.SessionManager
}

func NewServer(db DB) *Server {
	sessionManager := scs.New()
	sessionManager.Cookie.Persist = false                 // Don't store cookie across browser sessions. Required for GDPR cookie consent exemption criterion B.
	sessionManager.Cookie.SameSite = http.SameSiteLaxMode // good CSRF protection if/because HTTP GET don't modify anything
	sessionManager.Cookie.Secure = false                  // else running on localhost:8080 fails
	sessionManager.IdleTimeout = 2 * time.Hour
	sessionManager.Lifetime = 12 * time.Hour

	return &Server{
		DB:             db,
		icalCaches:     make(map[string]*ical.FeedCache),
		sessionManager: sessionManager,
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
