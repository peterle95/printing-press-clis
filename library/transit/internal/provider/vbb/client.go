package vbb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"transit-pp-cli/internal/transit"
)

type Client struct {
	BaseURL string
	HTTP    *http.Client
	Debug   bool
	Stderr  io.Writer
	limiter *rateLimiter
}

type JourneyQuery struct {
	FromLatitude  float64
	FromLongitude float64
	FromAddress   string
	ToID          string
	ToLatitude    float64
	ToLongitude   float64
	ToAddress     string
	Results       int
	Arrival       *time.Time
	Departure     *time.Time
	Remarks       bool
	Stopovers     bool
	StartWalking  bool
}

func New(baseURL string, timeout time.Duration) *Client {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = "https://v6.vbb.transport.rest"
	}
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		HTTP: &http.Client{
			Timeout: timeout,
		},
		Stderr:  os.Stderr,
		limiter: newRateLimiter(100, time.Minute),
	}
}

func (c *Client) Locations(ctx context.Context, query string, results int, stops bool, addresses bool, poi bool) ([]transit.Location, error) {
	params := url.Values{}
	params.Set("query", query)
	params.Set("results", strconv.Itoa(defaultInt(results, 5)))
	params.Set("stops", formatBool(stops))
	params.Set("addresses", formatBool(addresses))
	params.Set("poi", formatBool(poi))
	var out []transit.Location
	if err := c.get(ctx, "/locations", params, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) Nearby(ctx context.Context, latitude, longitude float64, results int, distance int, linesOfStops bool) ([]transit.Location, error) {
	params := url.Values{}
	params.Set("latitude", formatFloat(latitude))
	params.Set("longitude", formatFloat(longitude))
	params.Set("results", strconv.Itoa(defaultInt(results, 10)))
	params.Set("distance", strconv.Itoa(defaultInt(distance, 1000)))
	params.Set("linesOfStops", formatBool(linesOfStops))
	var out []transit.Location
	if err := c.get(ctx, "/locations/nearby", params, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) Departures(ctx context.Context, stopID string, durationMinutes int, results int, modes transit.ProductFlags) ([]transit.Departure, error) {
	params := url.Values{}
	params.Set("duration", strconv.Itoa(defaultInt(durationMinutes, 20)))
	params.Set("results", strconv.Itoa(defaultInt(results, 20)))
	params.Set("remarks", "true")
	setProductParams(params, modes)
	var out transit.DepartureResponse
	if err := c.get(ctx, "/stops/"+url.PathEscape(stopID)+"/departures", params, &out); err != nil {
		return nil, err
	}
	return out.Departures, nil
}

func (c *Client) Journeys(ctx context.Context, query JourneyQuery) (transit.JourneyResponse, error) {
	params := url.Values{}
	params.Set("from.latitude", formatFloat(query.FromLatitude))
	params.Set("from.longitude", formatFloat(query.FromLongitude))
	if query.FromAddress != "" {
		params.Set("from.address", query.FromAddress)
	}
	if query.ToID != "" {
		params.Set("to", query.ToID)
	} else {
		params.Set("to.latitude", formatFloat(query.ToLatitude))
		params.Set("to.longitude", formatFloat(query.ToLongitude))
		if query.ToAddress != "" {
			params.Set("to.address", query.ToAddress)
		}
	}
	params.Set("results", strconv.Itoa(defaultInt(query.Results, 3)))
	params.Set("remarks", formatBool(query.Remarks))
	params.Set("stopovers", formatBool(query.Stopovers))
	params.Set("startWithWalking", formatBool(query.StartWalking))
	if query.Arrival != nil {
		params.Set("arrival", query.Arrival.Format(time.RFC3339))
	}
	if query.Departure != nil {
		params.Set("departure", query.Departure.Format(time.RFC3339))
	}
	var out transit.JourneyResponse
	if err := c.get(ctx, "/journeys", params, &out); err != nil {
		return transit.JourneyResponse{}, err
	}
	return out, nil
}

func (c *Client) RefreshJourney(ctx context.Context, refreshToken string, stopovers bool, remarks bool) (transit.JourneyResponse, error) {
	params := url.Values{}
	params.Set("stopovers", formatBool(stopovers))
	params.Set("remarks", formatBool(remarks))
	var out transit.JourneyResponse
	if err := c.get(ctx, "/journeys/"+url.PathEscape(refreshToken), params, &out); err != nil {
		return transit.JourneyResponse{}, err
	}
	return out, nil
}

func (c *Client) Radar(ctx context.Context, box transit.BoundingBox, results int, durationMinutes int, frames int) (transit.RadarResponse, error) {
	params := url.Values{}
	params.Set("north", formatFloat(box.North))
	params.Set("west", formatFloat(box.West))
	params.Set("south", formatFloat(box.South))
	params.Set("east", formatFloat(box.East))
	params.Set("results", strconv.Itoa(defaultInt(results, 64)))
	params.Set("duration", strconv.Itoa(defaultInt(durationMinutes, 30)))
	params.Set("frames", strconv.Itoa(defaultInt(frames, 3)))
	var out transit.RadarResponse
	if err := c.get(ctx, "/radar", params, &out); err != nil {
		return transit.RadarResponse{}, err
	}
	return out, nil
}

func (c *Client) Trip(ctx context.Context, tripID string, lineName string) (transit.TripResponse, error) {
	params := url.Values{}
	params.Set("remarks", "true")
	params.Set("stopovers", "true")
	if strings.TrimSpace(lineName) != "" {
		params.Set("lineName", lineName)
	}
	var out transit.TripResponse
	if err := c.get(ctx, "/trips/"+url.PathEscape(tripID), params, &out); err != nil {
		return transit.TripResponse{}, err
	}
	return out, nil
}

func (c *Client) get(ctx context.Context, path string, params url.Values, dest any) error {
	u, err := url.Parse(c.BaseURL + path)
	if err != nil {
		return err
	}
	u.RawQuery = params.Encode()
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if c.limiter != nil {
			if err := c.limiter.Wait(ctx); err != nil {
				return err
			}
		}
		if c.Debug {
			fmt.Fprintf(c.stderr(), "GET %s\n", u.String())
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
		if err != nil {
			return err
		}
		req.Header.Set("Accept", "application/json")
		resp, err := c.HTTP.Do(req)
		if err != nil {
			lastErr = err
			if !isTransientErr(err) {
				return err
			}
		} else {
			body, readErr := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if readErr != nil {
				return readErr
			}
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				if err := json.Unmarshal(body, dest); err != nil {
					return fmt.Errorf("decode VBB response from %s: %w", u.String(), err)
				}
				return nil
			}
			lastErr = providerError(resp.StatusCode, body)
			if c.Debug {
				fmt.Fprintf(c.stderr(), "VBB error %d: %s\n", resp.StatusCode, truncate(string(body), 1200))
			}
			if resp.StatusCode != http.StatusTooManyRequests && (resp.StatusCode < 500 || resp.StatusCode > 599) {
				return lastErr
			}
		}
		delay := time.Duration(250*(1<<attempt)) * time.Millisecond
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	return lastErr
}

func providerError(status int, body []byte) error {
	var payload struct {
		Message string `json:"message"`
		Code    string `json:"code"`
	}
	if err := json.Unmarshal(body, &payload); err == nil && payload.Message != "" {
		if payload.Code != "" {
			return fmt.Errorf("VBB provider error %d %s: %s", status, payload.Code, payload.Message)
		}
		return fmt.Errorf("VBB provider error %d: %s", status, payload.Message)
	}
	return fmt.Errorf("VBB provider error %d: %s", status, truncate(string(body), 240))
}

func (c *Client) stderr() io.Writer {
	if c.Stderr != nil {
		return c.Stderr
	}
	return os.Stderr
}

func isTransientErr(err error) bool {
	return err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded)
}

func defaultInt(value, fallback int) int {
	if value <= 0 {
		return fallback
	}
	return value
}

func formatBool(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func formatFloat(value float64) string {
	return strconv.FormatFloat(value, 'f', 6, 64)
}

func setProductParams(params url.Values, modes transit.ProductFlags) {
	params.Set("suburban", formatBool(modes.Suburban))
	params.Set("subway", formatBool(modes.Subway))
	params.Set("tram", formatBool(modes.Tram))
	params.Set("bus", formatBool(modes.Bus))
	params.Set("ferry", formatBool(modes.Ferry))
	params.Set("express", formatBool(modes.Express))
	params.Set("regional", formatBool(modes.Regional))
}

func truncate(value string, limit int) string {
	value = strings.TrimSpace(value)
	if len(value) <= limit {
		return value
	}
	return value[:limit] + "..."
}

type rateLimiter struct {
	mu       sync.Mutex
	limit    int
	window   time.Duration
	requests []time.Time
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	return &rateLimiter{limit: limit, window: window}
}

func (r *rateLimiter) Wait(ctx context.Context) error {
	if r.limit <= 0 || r.window <= 0 {
		return nil
	}
	for {
		r.mu.Lock()
		now := time.Now()
		cutoff := now.Add(-r.window)
		kept := r.requests[:0]
		for _, t := range r.requests {
			if t.After(cutoff) {
				kept = append(kept, t)
			}
		}
		r.requests = kept
		if len(r.requests) < r.limit {
			r.requests = append(r.requests, now)
			r.mu.Unlock()
			return nil
		}
		wait := r.requests[0].Add(r.window).Sub(now)
		r.mu.Unlock()
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}
