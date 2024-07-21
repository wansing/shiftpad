package main

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	goical "github.com/emersion/go-ical"
	"github.com/gorhill/cronexpr"
	"github.com/wansing/shiftpad"
	"github.com/wansing/shiftpad/html"
	"github.com/wansing/shiftpad/html/static"
	"github.com/wansing/shiftpad/ical"
	"github.com/wansing/shiftpad/sqlite"
	"github.com/wansing/shiftpad/way"
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
	router.Handle(http.MethodGet, "/p/:pad/:auth/:authsig/apply/:shift", srv.withShift(srv.shiftApplyGet))
	router.Handle(http.MethodPost, "/p/:pad/:auth/:authsig/apply/:shift", srv.withShift(srv.shiftApplyPost))
	router.Handle(http.MethodGet, "/p/:pad/:auth/:authsig/payout", srv.withPad(srv.padPayoutGet))
	router.Handle(http.MethodGet, "/p/:pad/:auth/:authsig/payout/:taker", srv.withPad(srv.padPayoutTakerGet))
	router.Handle(http.MethodPost, "/p/:pad/:auth/:authsig/payout/:taker", srv.withPad(srv.padPayoutTakerPost))
	router.Handle(http.MethodGet, "/p/:pad/:auth/:authsig/payout/:taker/result", srv.withPad(srv.padPayoutTakerResultGet))
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
	router.Handle(http.MethodGet, "/p/:pad/:auth/:authsig/approve/:shift/:take", srv.withTake(srv.takeApproveGet))
	router.Handle(http.MethodPost, "/p/:pad/:auth/:authsig/approve/:shift/:take", srv.withTake(srv.takeApprovePost))
	router.Handle(http.MethodGet, "/p/:pad/:auth/:authsig/take/:shift", srv.withShift(srv.shiftTakeGet))
	router.Handle(http.MethodPost, "/p/:pad/:auth/:authsig/take/:shift", srv.withShift(srv.shiftTakePost))
	router.Handle(http.MethodGet, "/p/:pad/:auth/:authsig/edit/:shift", srv.withShift(srv.shiftEditGet))
	router.Handle(http.MethodPost, "/p/:pad/:auth/:authsig/edit/:shift", srv.withShift(srv.shiftEditPost))
	router.Handle(http.MethodGet, "/p/:pad/:auth/:authsig/delete/:shift", srv.withShift(srv.shiftDeleteGet))
	router.Handle(http.MethodPost, "/p/:pad/:auth/:authsig/delete/:shift", srv.withShift(srv.shiftDeletePost))

	log.Println("listening to 127.0.0.1:8200")
	http.ListenAndServe("127.0.0.1:8200", srv.sessionManager.LoadAndSave(router))
}

func linkDay(authpad shiftpad.AuthPad, t time.Time) string {
	return fmt.Sprintf("%s/day/%s", authpad.Link(), t.Format("2006-01-02"))
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

// withShift calls handlers with a pad and a shift. It calls GetShift which ensures that the shift belongs to the pad.
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

func (srv *Server) withTake(f func(http.ResponseWriter, *http.Request, shiftpad.AuthPad, *shiftpad.Shift, shiftpad.Take) http.Handler) HandlerFunc {
	return srv.withShift(func(w http.ResponseWriter, r *http.Request, authpad shiftpad.AuthPad, shift *shiftpad.Shift) http.Handler {
		id, _ := strconv.Atoi(way.Param(r.Context(), "take"))
		// linear search
		for _, take := range shift.Takes {
			if take.ID == id {
				return f(w, r, authpad, shift, take)
			}
		}
		return NotFound()
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
		html.InternalServerError.Execute(w, html.MakeLayoutData(r))
	})
}

func NotFound() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		html.NotFound.Execute(w, html.MakeLayoutData(r))
	})
}

func (srv *Server) indexGet(w http.ResponseWriter, r *http.Request) http.Handler {
	err := html.Index.Execute(w, html.MakeLayoutData(r))
	if err != nil {
		return InternalServerError(err)
	}
	return nil
}

func (srv *Server) createGet(w http.ResponseWriter, r *http.Request) http.Handler {
	err := html.PadCreate.Execute(w, html.MakeLayoutData(r))
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
			Admin: true,
			Note:  "Admin-Link",
		},
		Pad: &newPad,
	}
	return http.RedirectHandler(authpad.Link(), http.StatusSeeOther)
}

func (srv *Server) padPayoutGet(w http.ResponseWriter, r *http.Request, authpad shiftpad.AuthPad) http.Handler {
	if !authpad.CanPayout() {
		return NotFound()
	}

	takerNames, err := srv.DB.GetTakerNames(authpad.Pad)
	if err != nil {
		return InternalServerError(err)
	}

	err = html.PadPayout.Execute(w, html.PadPayoutData{
		PadData: html.PadData{
			LayoutData: html.MakeLayoutData(r),
			ActiveTab:  "payout",
			Pad:        authpad,
		},
		TakerNames: takerNames,
	})
	if err != nil {
		return InternalServerError(err)
	}
	return nil
}

func (srv *Server) padPayoutTakerGet(w http.ResponseWriter, r *http.Request, authpad shiftpad.AuthPad) http.Handler {
	if !authpad.CanPayout() {
		return NotFound()
	}

	takerName := way.Param(r.Context(), "taker")
	shifts, err := srv.DB.GetTakesByTaker(authpad.Pad, takerName)
	if err != nil {
		return InternalServerError(err)
	}
	// don't filter by CanPayoutTake because we also list takes which are paid out
	// don't restrict to paid shifts because shift.paid may have been changed after payout

	// group consecutive shifts by event
	var events []shiftpad.Event
	var lastUID string
	for _, shift := range shifts {
		if !shift.Paid && !shift.HasPayouts() {
			continue
		}

		if shift.EventUID != "" && shift.EventUID == lastUID {
			events[len(events)-1].Shifts = append(events[len(events)-1].Shifts, shift)
		} else {
			events = append(events, shiftpad.Event{
				Shifts: []shiftpad.Shift{shift},
			})
		}
		lastUID = shift.EventUID
	}

	// collect ical events if exist
	icalEvents, _ := srv.GetICalFeedCache(authpad.ICalOverlay).Get(authpad.Location)
	var icalMap = make(map[string]*ical.Event)
	for _, icalEvent := range icalEvents {
		icalMap[icalEvent.UID] = &icalEvent
	}
	for i, event := range events {
		uid := event.Shifts[0].EventUID
		if icalEvent, ok := icalMap[uid]; ok {
			events[i].Event = icalEvent
		}
	}

	err = html.PadPayoutTaker.Execute(w, html.PadPayoutTakerData{
		PadData: html.PadData{
			LayoutData: html.MakeLayoutData(r),
			ActiveTab:  "payout",
			Pad:        authpad,
		},
		Name:   takerName,
		Events: events,
	})
	if err != nil {
		return InternalServerError(err)
	}
	return nil
}

func (srv *Server) padPayoutTakerPost(w http.ResponseWriter, r *http.Request, authpad shiftpad.AuthPad) http.Handler {
	if !authpad.CanPayout() {
		return NotFound()
	}

	// collect input
	if err := r.ParseForm(); err != nil {
		return InternalServerError(err)
	}
	var setPaidOut = make(map[int]any)
	for _, idstr := range r.PostForm["take"] {
		id, _ := strconv.Atoi(idstr)
		setPaidOut[id] = struct{}{}
	}

	// validate input by iterating over GetTakesByTaker and checking CanPayoutTake
	takerName := way.Param(r.Context(), "taker")
	shifts, err := srv.DB.GetTakesByTaker(authpad.Pad, takerName)
	if err != nil {
		return InternalServerError(err)
	}
	var updateTakes []shiftpad.Take
	for _, shift := range shifts {
		for _, take := range shift.Takes {
			if authpad.CanPayoutTake(shift, take) {
				if _, ok := setPaidOut[take.ID]; ok {
					take.PaidOut = true
					updateTakes = append(updateTakes, take)
				}
			}
		}
	}
	if err := srv.DB.SetPaidOut(updateTakes); err != nil {
		return InternalServerError(err)
	}
	if err := srv.UpdatePadLastUpdated(authpad.Pad); err != nil {
		return InternalServerError(err)
	}

	if len(updateTakes) > 0 {
		var query = make(url.Values)
		for _, updated := range updateTakes {
			query.Add("paid_out", strconv.Itoa(updated.ID))
		}
		var u = *r.URL // copy struct
		u.Path += "/result"
		u.RawQuery = query.Encode()
		return http.RedirectHandler(u.String(), http.StatusSeeOther)
	} else {
		return http.RedirectHandler(r.URL.String(), http.StatusSeeOther) // no shift checked, redirect to payoutTaker
	}
}

func (srv *Server) padPayoutTakerResultGet(w http.ResponseWriter, r *http.Request, authpad shiftpad.AuthPad) http.Handler {
	if !authpad.CanPayout() {
		return NotFound()
	}

	// ids from url query
	var takeIDs = make(map[int]any)
	for _, s := range r.URL.Query()["paid_out"] {
		if id, err := strconv.Atoi(s); err == nil {
			takeIDs[id] = struct{}{}
		}
	}

	// collect input by iterating over GetTakesByTaker, don't check CanPayoutTake because displayed takes are already paid out
	takerName := way.Param(r.Context(), "taker")
	shifts, err := srv.DB.GetTakesByTaker(authpad.Pad, takerName)
	if err != nil {
		return InternalServerError(err)
	}
	for i := range shifts {
		// filter takes
		n := 0
		for _, take := range shifts[i].Takes {
			if _, ok := takeIDs[take.ID]; ok {
				shifts[i].Takes[n] = take
				n++
			}
		}
		shifts[i].Takes = shifts[i].Takes[:n]
	}

	// filter empty shifts so they don't disturb the table
	shifts = slices.DeleteFunc(shifts, func(shift shiftpad.Shift) bool {
		return !shift.Paid && !shift.HasPayouts()
	})

	// group consecutive shifts by event (copied from above)
	var events []shiftpad.Event
	var lastUID string
	for _, shift := range shifts {
		if shift.EventUID != "" && shift.EventUID == lastUID {
			events[len(events)-1].Shifts = append(events[len(events)-1].Shifts, shift)
		} else {
			events = append(events, shiftpad.Event{
				Shifts: []shiftpad.Shift{shift},
			})
		}
		lastUID = shift.EventUID
	}

	err = html.PadPayoutTakerResult.Execute(w, html.PadPayoutTakerResultData{
		PadData: html.PadData{
			LayoutData: html.MakeLayoutData(r),
			ActiveTab:  "payout",
			Pad:        authpad,
		},
		Name:   takerName,
		Events: events,
	})
	if err != nil {
		return InternalServerError(err)
	}
	return nil
}

func (srv *Server) padSettingsGet(w http.ResponseWriter, r *http.Request, authpad shiftpad.AuthPad) http.Handler {
	if !authpad.Admin {
		return NotFound()
	}

	err := html.PadSettings.Execute(w, html.PadSettingsData{
		PadData: html.PadData{
			LayoutData: html.MakeLayoutData(r),
			ActiveTab:  "settings",
			Pad:        authpad,
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
			LayoutData: html.MakeLayoutData(r),
			ActiveTab:  "share",
			Pad:        authpad,
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

	apply := shiftpad.Intersect(authpad.ShiftNames, r.PostForm["apply"])
	edit := shiftpad.Intersect(authpad.ShiftNames, r.PostForm["edit"])
	take := shiftpad.Intersect(authpad.ShiftNames, r.PostForm["take"])
	takerName := split(r.PostFormValue("taker-name")) // taker-name is a textarea, not checkboxes

	shareAuth := shiftpad.Auth{
		Admin:            r.PostFormValue("admin") != "",
		Apply:            apply,
		ApplyAll:         r.PostFormValue("apply-all") != "",
		Edit:             edit,
		EditAll:          r.PostFormValue("edit-all") != "",
		EditRetroAlways:  r.PostFormValue("edit-retro-always") != "",
		Expires:          expires,
		Note:             trim(r.PostFormValue("note"), 128),
		PayoutAll:        r.PostFormValue("payout-all") != "",
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
			LayoutData: html.MakeLayoutData(r),
			Pad:        authpad,
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
	cal := goical.NewCalendar()
	cal.Props.SetText(goical.PropVersion, "2.0")
	cal.Props.SetText(goical.PropProductID, "shiftpad")

	from := time.Now().Add(-24 * time.Hour) // hardcoded, for shifts that have begun shortly before and have no end time
	to := time.Now().Add(shiftpad.MaxFuture)
	shifts, err := srv.DB.GetShifts(authpad.Pad, from.Unix(), to.Unix())
	if err != nil {
		return InternalServerError(err)
	}

	for _, shift := range shifts {
		uid := fmt.Sprintf("%d@%s", shift.ID, authpad.Pad.ID)

		var summary strings.Builder
		summary.WriteString(shift.Name)
		if shift.Note != "" {
			summary.WriteString("(")
			summary.WriteString(shift.Note)
			summary.WriteString(")")
		}
		if takers := shift.TakeViews(authpad.Auth); len(takers) > 0 {
			summary.WriteString(":")
			for _, taker := range takers {
				summary.WriteString("\n")
				summary.WriteString(taker.String())
			}
		}

		event := goical.NewEvent()
		event.Props.SetText(goical.PropUID, uid)
		event.Props.SetText(goical.PropSummary, summary.String())
		// use UTC ("Z") because go-ical can't export timezone details
		event.Props.SetDateTime(goical.PropDateTimeStamp, shift.Modified.In(time.UTC))
		event.Props.SetDateTime(goical.PropDateTimeStart, shift.Begin.In(time.UTC))
		event.Props.SetDateTime(goical.PropDateTimeEnd, shift.End.In(time.UTC))
		cal.Children = append(cal.Children, event.Component)
	}

	w.Header().Add("Content-Type", "text/calendar")

	err = goical.NewEncoder(w).Encode(cal)
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

	errs, _ := srv.sessionManager.Pop(r.Context(), "errs").([]string)

	if err := html.PadViewWeek.Execute(w, html.PadViewWeekData{
		PadData: html.PadData{
			LayoutData: html.MakeLayoutData(r),
			Pad:        authpad,
			Errors:     errs,
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

	var minDate = time.Now()
	if authpad.EditRetroAlways {
		minDate = time.Date(2000, time.January, 1, 0, 0, 0, 0, authpad.Location)
	}

	if err := html.ShiftCreate.Execute(w, html.ShiftCreateData{
		PadData: html.PadData{
			LayoutData: html.MakeLayoutData(r),
			Pad:        authpad,
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

	eventUID := trim(r.PostFormValue("event-uid"), 128)

	quantites := r.PostForm["quantity"]
	begins := r.PostForm["begin"]
	ends := r.PostForm["end"]
	names := r.PostForm["name"]
	notes := r.PostForm["note"]
	paids := r.PostForm["paid"]

	var errs []string
	for i := 0; i < min(len(quantites), len(begins), len(ends), len(names), len(notes)); i++ {
		quantity, err := strconv.Atoi(quantites[i])
		if err != nil {
			continue
		}
		if quantity < 1 {
			quantity = 1
		}
		if quantity > 64 {
			quantity = 64
		}

		begin, err := time.ParseInLocation("2006-01-02T15:04", begins[i], authpad.Location)
		if err != nil {
			continue
		}
		end, err := time.ParseInLocation("2006-01-02T15:04", ends[i], authpad.Location)
		if err != nil {
			continue
		}
		if err := shiftpad.CheckBeginEnd(begin, end, authpad.EditRetroAlways, shiftpad.MaxFuture); err != nil {
			errs = append(errs, fmt.Sprintf("adding row %d: %v", i+1, err))
			continue
		}

		name := trim(names[i], 64)
		if name == "" {
			continue
		}
		note := trim(notes[i], 64)
		paid := slices.Contains(paids, strconv.Itoa(i)) // Checkbox form input is sparse, so we can't use its indices. Instead we have submitted the form row indices.

		shift := shiftpad.Shift{
			Name:     name,
			Note:     note,
			Paid:     paid,
			Modified: time.Now().In(authpad.Location),
			EventUID: eventUID,
			Quantity: quantity,
			Begin:    begin,
			End:      end,
		}

		if !authpad.CanEditShift(shift) {
			errs = append(errs, fmt.Sprintf("adding row %d: unauthorized: %v", i+1, err))
			continue
		}

		if err := srv.DB.AddShift(authpad.Pad, shift); err != nil {
			return InternalServerError(err)
		}
	}
	srv.sessionManager.Put(r.Context(), "errs", errs)

	if err := srv.UpdatePadLastUpdated(authpad.Pad); err != nil {
		return InternalServerError(err)
	}

	redirectDate := way.Param(r.Context(), "date") // alternative: min shift or event begin time
	return http.RedirectHandler(authpad.Link()+fmt.Sprintf("/day/%s", redirectDate), http.StatusSeeOther)
}

func (srv *Server) shiftDeleteGet(w http.ResponseWriter, r *http.Request, authpad shiftpad.AuthPad, shift *shiftpad.Shift) http.Handler {
	if !authpad.CanEditShift(*shift) {
		return NotFound()
	}

	day, err := shiftpad.GetDay(srv, authpad.Pad, shift.Begin, authpad.Location)
	if err != nil {
		return InternalServerError(err)
	}

	if err := html.ShiftDelete.Execute(w, html.ShiftDeleteData{
		PadData: html.PadData{
			LayoutData: html.MakeLayoutData(r),
			Pad:        authpad,
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

	if err := srv.DB.DeleteShift(shift); err != nil {
		return InternalServerError(err)
	}
	if err := srv.UpdatePadLastUpdated(authpad.Pad); err != nil {
		return InternalServerError(err)
	}

	return http.RedirectHandler(linkDay(authpad, shift.Begin), http.StatusSeeOther)
}

func (srv *Server) shiftEditGet(w http.ResponseWriter, r *http.Request, authpad shiftpad.AuthPad, shift *shiftpad.Shift) http.Handler {
	if !authpad.CanEditShift(*shift) {
		return NotFound()
	}

	return srv.shiftEditTemplate(w, r, authpad, shift, "")
}

func (srv *Server) shiftEditTemplate(w http.ResponseWriter, r *http.Request, authpad shiftpad.AuthPad, shift *shiftpad.Shift, errMsg string) http.Handler {
	day, err := shiftpad.GetDay(srv, authpad.Pad, shift.Begin, authpad.Location)
	if err != nil {
		return InternalServerError(err)
	}

	var minDate = time.Now()
	if authpad.EditRetroAlways {
		minDate = time.Date(2000, time.January, 1, 0, 0, 0, 0, authpad.Location)
	}

	if err := html.ShiftEdit.Execute(w, html.ShiftEditData{
		PadData: html.PadData{
			LayoutData: html.MakeLayoutData(r),
			Pad:        authpad,
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

	begin, _ := time.ParseInLocation("2006-01-02T15:04", r.PostFormValue("begin"), authpad.Location)
	end, _ := time.ParseInLocation("2006-01-02T15:04", r.PostFormValue("end"), authpad.Location)
	if err := shiftpad.CheckBeginEnd(begin, end, authpad.EditRetroAlways, shiftpad.MaxFuture); err != nil {
		return srv.shiftEditTemplate(w, r, authpad, shift, err.Error())
	}

	quantity, _ := strconv.Atoi(r.PostFormValue("quantity"))
	quantity = min(quantity, 64)
	name := trim(r.PostFormValue("name"), 64)
	note := trim(r.PostFormValue("note"), 64)
	paid := r.PostFormValue("paid") != ""
	eventUID := trim(r.PostFormValue("event-uid"), 128)

	var takes []shiftpad.Take
	// existing takes (take.ID must not be user input)
	for _, take := range shift.Takes {
		takerName := trim(r.PostFormValue(fmt.Sprintf("taker-name-%d", take.ID)), 64)
		takerContact := trim(r.PostFormValue(fmt.Sprintf("taker-contact-%d", take.ID)), 128)
		takeApproved := r.PostFormValue(fmt.Sprintf("take-approved-%d", take.ID)) != ""
		if takerName != "" {
			takes = append(takes, shiftpad.Take{
				ID:       take.ID, // keep existing id
				Name:     takerName,
				Contact:  takerContact,
				Approved: takeApproved,
				PaidOut:  take.PaidOut, // keep existing payments
			})
		}
	}
	// new takes
	var newNames = r.PostForm["new-taker-name"]
	if len(newNames) > 64 {
		newNames = newNames[:64]
	}
	var newContacts = r.PostForm["new-taker-contact"]
	if len(newContacts) > 64 {
		newContacts = newContacts[:64]
	}
	var newApproved = r.PostForm["new-take-approved"]
	if len(newApproved) > 64 {
		newApproved = newApproved[:64]
	}
	maxNew := min(quantity-len(takes), len(newNames), len(newContacts))
	for i := 0; i < maxNew; i++ {
		takerName := trim(newNames[i], 64)
		takerContact := trim(newContacts[i], 128)
		takeApproved := slices.Contains(newApproved, strconv.Itoa(i)) // Checkbox form input is sparse, so we can't use its indices. Instead we have submitted the form row indices.
		if takerName != "" {
			takes = append(takes, shiftpad.Take{
				Name:     takerName,
				Contact:  takerContact,
				Approved: takeApproved,
			})
		}
	}

	shift.Name = name
	shift.Note = note
	shift.Paid = paid
	shift.EventUID = eventUID
	shift.Quantity = quantity
	shift.Begin = begin
	shift.End = end
	shift.Modified = time.Now()
	shift.Takes = takes

	if !authpad.CanEditShift(*shift) {
		return NotFound()
	}

	if err := srv.DB.UpdateShift(authpad.Pad, shift); err != nil {
		return InternalServerError(err)
	}
	if err := srv.UpdatePadLastUpdated(authpad.Pad); err != nil {
		return InternalServerError(err)
	}

	return http.RedirectHandler(linkDay(authpad, shift.Begin), http.StatusSeeOther)
}

func (srv *Server) shiftApplyGet(w http.ResponseWriter, r *http.Request, authpad shiftpad.AuthPad, shift *shiftpad.Shift) http.Handler {
	if !authpad.CanApplyShift(*shift) {
		return NotFound()
	}

	day, err := shiftpad.GetDay(srv, authpad.Pad, shift.Begin, authpad.Location)
	if err != nil {
		return InternalServerError(err)
	}

	if err := html.ShiftTake.Execute(w, html.ShiftTakeData{
		PadData: html.PadData{
			LayoutData: html.MakeLayoutData(r),
			Pad:        authpad,
		},
		Apply:      true,
		Day:        day,
		Shift:      shift,
		TakerNames: authpad.TakerName,
	}); err != nil {
		return InternalServerError(err)
	}
	return nil
}

func (srv *Server) shiftApplyPost(w http.ResponseWriter, r *http.Request, authpad shiftpad.AuthPad, shift *shiftpad.Shift) http.Handler {
	if !authpad.CanApplyShift(*shift) {
		return NotFound()
	}

	takerName := trim(r.PostFormValue("taker-name"), 64)
	takerContact := trim(r.PostFormValue("taker-contact"), 128)
	if takerName == "" {
		return http.RedirectHandler(linkDay(authpad, shift.Begin), http.StatusSeeOther)
	}
	if !authpad.CanApplyName(*shift, takerName) {
		return NotFound()
	}

	take := shiftpad.Take{
		Name:     takerName,
		Contact:  takerContact,
		Approved: false,
	}
	if err := srv.DB.TakeShift(authpad.Pad, shift, take); err != nil {
		return InternalServerError(err)
	}
	if err := srv.UpdatePadLastUpdated(authpad.Pad); err != nil {
		return InternalServerError(err)
	}

	return http.RedirectHandler(linkDay(authpad, shift.Begin), http.StatusSeeOther)
}

func (srv *Server) takeApproveGet(w http.ResponseWriter, r *http.Request, authpad shiftpad.AuthPad, shift *shiftpad.Shift, take shiftpad.Take) http.Handler {
	if !authpad.CanTakerName(*shift, take.Name) {
		return NotFound()
	}

	day, err := shiftpad.GetDay(srv, authpad.Pad, shift.Begin, authpad.Location)
	if err != nil {
		return InternalServerError(err)
	}

	if err := html.TakeApprove.Execute(w, html.TakeApproveData{
		PadData: html.PadData{
			LayoutData: html.MakeLayoutData(r),
			Pad:        authpad,
		},
		Day:   day,
		Shift: shift,
		Take:  take,
	}); err != nil {
		return InternalServerError(err)
	}
	return nil
}

func (srv *Server) takeApprovePost(w http.ResponseWriter, r *http.Request, authpad shiftpad.AuthPad, shift *shiftpad.Shift, take shiftpad.Take) http.Handler {
	if !authpad.CanTakerName(*shift, take.Name) {
		return NotFound()
	}

	if err := srv.DB.ApproveTake(shift, take); err != nil {
		return InternalServerError(err)
	}
	if err := srv.UpdatePadLastUpdated(authpad.Pad); err != nil {
		return InternalServerError(err)
	}

	return http.RedirectHandler(linkDay(authpad, shift.Begin), http.StatusSeeOther)
}

func (srv *Server) shiftTakeGet(w http.ResponseWriter, r *http.Request, authpad shiftpad.AuthPad, shift *shiftpad.Shift) http.Handler {
	if !authpad.CanTakeShift(*shift) {
		return NotFound()
	}

	day, err := shiftpad.GetDay(srv, authpad.Pad, shift.Begin, authpad.Location)
	if err != nil {
		return InternalServerError(err)
	}

	if err := html.ShiftTake.Execute(w, html.ShiftTakeData{
		PadData: html.PadData{
			LayoutData: html.MakeLayoutData(r),
			Pad:        authpad,
		},
		Apply:      false,
		Day:        day,
		Shift:      shift,
		TakerNames: authpad.TakerName,
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
	if takerName == "" {
		return http.RedirectHandler(linkDay(authpad, shift.Begin), http.StatusSeeOther)
	}
	if !authpad.CanTakerName(*shift, takerName) {
		return NotFound()
	}

	take := shiftpad.Take{
		Name:     takerName,
		Contact:  takerContact,
		Approved: true,
	}
	if err := srv.DB.TakeShift(authpad.Pad, shift, take); err != nil {
		return InternalServerError(err)
	}
	if err := srv.UpdatePadLastUpdated(authpad.Pad); err != nil {
		return InternalServerError(err)
	}

	return http.RedirectHandler(linkDay(authpad, shift.Begin), http.StatusSeeOther)
}
