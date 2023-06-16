package main

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/emersion/go-ical"
	"github.com/gorhill/cronexpr"
	"github.com/wansing/shiftpad"
	"github.com/wansing/shiftpad/datefmt"
	"github.com/wansing/shiftpad/html"
	"github.com/wansing/shiftpad/html/static"
	"github.com/wansing/shiftpad/sqlite"
	"github.com/wansing/shiftpad/way"
	"golang.org/x/exp/slices"
)

type HandlerFunc func(w http.ResponseWriter, r *http.Request) http.Handler

func (f HandlerFunc) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if handler := f(w, r); handler != nil {
		handler.ServeHTTP(w, r)
	}
}

func main() {
	db, err := sqlite.OpenDB(filepath.Join(os.Getenv("STATE_DIRECTORY"), "db.sqlite3?_busy_timeout=10000&_journal=WAL&_sync=NORMAL&cache=shared"))
	if err != nil {
		log.Printf("error opening db: %s", err)
		return
	}

	log.Printf("system time location: %s", shiftpad.SystemLocation)

	srv := NewServer(db)

	createKeysFile, err := os.ReadFile(filepath.Join(os.Getenv("STATE_DIRECTORY"), "create-keys.txt"))
	if err == nil {
		srv.CreateKeys = split(string(createKeysFile))
		log.Printf("loaded %d create keys", len(srv.CreateKeys))
	}

	go func() {
		for ; true; <-time.Tick(12 * time.Hour) {
			if err := srv.Cleanup(); err == nil {
				log.Println("cleaned up old pads")
			} else {
				log.Printf("error cleaning up old pads: %v", err)
			}
		}
	}()

	// httprouter does not work here because of its %2F bug: https://github.com/julienschmidt/httprouter/issues/208
	var router = way.NewRouter()
	router.Handle(http.MethodGet, "/static/", http.StripPrefix("/static", http.FileServer(http.FS(static.Files))))
	router.Handle(http.MethodGet, "/", HandlerFunc(srv.indexGet))
	router.Handle(http.MethodGet, "/create/:key", srv.withCreateKey(srv.createGet))
	router.Handle(http.MethodPost, "/create/:key", srv.withCreateKey(srv.createPost))
	router.Handle(http.MethodGet, "/p/:pad/:auth/:authsig", srv.withPad(srv.padViewGet))
	router.Handle(http.MethodPost, "/p/:pad/:auth/:authsig", srv.withPad(srv.padViewPost))
	router.Handle(http.MethodGet, "/p/:pad/:auth/:authsig/settings", srv.withPad(srv.padSettingsGet))
	router.Handle(http.MethodPost, "/p/:pad/:auth/:authsig/settings", srv.withPad(srv.padSettingsPost))
	router.Handle(http.MethodGet, "/p/:pad/:auth/:authsig/share", srv.withPad(srv.padShareGet))
	router.Handle(http.MethodPost, "/p/:pad/:auth/:authsig/share", srv.withPad(srv.padSharePost))
	router.Handle(http.MethodGet, "/p/:pad/:auth/:authsig/ical", srv.withPad(srv.padICal))
	router.Handle(http.MethodGet, "/p/:pad/:auth/:authsig/day/:date", srv.withPad(srv.padViewDay))
	router.Handle(http.MethodGet, "/p/:pad/:auth/:authsig/week/:year/:week", srv.withPad(srv.padViewWeekGet))
	router.Handle(http.MethodPost, "/p/:pad/:auth/:authsig/week/:year/:week", srv.withPad(srv.padViewPost)) // same handler as without :year/:week
	router.Handle(http.MethodGet, "/p/:pad/:auth/:authsig/add/:date", srv.withPad(srv.shiftAddGet))
	router.Handle(http.MethodPost, "/p/:pad/:auth/:authsig/add/:date", srv.withPad(srv.shiftAddPost))
	router.Handle(http.MethodGet, "/p/:pad/:auth/:authsig/take/:shift", srv.withShift(srv.shiftTakeGet))
	router.Handle(http.MethodPost, "/p/:pad/:auth/:authsig/take/:shift", srv.withShift(srv.shiftTakePost))
	router.Handle(http.MethodGet, "/p/:pad/:auth/:authsig/edit/:shift", srv.withShift(srv.shiftEditGet))
	router.Handle(http.MethodPost, "/p/:pad/:auth/:authsig/edit/:shift", srv.withShift(srv.shiftEditPost))
	router.Handle(http.MethodGet, "/p/:pad/:auth/:authsig/delete/:shift", srv.withShift(srv.shiftDeleteGet))
	router.Handle(http.MethodPost, "/p/:pad/:auth/:authsig/delete/:shift", srv.withShift(srv.shiftDeletePost))

	log.Println("listening to 127.0.0.1:8200")
	http.ListenAndServe("127.0.0.1:8200", router)
}

func (srv *Server) withCreateKey(f HandlerFunc) HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) http.Handler {
		key := way.Param(r.Context(), "key")
		if key == "" {
			return Forbidden()
		}
		if !slices.Contains(srv.CreateKeys, key) {
			return Forbidden()
		}
		return f(w, r)
	}
}

func (srv *Server) withPad(f func(http.ResponseWriter, *http.Request, shiftpad.AuthPad) http.Handler) HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) http.Handler {
		pad, err := srv.DB.GetPad(way.Param(r.Context(), "pad"))
		if err != nil {
			return NotFound()
		}
		authpad, err := shiftpad.Verify(pad, way.Param(r.Context(), "auth"), way.Param(r.Context(), "authsig"))
		if err != nil {
			return Forbidden()
		}
		return f(w, r, authpad)
	}
}

func (srv *Server) withShift(f func(http.ResponseWriter, *http.Request, shiftpad.AuthPad, *shiftpad.Shift) http.Handler) HandlerFunc {
	return srv.withPad(func(w http.ResponseWriter, r *http.Request, authpad shiftpad.AuthPad) http.Handler {
		id, _ := strconv.Atoi(way.Param(r.Context(), "shift"))
		shift, err := srv.DB.GetShift(authpad.Pad, id)
		if err != nil {
			return NotFound()
		}
		return f(w, r, authpad, shift)
	})
}

func Forbidden() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	})
}

func InternalServerError(err error) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("internal server error: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		html.InternalServerError.Execute(w, nil)
	})
}

func NotFound() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		html.NotFound.Execute(w, nil)
	})
}

func (srv *Server) indexGet(w http.ResponseWriter, r *http.Request) http.Handler {
	err := html.Index.Execute(w, nil)
	if err != nil {
		return InternalServerError(err)
	}
	return nil
}

func (srv *Server) createGet(w http.ResponseWriter, r *http.Request) http.Handler {
	err := html.PadCreate.Execute(w, nil)
	if err != nil {
		return InternalServerError(err)
	}
	return nil
}

func (srv *Server) createPost(w http.ResponseWriter, r *http.Request) http.Handler {
	newPad, err := shiftpad.NewPad()
	if err != nil {
		return InternalServerError(err)
	}
	if err := srv.DB.AddPad(newPad); err != nil {
		return InternalServerError(err)
	}
	authpad := shiftpad.AuthPad{
		Auth: shiftpad.Auth{
			Admin:            true,
			EditAll:          true,
			Note:             "Admin-Link",
			TakeAll:          true,
			TakerNameAll:     true,
			ViewTakerContact: true,
			ViewTakerName:    true,
		},
		Pad: &newPad,
	}
	return http.RedirectHandler(authpad.Link(), http.StatusSeeOther)
}

func (srv *Server) padSettingsGet(w http.ResponseWriter, r *http.Request, authpad shiftpad.AuthPad) http.Handler {
	if !authpad.Admin {
		return NotFound()
	}

	err := html.PadSettings.Execute(w, html.PadSettingsData{
		PadData: html.PadData{
			ActiveTab: "settings",
			Pad:       authpad,
		},
		Locations: shiftpad.Locations(authpad.Location.String()),
	})
	if err != nil {
		return InternalServerError(err)
	}
	return nil
}

func (srv *Server) padSettingsPost(w http.ResponseWriter, r *http.Request, authpad shiftpad.AuthPad) http.Handler {
	if !authpad.Admin {
		return NotFound()
	}

	name := trim(r.PostFormValue("name"), 64)
	description := trim(r.PostFormValue("description"), 1024)
	location := trim(r.PostFormValue("location"), 128)
	loc, err := time.LoadLocation(location)
	if err != nil {
		loc = shiftpad.SystemLocation
	}
	shiftnames := split(trim(r.PostFormValue("shift-names"), 1024))
	slices.Sort(shiftnames)
	icalURL := trim(r.PostFormValue("ical"), 128)
	if icalURL != "" {
		if _, err := url.ParseRequestURI(icalURL); err != nil {
			icalURL = ""
		}
	}

	authpad.Name = name
	authpad.Description = description
	authpad.Location = loc
	authpad.ShiftNames = shiftnames
	authpad.ICalOverlay = icalURL

	if err := srv.DB.UpdatePad(authpad.Pad); err != nil {
		return InternalServerError(err)
	}
	if err := srv.UpdatePadLastUpdated(authpad.Pad); err != nil {
		return InternalServerError(err)
	}

	return http.RedirectHandler(authpad.Link()+"/settings", http.StatusSeeOther)
}

func (srv *Server) padShareGet(w http.ResponseWriter, r *http.Request, authpad shiftpad.AuthPad) http.Handler {
	err := html.PadShare.Execute(w, html.PadShareData{
		PadData: html.PadData{
			ActiveTab: "share",
			Pad:       authpad,
		},
	})
	if err != nil {
		return InternalServerError(err)
	}
	return nil
}

func (srv *Server) padSharePost(w http.ResponseWriter, r *http.Request, authpad shiftpad.AuthPad) http.Handler {
	expires := trim(r.PostFormValue("expires"), 10)
	if expires != "" {
		if _, err := time.Parse("2006-01-02", expires); err != nil {
			return InternalServerError(err)
		}
	}

	takeDeadline := trim(r.PostFormValue("take-deadline"), 64)
	if takeDeadline != "" {
		if _, err := cronexpr.Parse(takeDeadline); err != nil {
			takeDeadline = ""
		}
	}

	edit := shiftpad.Intersect(authpad.ShiftNames, r.PostForm["edit"])
	take := shiftpad.Intersect(authpad.ShiftNames, r.PostForm["take"])
	takerName := split(r.PostFormValue("taker-name")) // taker-name is a textarea, not checkboxes

	shareAuth := shiftpad.Auth{
		Admin:            r.PostFormValue("admin") != "",
		Edit:             edit,
		EditAll:          r.PostFormValue("edit-all") != "",
		EditRetroAlways:  r.PostFormValue("edit-retro-always") != "",
		Expires:          expires,
		Note:             trim(r.PostFormValue("note"), 128),
		Take:             take,
		TakeAll:          r.PostFormValue("take-all") != "",
		TakeDeadline:     takeDeadline,
		TakerName:        takerName,
		TakerNameAll:     r.PostFormValue("taker-name-all") != "",
		ViewTakerContact: r.PostFormValue("view-taker-contact") != "",
		ViewTakerName:    r.PostFormValue("view-taker-name") != "",
	}
	shareAuth = authpad.Restrict(shareAuth)

	var scheme = "https"
	if r.Host == "127.0.0.1" || strings.HasPrefix(r.Host, "127.0.0.1:") || strings.HasSuffix(r.Host, ".onion") {
		scheme = "http"
	}

	sharePad := shiftpad.AuthPad{
		Auth: shareAuth,
		Pad:  authpad.Pad,
	}

	err := html.PadShareResult.Execute(w, html.PadShareResultData{
		PadData: html.PadData{
			Pad: authpad,
		},
		Host: scheme + "://" + r.Host,
		Link: sharePad.Link(),
	})
	if err != nil {
		return InternalServerError(err)
	}
	return nil
}

func (srv *Server) padICal(w http.ResponseWriter, r *http.Request, authpad shiftpad.AuthPad) http.Handler {
	cal := ical.NewCalendar()
	cal.Props.SetText(ical.PropVersion, "2.0")
	cal.Props.SetText(ical.PropProductID, "shiftpad")

	from := time.Now().Add(-24 * time.Hour) // hardcoded, for shifts that have begun shortly before and have no end time
	to := time.Now().Add(shiftpad.MaxFuture)
	shifts, err := srv.DB.GetShifts(authpad.Pad, from.Unix(), to.Unix())
	if err != nil {
		return InternalServerError(err)
	}

	for _, shift := range shifts {
		uid := fmt.Sprintf("%d@%s", shift.ID, authpad.Pad.ID)

		var summary = shift.Name
		if shift.Note != "" {
			summary = summary + " (" + shift.Note + ")"
		}
		if shift.Taken() {
			summary = summary + ": " + authpad.TakerString(shift)
		}

		// ical requires both begin and end
		var begin = shift.Begin
		if begin.IsZero() {
			begin = shift.End.Add(-1 * time.Hour)
			summary = "[open beginning] " + summary
		}
		var end = shift.End
		if end.IsZero() {
			end = shift.Begin.Add(1 * time.Hour)
			summary = "[open end] " + summary
		}

		event := ical.NewEvent()
		event.Props.SetText(ical.PropUID, uid)
		event.Props.SetText(ical.PropSummary, summary)
		// use UTC ("Z") because go-ical can't export timezone details
		event.Props.SetDateTime(ical.PropDateTimeStamp, shift.Modified.In(time.UTC))
		event.Props.SetDateTime(ical.PropDateTimeStart, begin.In(time.UTC))
		event.Props.SetDateTime(ical.PropDateTimeEnd, end.In(time.UTC))
		cal.Children = append(cal.Children, event.Component)
	}

	w.Header().Add("Content-Type", "text/calendar")

	err = ical.NewEncoder(w).Encode(cal)
	switch {
	case err == nil:
		return nil
	case strings.Contains(err.Error(), "calendar is empty"): // go-ical and https://www.rfc-editor.org/rfc/rfc5545#section-3.6 require "one or more calendar components"
		return nil
	default:
		return InternalServerError(err)
	}
}

// padViewGet shows the current week. It does not redirect, so users can bookmark the link.
func (srv *Server) padViewGet(w http.ResponseWriter, r *http.Request, authpad shiftpad.AuthPad) http.Handler {
	year, week := time.Now().ISOWeek()
	return srv.padViewWeek(w, r, authpad, year, week)
}

func (srv *Server) padViewPost(w http.ResponseWriter, r *http.Request, authpad shiftpad.AuthPad) http.Handler {
	yearStr, weekStr, _ := strings.Cut(r.PostFormValue("week"), "-W")
	year, _ := strconv.Atoi(yearStr)
	week, _ := strconv.Atoi(weekStr)
	if year == 0 || week == 0 {
		year, week = time.Now().ISOWeek()
	}
	return http.RedirectHandler(authpad.Link()+fmt.Sprintf("/week/%d/%d", year, week), http.StatusSeeOther)
}

func (srv *Server) padViewDay(w http.ResponseWriter, r *http.Request, authpad shiftpad.AuthPad) http.Handler {
	date, err := time.Parse("2006-01-02", way.Param(r.Context(), "date"))
	if err != nil {
		return NotFound()
	}
	year, week := date.ISOWeek()
	return http.RedirectHandler(authpad.Link()+fmt.Sprintf("/week/%d/%d#%s", year, week, date.Format("2006-01-02")), http.StatusSeeOther)
}

func (srv *Server) padViewWeekGet(w http.ResponseWriter, r *http.Request, authpad shiftpad.AuthPad) http.Handler {
	year, err := strconv.Atoi(way.Param(r.Context(), "year"))
	if err != nil {
		return NotFound()
	}
	if year < 2022 || year > 2100 {
		return NotFound()
	}

	week, err := strconv.Atoi(way.Param(r.Context(), "week"))
	if err != nil {
		return NotFound()
	}
	if week < 1 {
		week = 1
	}
	if week > 53 {
		week = 53
	}

	return srv.padViewWeek(w, r, authpad, year, week)
}

func (srv *Server) padViewWeek(w http.ResponseWriter, r *http.Request, authpad shiftpad.AuthPad, year, weekNumber int) http.Handler {
	week, err := shiftpad.GetWeek(srv, authpad.Pad, year, weekNumber, authpad.Location)
	if err != nil {
		return InternalServerError(err)
	}

	earlierYear, earlierWeek := week.Begin.AddDate(0, 0, -7).ISOWeek()
	laterYear, laterWeek := week.Begin.AddDate(0, 0, 7).ISOWeek()
	thisYear, thisWeek := time.Now().ISOWeek()
	nextYear, nextWeek := time.Now().AddDate(0, 0, 7).ISOWeek()

	if err := html.PadViewWeek.Execute(w, html.PadViewWeekData{
		PadData: html.PadData{
			Pad: authpad,
		},
		ISOWeek:     fmt.Sprintf("%04d-W%02d", year, weekNumber),
		Days:        week.Days,
		EarlierYear: earlierYear,
		EarlierWeek: earlierWeek,
		LaterYear:   laterYear,
		LaterWeek:   laterWeek,
		ThisYear:    thisYear,
		ThisWeek:    thisWeek,
		NextYear:    nextYear,
		NextWeek:    nextWeek,
	}); err != nil {
		return InternalServerError(err)
	}
	return nil
}

func (srv *Server) shiftAddGet(w http.ResponseWriter, r *http.Request, authpad shiftpad.AuthPad) http.Handler {
	if !authpad.CanEditSomeShift() {
		return NotFound()
	}

	return srv.shiftAddTemplate(w, r, authpad, "")
}

func (srv *Server) shiftAddTemplate(w http.ResponseWriter, r *http.Request, authpad shiftpad.AuthPad, errMsg string) http.Handler {
	date, err := time.Parse("2006-01-02", way.Param(r.Context(), "date"))
	if err != nil {
		return NotFound()
	}

	day, err := shiftpad.GetDay(srv, authpad.Pad, date, authpad.Location)
	if err != nil {
		return InternalServerError(err)
	}

	var minDate = time.Date(2000, time.January, 1, 0, 0, 0, 0, authpad.Location)
	if authpad.EditRetroAlways {
		minDate = time.Now()
	}

	if err := html.ShiftCreate.Execute(w, html.ShiftCreateData{
		PadData: html.PadData{
			Pad: authpad,
		},
		Day:     day,
		MaxDate: time.Now().Add(shiftpad.MaxFuture).Format("2006-01-02"),
		MinDate: minDate.Format("2006-01-02"),
		Error:   errMsg,
	}); err != nil {
		return InternalServerError(err)
	}
	return nil
}

func (srv *Server) shiftAddPost(w http.ResponseWriter, r *http.Request, authpad shiftpad.AuthPad) http.Handler {
	if !authpad.CanEditSomeShift() {
		return NotFound()
	}

	name := trim(r.PostFormValue("name"), 64)
	note := trim(r.PostFormValue("note"), 64)
	eventUID := trim(r.PostFormValue("event-uid"), 128)
	begin, _ := time.ParseInLocation("2006-01-02 15:04", r.PostFormValue("begin-date")+" "+r.PostFormValue("begin-time"), authpad.Location)
	end, _ := time.ParseInLocation("2006-01-02 15:04", r.PostFormValue("end-date")+" "+r.PostFormValue("end-time"), authpad.Location)
	if err := shiftpad.CheckBeginEnd(begin, end, authpad.EditRetroAlways, shiftpad.MaxFuture); err != nil {
		return srv.shiftAddTemplate(w, r, authpad, err.Error())
	}

	shift := shiftpad.Shift{
		Name:     name,
		Note:     note,
		Modified: time.Now().In(authpad.Location),
		EventUID: eventUID,
		Begin:    begin,
		End:      end,
	}

	if !authpad.CanEditShift(shift) {
		return Forbidden()
	}

	count, _ := strconv.Atoi(r.PostFormValue("count"))
	if count < 0 {
		count = 0
	}
	if count > 10 {
		count = 10
	}

	for i := 0; i < count; i++ {
		if err := srv.DB.AddShift(authpad.Pad, shift); err != nil {
			return InternalServerError(err)
		}
	}
	if err := srv.UpdatePadLastUpdated(authpad.Pad); err != nil {
		return InternalServerError(err)
	}

	return http.RedirectHandler(authpad.Link()+fmt.Sprintf("/day/%s", datefmt.ISODate(shift.BeginTime())), http.StatusSeeOther)
}

func (srv *Server) shiftDeleteGet(w http.ResponseWriter, r *http.Request, authpad shiftpad.AuthPad, shift *shiftpad.Shift) http.Handler {
	if !authpad.CanEditShift(*shift) {
		return NotFound()
	}

	day, err := shiftpad.GetDay(srv, authpad.Pad, shift.BeginTime(), authpad.Location)
	if err != nil {
		return InternalServerError(err)
	}

	if err := html.ShiftDelete.Execute(w, html.ShiftDeleteData{
		PadData: html.PadData{
			Pad: authpad,
		},
		Day:   day,
		Shift: shift,
	}); err != nil {
		return InternalServerError(err)
	}
	return nil
}

func (srv *Server) shiftDeletePost(w http.ResponseWriter, r *http.Request, authpad shiftpad.AuthPad, shift *shiftpad.Shift) http.Handler {
	if !authpad.CanEditShift(*shift) {
		return NotFound()
	}

	if err := srv.DB.DeleteShift(authpad.Pad, shift); err != nil {
		return InternalServerError(err)
	}
	if err := srv.UpdatePadLastUpdated(authpad.Pad); err != nil {
		return InternalServerError(err)
	}

	return http.RedirectHandler(authpad.Link()+fmt.Sprintf("/day/%s", datefmt.ISODate(shift.BeginTime())), http.StatusSeeOther)
}

func (srv *Server) shiftEditGet(w http.ResponseWriter, r *http.Request, authpad shiftpad.AuthPad, shift *shiftpad.Shift) http.Handler {
	if !authpad.CanEditShift(*shift) {
		return NotFound()
	}

	return srv.shiftEditTemplate(w, r, authpad, shift, "")
}

func (srv *Server) shiftEditTemplate(w http.ResponseWriter, r *http.Request, authpad shiftpad.AuthPad, shift *shiftpad.Shift, errMsg string) http.Handler {
	day, err := shiftpad.GetDay(srv, authpad.Pad, shift.BeginTime(), authpad.Location)
	if err != nil {
		return InternalServerError(err)
	}

	var minDate = time.Date(2000, time.January, 1, 0, 0, 0, 0, authpad.Location)
	if authpad.EditRetroAlways {
		minDate = time.Now()
	}

	if err := html.ShiftEdit.Execute(w, html.ShiftEditData{
		PadData: html.PadData{
			Pad: authpad,
		},
		Day:     day,
		MaxDate: time.Now().Add(shiftpad.MaxFuture).Format("2006-01-02"),
		MinDate: minDate.Format("2006-01-02"),
		Shift:   shift,
		Error:   errMsg,
	}); err != nil {
		return InternalServerError(err)
	}
	return nil
}

func (srv *Server) shiftEditPost(w http.ResponseWriter, r *http.Request, authpad shiftpad.AuthPad, shift *shiftpad.Shift) http.Handler {
	if !authpad.CanEditShift(*shift) {
		return NotFound()
	}

	begin, _ := time.ParseInLocation("2006-01-02 15:04", r.PostFormValue("begin-date")+" "+r.PostFormValue("begin-time"), authpad.Location)
	end, _ := time.ParseInLocation("2006-01-02 15:04", r.PostFormValue("end-date")+" "+r.PostFormValue("end-time"), authpad.Location)
	if err := shiftpad.CheckBeginEnd(begin, end, authpad.EditRetroAlways, shiftpad.MaxFuture); err != nil {
		return srv.shiftEditTemplate(w, r, authpad, shift, err.Error())
	}

	name := trim(r.PostFormValue("name"), 64)
	note := trim(r.PostFormValue("note"), 64)
	eventUID := trim(r.PostFormValue("event-uid"), 128)
	takerName := trim(r.PostFormValue("taker-name"), 64)
	takerContact := trim(r.PostFormValue("taker-contact"), 128)

	shift.Name = name
	shift.Note = note
	shift.EventUID = eventUID
	shift.Begin = begin
	shift.End = end
	shift.Modified = time.Now()
	shift.TakerName = takerName
	shift.TakerContact = takerContact

	if !authpad.CanEditShift(*shift) {
		return NotFound()
	}

	if err := srv.DB.UpdateShift(authpad.Pad, shift); err != nil {
		return InternalServerError(err)
	}
	if err := srv.UpdatePadLastUpdated(authpad.Pad); err != nil {
		return InternalServerError(err)
	}

	return http.RedirectHandler(authpad.Link()+fmt.Sprintf("/day/%s", datefmt.ISODate(shift.BeginTime())), http.StatusSeeOther)
}

func (srv *Server) shiftTakeGet(w http.ResponseWriter, r *http.Request, authpad shiftpad.AuthPad, shift *shiftpad.Shift) http.Handler {
	if !authpad.CanTakeShift(*shift) {
		return NotFound()
	}

	return srv.shiftTakeTemplate(w, r, authpad, shift, "")
}

func (srv *Server) shiftTakeTemplate(w http.ResponseWriter, r *http.Request, authpad shiftpad.AuthPad, shift *shiftpad.Shift, errMsg string) http.Handler {
	day, err := shiftpad.GetDay(srv, authpad.Pad, shift.BeginTime(), authpad.Location)
	if err != nil {
		return InternalServerError(err)
	}

	if err := html.ShiftTake.Execute(w, html.ShiftTakeData{
		PadData: html.PadData{
			Pad: authpad,
		},
		Day:        day,
		Shift:      shift,
		TakerNames: authpad.TakerName,
		Error:      errMsg,
	}); err != nil {
		return InternalServerError(err)
	}
	return nil
}

func (srv *Server) shiftTakePost(w http.ResponseWriter, r *http.Request, authpad shiftpad.AuthPad, shift *shiftpad.Shift) http.Handler {
	if !authpad.CanTakeShift(*shift) {
		return NotFound()
	}

	takerName := trim(r.PostFormValue("taker-name"), 64)
	takerContact := trim(r.PostFormValue("taker-contact"), 128)
	if len(takerName) < 2 {
		return srv.shiftTakeTemplate(w, r, authpad, shift, "name must have at least two characters")
	}

	if !authpad.CanTakerName(*shift, takerName) {
		return NotFound()
	}

	shift.Modified = time.Now()
	shift.TakerName = takerName
	shift.TakerContact = takerContact

	if err := srv.DB.TakeShift(authpad.Pad, shift); err != nil {
		return InternalServerError(err)
	}
	if err := srv.UpdatePadLastUpdated(authpad.Pad); err != nil {
		return InternalServerError(err)
	}

	return http.RedirectHandler(authpad.Link()+fmt.Sprintf("/day/%s", datefmt.ISODate(shift.BeginTime())), http.StatusSeeOther)
}
