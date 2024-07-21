package html

import (
	"embed"
	"fmt"
	"html/template"
	"net/http"
	"strings"
	"time"

	"github.com/wansing/shiftpad"
	"gitlab.com/golang-commonmark/markdown"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

//go:embed *
var files embed.FS

var md = markdown.New(markdown.HTML(true), markdown.Linkify(false))

func dateTimeRef(t, reference time.Time) string {
	if t.Format(time.DateOnly) == reference.Format(time.DateOnly) {
		return t.Format("15:04")
	}
	return t.Format("2. Jan 2006 15:04")
}

func parse(fn ...string) *template.Template {
	return template.Must(template.New(fn[0]).Funcs(template.FuncMap{
		"FmtDate": func(t time.Time) string {
			return t.Format("Monday 2. Jan 2006")
		},
		"FmtDateTimeRef": dateTimeRef,
		"FmtDateTimeRange": func(begin, end time.Time) string {
			return fmt.Sprintf("%s – %s", begin.Format("2. Jan 2006 15:04"), dateTimeRef(end, begin)) // begin always contains day, end contains day only if it differs
		},
		"FmtDateTimeRangeRef": func(begin, end, reference time.Time) string {
			return fmt.Sprintf("%s – %s", dateTimeRef(begin, reference), dateTimeRef(end, reference))
		},
		"FmtISODate": func(t time.Time) string {
			return t.Format("2006-01-02")
		},
		"FmtISODateTime": func(t time.Time) string {
			return t.Format("2006-01-02T15:04")
		},
		"Join": func(elems []string) string {
			return strings.Join(elems, "\r\n")
		},
		"Markdown": func(input string) template.HTML {
			return template.HTML(md.RenderToString([]byte(input)))
		},
		"Max": func(a, b int) int {
			return max(a, b)
		},
	}).ParseFS(files, fn...))
}

var (
	Index                = parse("layout.html", "index.html")
	InternalServerError  = parse("layout.html", "internal-server-error.html")
	NotFound             = parse("layout.html", "not-found.html")
	PadCreate            = parse("layout.html", "pad-create.html")
	PadPayout            = parse("layout.html", "pad.html", "pad-payout.html")
	PadPayoutTaker       = parse("layout.html", "pad.html", "pad-payout-taker.html")
	PadPayoutTakerResult = parse("layout.html", "pad.html", "pad-payout-taker-result.html")
	PadSettings          = parse("layout.html", "pad.html", "pad-settings.html")
	PadShare             = parse("layout.html", "pad.html", "pad-share.html")
	PadShareResult       = parse("layout.html", "pad.html", "pad-share-result.html")
	PadViewWeek          = parse("layout.html", "pad.html", "pad-view-week.html")
	ShiftCreate          = parse("layout.html", "pad.html", "shift-create.html")
	ShiftDelete          = parse("layout.html", "pad.html", "shift-delete.html")
	ShiftEdit            = parse("layout.html", "pad.html", "shift-edit.html")
	ShiftTake            = parse("layout.html", "pad.html", "shift-take.html")
	TakeApprove          = parse("layout.html", "pad.html", "take-approve.html")
)

type Lang language.Tag

func (l Lang) Tr(key message.Reference, a ...interface{}) string {
	return message.NewPrinter(language.Tag(l)).Sprintf(key, a...)
}

// supported languages
var matcher = language.NewMatcher([]language.Tag{
	message.MatchLanguage("en-US"), // The first language is used as fallback.
	message.MatchLanguage("de-DE"),
})

type LayoutData struct {
	Lang
}

func MakeLayoutData(r *http.Request) LayoutData {
	tag, _ := language.MatchStrings(matcher, r.Header.Get("Accept-Language"))
	return LayoutData{
		Lang: Lang(tag),
	}
}

type PadData struct {
	LayoutData
	ActiveTab string
	Errors    []string
	Pad       shiftpad.AuthPad
}

type PadPayoutData struct {
	PadData
	TakerNames []string
}

type PadPayoutTakerData struct {
	PadData
	Name   string
	Events []shiftpad.Event
}

type PadPayoutTakerResultData PadPayoutTakerData

type PadSettingsData struct {
	PadData
	Error     string
	Locations []string
}

type PadShareData struct {
	PadData
}

type PadShareResultData struct {
	PadData
	Host string
	Link string
}

type PadViewWeekData struct {
	PadData
	ISOWeek     string
	Days        [7]*shiftpad.Day
	EarlierYear int
	EarlierWeek int
	LaterYear   int
	LaterWeek   int
	ThisYear    int
	ThisWeek    int
	NextYear    int
	NextWeek    int
}

type TakeApproveData struct {
	PadData
	Day   shiftpad.Day
	Shift *shiftpad.Shift
	Take  shiftpad.Take
}

type ShiftCreateData struct {
	PadData
	Day     shiftpad.Day
	MaxDate string
	MinDate string
	Error   string
}

type ShiftDeleteData struct {
	PadData
	Day   shiftpad.Day
	Shift *shiftpad.Shift
	Error string
}

type ShiftEditData struct {
	PadData
	Day     shiftpad.Day
	MaxDate string
	MinDate string
	Shift   *shiftpad.Shift
	Error   string
}

type ShiftTakeData struct {
	PadData
	Apply      bool
	Day        shiftpad.Day
	Shift      *shiftpad.Shift
	TakerNames []string
}
