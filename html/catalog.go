// Code generated by running "go generate" in golang.org/x/text. DO NOT EDIT.

package html

import (
	"golang.org/x/text/language"
	"golang.org/x/text/message"
	"golang.org/x/text/message/catalog"
)

type dictionary struct {
	index []uint32
	data  string
}

func (d *dictionary) Lookup(key string) (data string, ok bool) {
	p, ok := messageKeyToIndex[key]
	if !ok {
		return "", false
	}
	start, end := d.index[p], d.index[p+1]
	if start == end {
		return "", false
	}
	return d.data[start:end], true
}

func init() {
	dict := map[string]catalog.Dictionary{
		"de_DE": &dictionary{index: de_DEIndex, data: de_DEData},
		"en_US": &dictionary{index: en_USIndex, data: en_USData},
	}
	fallback := language.MustParse("en-US")
	cat, err := catalog.NewFromMap(dict, catalog.Fallback(fallback))
	if err != nil {
		panic(err)
	}
	message.DefaultCatalog = cat
}

var messageKeyToIndex = map[string]int{
	"Administrate this Pad":          26,
	"Administrate this pad":          27,
	"Any shift":                      29,
	"Any taker name":                 38,
	"Apply":                          54,
	"Apply for Shifts":               34,
	"Apply for shift":                77,
	"Approve take":                   79,
	"Back":                           23,
	"Begin":                          66,
	"Begin must be before end.":      68,
	"Cancel":                         48,
	"Contact":                        74,
	"Copy iCalendar":                 60,
	"Copy link":                      25,
	"Create new Pad":                 3,
	"Create share link":              47,
	"Create shifts":                  52,
	"Create, Edit and Delete Shifts": 28,
	"Cron expression, example":       37,
	"Deadline (optional)":            36,
	"Delete":                         56,
	"Delete shift":                   72,
	"Description (Markdown)":         18,
	"Edit":                           55,
	"Edit retroactively":             30,
	"End":                            67,
	"Error":                          51,
	"Expires":                        45,
	"Link Properties":                44,
	"Link expires":                   59,
	"Location":                       19,
	"Mark any shift as paid out":     32,
	"Mark as paid out":               16,
	"Name":                           17,
	"Next":                           50,
	"No shifts or events yet.":       57,
	"No shifts.":                     13,
	"Note":                           46,
	"Paid out":                       8,
	"Paid shifts taken by":           14,
	"Payout":                         31,
	"Please use the full link.":      0,
	"Quantity":                       69,
	"Save":                           22,
	"Save changes":                   75,
	"Settings":                       61,
	"Share":                          62,
	"Shift":                          6,
	"Shift Names (one name per row)": 20,
	"Shift name":                     70,
	"Sorry, internal server error":   1,
	"Sorry, not found":               2,
	"Sum":                            12,
	"Take":                           53,
	"Take Shifts":                    33,
	"Take and Apply":                 35,
	"Take shift":                     78,
	"Take shifts as":                 40,
	"Taker":                          7,
	"Taker names":                    39,
	"These shifts have been marked as paid out for": 4,
	"This":                               49,
	"This is your customized share link": 24,
	"Time":                               5,
	"Unknown event":                      9,
	"Unnamed Pad":                        58,
	"View Shifts":                        41,
	"View taker contact":                 43,
	"View taker name":                    42,
	"applied":                            64,
	"do not assign to an event":          73,
	"hours":                              11,
	"ical Overlay":                       21,
	"last changed":                       63,
	"no shifts available":                71,
	"not paid out yet":                   76,
	"not yet approved":                   15,
	"paid":                               10,
	"paid out":                           65,
}

var de_DEIndex = []uint32{ // 81 elements
	// Entry 0 - 1F
	0x00000000, 0x00000028, 0x00000045, 0x0000005b,
	0x0000006d, 0x000000a1, 0x000000a6, 0x000000ae,
	0x000000b3, 0x000000be, 0x000000d0, 0x000000d8,
	0x000000e0, 0x000000e6, 0x000000f7, 0x0000010e,
	0x00000124, 0x0000013d, 0x00000142, 0x0000015a,
	0x00000163, 0x00000183, 0x00000190, 0x0000019a,
	0x000001a2, 0x000001ca, 0x000001d8, 0x000001f2,
	0x0000020c, 0x00000237, 0x00000244, 0x0000025c,
	// Entry 20 - 3F
	0x00000267, 0x0000028d, 0x000002a6, 0x000002be,
	0x000002e4, 0x000002f8, 0x00000315, 0x00000320,
	0x00000326, 0x00000340, 0x00000353, 0x00000362,
	0x00000373, 0x00000386, 0x00000392, 0x0000039a,
	0x000003b0, 0x000003ba, 0x000003c0, 0x000003c9,
	0x000003d0, 0x000003e2, 0x000003ec, 0x000003f5,
	0x00000400, 0x00000409, 0x00000434, 0x00000444,
	0x00000455, 0x0000046d, 0x0000047b, 0x00000482,
	// Entry 40 - 5F
	0x00000494, 0x0000049d, 0x000004a8, 0x000004af,
	0x000004b4, 0x000004d9, 0x000004e0, 0x000004e8,
	0x00000502, 0x00000513, 0x0000052b, 0x00000533,
	0x00000549, 0x0000055f, 0x00000574, 0x0000058b,
	0x0000059e,
} // Size: 348 bytes

const de_DEData string = "" + // Size: 1438 bytes
	"\x02Bitte verwende den vollständigen Link.\x02Sorry, interner Serverfehl" +
	"er\x02Sorry, nicht gefunden\x02Neues Pad anlegen\x02Diese Schichten wurd" +
	"en als ausbezahlt markiert für\x02Zeit\x02Schicht\x02Name\x02Ausbezahlt" +
	"\x02Unbekanntes Event\x02bezahlt\x02Stunden\x02Summe\x02Keine Schichten." +
	"\x02Bezahlte Schichten von\x02noch nicht angenommen\x02Als ausbezahlt ma" +
	"rkieren\x02Name\x02Beschreibung (Markdown)\x02Zeitzone\x02Schicht-Typen " +
	"(einer pro Zeile)\x02ical-Overlay\x02Speichern\x02Zurück\x02Dies ist dei" +
	"n gewünschter Freigabelink\x02Link kopieren\x02Dieses Pad administrieren" +
	"\x02Dieses Pad administrieren\x02Schichten anlegen, bearbeiten und lösch" +
	"en\x02Jede Schicht\x02Rückwirkend bearbeiten\x02Auszahlung\x02Jede Schic" +
	"ht als ausgezahlt markieren\x02Für Schichten eintragen\x02Für Schichten " +
	"bewerben\x02Für Schichten eintragen und bewerben\x02Deadline (optional)" +
	"\x02Cron-Ausdruck, beispielweise\x02Jeder Name\x02Namen\x02Schichten übe" +
	"rnehmen als\x02Schichten anzeigen\x02Namen anzeigen\x02Kontakt anzeigen" +
	"\x02Link-Eigenschaften\x02Gültig bis\x02Hinweis\x02Freigabelink erzeugen" +
	"\x02Abbrechen\x02Diese\x02Nächste\x02Fehler\x02Schichten anlegen\x02Eint" +
	"ragen\x02Bewerben\x02Bearbeiten\x02Löschen\x02Noch keine Schichten oder " +
	"Veranstaltungen.\x02Unbenanntes Pad\x02Link gültig bis\x02iCalendar-Link" +
	" kopieren\x02Einstellungen\x02Teilen\x02zuletzt geändert\x02beworben\x02" +
	"ausbezahlt\x02Beginn\x02Ende\x02Der Beginn muss vor dem Ende liegen.\x02" +
	"Anzahl\x02Schicht\x02keine Schichten vorhanden\x02Schicht löschen\x02kei" +
	"nem Event zugeordnet\x02Kontakt\x02Änderungen speichern\x02noch nicht au" +
	"sbezahlt\x02Auf Schicht bewerben\x02Für Schicht eintragen\x02Bewerbung a" +
	"nnehmen"

var en_USIndex = []uint32{ // 81 elements
	// Entry 0 - 1F
	0x00000000, 0x0000001a, 0x00000037, 0x00000048,
	0x00000057, 0x00000085, 0x0000008a, 0x00000090,
	0x00000096, 0x0000009f, 0x000000ad, 0x000000b2,
	0x000000b8, 0x000000bc, 0x000000c7, 0x000000dc,
	0x000000ed, 0x000000fe, 0x00000103, 0x0000011a,
	0x00000123, 0x00000142, 0x0000014f, 0x00000154,
	0x00000159, 0x0000017c, 0x00000186, 0x0000019c,
	0x000001b2, 0x000001d1, 0x000001db, 0x000001ee,
	// Entry 20 - 3F
	0x000001f5, 0x00000210, 0x0000021c, 0x0000022d,
	0x0000023c, 0x00000250, 0x00000269, 0x00000278,
	0x00000284, 0x00000293, 0x0000029f, 0x000002af,
	0x000002c2, 0x000002d2, 0x000002da, 0x000002df,
	0x000002f1, 0x000002f8, 0x000002fd, 0x00000302,
	0x00000308, 0x00000316, 0x0000031b, 0x00000321,
	0x00000326, 0x0000032d, 0x00000346, 0x00000352,
	0x0000035f, 0x0000036e, 0x00000377, 0x0000037d,
	// Entry 40 - 5F
	0x0000038a, 0x00000392, 0x0000039b, 0x000003a1,
	0x000003a5, 0x000003bf, 0x000003c8, 0x000003d3,
	0x000003e7, 0x000003f4, 0x0000040e, 0x00000416,
	0x00000423, 0x00000434, 0x00000444, 0x0000044f,
	0x0000045c,
} // Size: 348 bytes

const en_USData string = "" + // Size: 1116 bytes
	"\x02Please use the full link.\x02Sorry, internal server error\x02Sorry, " +
	"not found\x02Create new Pad\x02These shifts have been marked as paid out" +
	" for\x02Time\x02Shift\x02Taker\x02Paid out\x02Unknown event\x02paid\x02h" +
	"ours\x02Sum\x02No shifts.\x02Paid shifts taken by\x02not yet approved" +
	"\x02Mark as paid out\x02Name\x02Description (Markdown)\x02Location\x02Sh" +
	"ift Names (one name per row)\x02ical Overlay\x02Save\x02Back\x02This is " +
	"your customized share link\x02Copy link\x02Administrate this Pad\x02Admi" +
	"nistrate this pad\x02Create, Edit and Delete Shifts\x02Any shift\x02Edit" +
	" retroactively\x02Payout\x02Mark any shift as paid out\x02Take Shifts" +
	"\x02Apply for Shifts\x02Take and Apply\x02Deadline (optional)\x02Cron ex" +
	"pression, example\x02Any taker name\x02Taker names\x02Take shifts as\x02" +
	"View Shifts\x02View taker name\x02View taker contact\x02Link Properties" +
	"\x02Expires\x02Note\x02Create share link\x02Cancel\x02This\x02Next\x02Er" +
	"ror\x02Create shifts\x02Take\x02Apply\x02Edit\x02Delete\x02No shifts or " +
	"events yet.\x02Unnamed Pad\x02Link expires\x02Copy iCalendar\x02Settings" +
	"\x02Share\x02last changed\x02applied\x02paid out\x02Begin\x02End\x02Begi" +
	"n must be before end.\x02Quantity\x02Shift name\x02no shifts available" +
	"\x02Delete shift\x02do not assign to an event\x02Contact\x02Save changes" +
	"\x02not paid out yet\x02Apply for shift\x02Take shift\x02Approve take"

	// Total table size 3250 bytes (3KiB); checksum: 3EEF0747
