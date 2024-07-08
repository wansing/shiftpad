package ical

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/emersion/go-ical"
)

var client = &http.Client{
	Timeout: 5 * time.Second,
	Transport: &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	},
}

// FeedCache gets events from an ical feed and caches them for the given interval.
// After each interval, calls to update have to wait until the http request is done.
type FeedCache struct {
	Config
	Interval     time.Duration // default is two minutes
	events       []Event
	lastChecked  time.Time
	lastModified time.Time
	lock         sync.RWMutex
}

// Get returns all events. The location parameter is only used if the ical data contains no TZID location.
func (fc *FeedCache) Get(location *time.Location) ([]Event, error) {
	if fc.URL == "" {
		return nil, nil
	}

	if fc.Interval < 30*time.Second { // see also http client timeout
		fc.Interval = 2 * time.Minute
	}

	if time.Since(fc.lastChecked) < fc.Interval {
		return fc.events, nil
	}

	// first call takes the write lock and does the job, subsequent calls wait until the job is finished
	if fc.lock.TryLock() {
		defer fc.lock.Unlock()

		req, err := http.NewRequest(http.MethodHead, fc.URL, nil)
		if err != nil {
			return nil, err
		}
		if fc.Username != "" {
			req.SetBasicAuth(fc.Username, fc.Password)
		}

		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}

		if lastModified, err := time.Parse("Mon, 02 Jan 2006 15:04:05 GMT", resp.Header.Get("Last-Modified")); err == nil {
			if lastModified.Before(fc.lastModified) || lastModified.Equal(fc.lastModified) {
				return fc.events, nil
			}
			fc.lastModified = lastModified
		}

		req, err = http.NewRequest(http.MethodGet, fc.URL, nil)
		if err != nil {
			return nil, err
		}
		if fc.Username != "" {
			req.SetBasicAuth(fc.Username, fc.Password)
		}

		resp, err = client.Do(req)
		if err != nil {
			return nil, err
		}

		cal, err := ical.NewDecoder(resp.Body).Decode()
		if err == io.EOF { // no calendars in file
			fc.events = nil
			return fc.events, nil
		}
		if err != nil {
			return nil, err
		}

		var events []Event
		for _, event := range cal.Events() {
			uid, err := event.Props.Text(ical.PropUID)
			if err != nil {
				return nil, fmt.Errorf("getting uid: %w", err)
			}
			summary, err := event.Props.Text(ical.PropSummary)
			if err != nil {
				return nil, fmt.Errorf("getting summary: %w", err)
			}
			url, err := event.Props.URI(ical.PropURL)
			if err != nil {
				return nil, fmt.Errorf("getting url: %w", err)
			}

			// remove TZIDs which can't be loaded by time.LoadLocation (workaround for https://github.com/emersion/go-ical/issues/10)
			for _, propid := range []string{ical.PropDateTimeStart, ical.PropDateTimeEnd} {
				prop := event.Props.Get(propid)
				if prop != nil {
					// similar to https://github.com/emersion/go-ical/blob/fc1c9d8fb2b6/ical.go#L149C6-L149C58
					if tzid := prop.Params.Get(ical.PropTimezoneID); tzid != "" {
						_, err := time.LoadLocation(tzid)
						if err != nil {
							prop.Params.Del(ical.PropTimezoneID)
						}
					}
				}
			}

			// go-ical: "Use the TZID location, if available."
			start, err := event.DateTimeStart(location)
			if err != nil {
				return nil, fmt.Errorf("getting start time: %w", err)
			}
			end, err := event.DateTimeEnd(location)
			if err != nil {
				return nil, fmt.Errorf("getting end time: %w", err)
			}

			var urlString string
			if url != nil {
				urlString = url.String()
			}

			events = append(events, Event{
				UID:     uid,
				Summary: summary,
				URL:     urlString,
				Start:   start,
				End:     end,
			})
		}

		fc.events = events
		fc.lastChecked = time.Now()
	} else {
		// wait until write lock is released
		fc.lock.RLock()
		fc.lock.RUnlock()
	}

	return fc.events, nil
}
