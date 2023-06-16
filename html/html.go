package html

import (
	"embed"
	"html/template"
	"strings"

	"github.com/wansing/shiftpad"
	"github.com/wansing/shiftpad/datefmt"
	"gitlab.com/golang-commonmark/markdown"
)

//go:embed *
var files embed.FS

var md = markdown.New(markdown.HTML(true), markdown.Linkify(false))

func parse(fn ...string) *template.Template {
	return template.Must(template.New(fn[0]).Funcs(template.FuncMap{
		"FmtDate":          datefmt.Date,
		"FmtDateTime":      datefmt.DateTime,
		"FmtDateTimeRange": datefmt.DateTimeRange,
		"FmtISODate":       datefmt.ISODate,
		"FmtISOTime":       datefmt.ISOTime,
		"Join": func(elems []string) string {
			return strings.Join(elems, "\r\n")
		},
		"Markdown": func(input string) template.HTML {
			return template.HTML(md.RenderToString([]byte(input)))
		},
		"Max": func(a, b int) int {
			if a > b {
				return a
			} else {
				return b
			}
		},
	}).ParseFS(files, fn...))
}

var (
	Index               = parse("layout.html", "index.html")
	InternalServerError = parse("layout.html", "internal-server-error.html")
	NotFound            = parse("layout.html", "not-found.html")
	PadCreate           = parse("layout.html", "pad-create.html")
	PadSettings         = parse("layout.html", "pad.html", "pad-settings.html")
	PadShare            = parse("layout.html", "pad.html", "pad-share.html")
	PadShareResult      = parse("layout.html", "pad.html", "pad-share-result.html")
	PadViewWeek         = parse("layout.html", "pad.html", "pad-view-week.html")
	ShiftCreate         = parse("layout.html", "pad.html", "shift-create.html")
	ShiftDelete         = parse("layout.html", "pad.html", "shift-delete.html")
	ShiftEdit           = parse("layout.html", "pad.html", "shift-edit.html")
	ShiftTake           = parse("layout.html", "pad.html", "shift-take.html")
)

type PadData struct {
	ActiveTab string
	Pad       shiftpad.AuthPad
}

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
	Day        shiftpad.Day
	Shift      *shiftpad.Shift
	TakerNames []string
	Error      string
}
