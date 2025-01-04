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
			// InsecureSkipVerify:
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
func (cache *FeedCache) Get(location *time.Location) ([]Event, error) {
	if cache.URL == "" {
		return nil, nil
	}

	if cache.Interval < 30*time.Second { // see also http client timeout
		cache.Interval = 2 * time.Minute
	}

	if time.Since(cache.lastChecked) < cache.Interval {
		return cache.events, nil
	}

	// first call takes the write lock and does the job, subsequent calls wait until the job is finished
	if cache.lock.TryLock() {
		defer cache.lock.Unlock()

		req, err := http.NewRequest(http.MethodHead, cache.URL, nil)
		if err != nil {
			return nil, err
		}
		if cache.Config.Username != "" {
			req.SetBasicAuth(cache.Config.Username, cache.Config.Password)
		}
		if t, ok := client.Transport.(*http.Transport); ok {
			t.TLSClientConfig.InsecureSkipVerify = cache.SkipTLSVerify
		}

		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}

		if lastModified, err := time.Parse("Mon, 02 Jan 2006 15:04:05 GMT", resp.Header.Get("Last-Modified")); err == nil {
			if lastModified.Before(cache.lastModified) || lastModified.Equal(cache.lastModified) {
				return cache.events, nil
			}
			cache.lastModified = lastModified
		}

		req, err = http.NewRequest(http.MethodGet, cache.URL, nil)
		if err != nil {
			return nil, err
		}
		if cache.Config.Username != "" {
			req.SetBasicAuth(cache.Config.Username, cache.Config.Password)
		}

		resp, err = client.Do(req)
		if err != nil {
			return nil, err
		}

		cal, err := ical.NewDecoder(resp.Body).Decode()
		if err == io.EOF { // no calendars in file
			cache.events = nil
			return cache.events, nil
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

			// replace TZIDs which can't be loaded by time.LoadLocation (workaround for https://github.com/emersion/go-ical/issues/10) with target location
			for _, propid := range []string{ical.PropDateTimeStart, ical.PropDateTimeEnd} {
				prop := event.Props.Get(propid)
				if prop != nil {
					// similar to https://github.com/emersion/go-ical/blob/fc1c9d8fb2b6/ical.go#L149C6-L149C58
					if tzid := prop.Params.Get(ical.PropTimezoneID); tzid != "" {
						_, err := time.LoadLocation(tzid)
						if err != nil {
							prop.Params.Set(ical.PropTimezoneID, location.String())
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

		cache.events = events
		cache.lastChecked = time.Now()
	} else {
		// wait until write lock is released
		cache.lock.RLock()
		cache.lock.RUnlock()
	}

	return cache.events, nil
}
