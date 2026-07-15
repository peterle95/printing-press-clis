// Copyright 2026 peter-moelzer. Licensed under Apache-2.0. See LICENSE.

package googlecalendar

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"trello-calendar-pp-cli/internal/cliutil"
	"trello-calendar-pp-cli/internal/scheduling"
)

type Client struct {
	BaseURL     string
	CalendarID  string
	Timezone    *time.Location
	HTTPClient  *http.Client
	Sleep       func(time.Duration)
	limiter     *cliutil.AdaptiveLimiter
	confirmWait func(context.Context, time.Duration) error
}

type apiEventTime struct {
	Date     string `json:"date,omitempty"`
	DateTime string `json:"dateTime,omitempty"`
	TimeZone string `json:"timeZone,omitempty"`
}

type apiEvent struct {
	ID                 string       `json:"id,omitempty"`
	Summary            string       `json:"summary,omitempty"`
	Description        string       `json:"description,omitempty"`
	Status             string       `json:"status,omitempty"`
	ColorID            string       `json:"colorId,omitempty"`
	Start              apiEventTime `json:"start"`
	End                apiEventTime `json:"end"`
	ExtendedProperties struct {
		Private map[string]string `json:"private,omitempty"`
	} `json:"extendedProperties,omitempty"`
}

type colorsResponse struct {
	Event map[string]json.RawMessage `json:"event"`
}

type eventsResponse struct {
	Items         []apiEvent `json:"items"`
	NextPageToken string     `json:"nextPageToken"`
	AccessRole    string     `json:"accessRole"`
}

type APIError struct {
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("Google Calendar returned HTTP %d: %s", e.StatusCode, e.Body)
}

func NewClient(baseURL, calendarID string, location *time.Location, httpClient *http.Client) *Client {
	return &Client{BaseURL: strings.TrimRight(baseURL, "/"), CalendarID: calendarID, Timezone: location, HTTPClient: httpClient, Sleep: time.Sleep, limiter: cliutil.NewAdaptiveLimiter(0), confirmWait: waitForContext}
}

func (c *Client) SetRateLimit(requestsPerSecond float64) {
	c.limiter = cliutil.NewAdaptiveLimiter(requestsPerSecond)
}

func (c *Client) ListEvents(ctx context.Context, start, end time.Time) ([]scheduling.Event, error) {
	values := url.Values{
		"timeMin":      {start.Format(time.RFC3339)},
		"timeMax":      {end.Format(time.RFC3339)},
		"timeZone":     {c.Timezone.String()},
		"singleEvents": {"true"},
		"showDeleted":  {"true"},
		"maxResults":   {"2500"},
	}
	items, _, err := c.list(ctx, values)
	if err != nil {
		return nil, err
	}
	result := make([]scheduling.Event, 0, len(items))
	for _, item := range items {
		event, err := c.normalize(item)
		if err != nil {
			return nil, err
		}
		result = append(result, event)
	}
	scheduling.SortEvents(result)
	return result, nil
}

// PATCH: Fetch Calendar-provided event color IDs before assigning configured priority colors.
func (c *Client) ListEventColors(ctx context.Context) (map[string]bool, error) {
	var response colorsResponse
	if err := c.doJSON(ctx, http.MethodGet, c.BaseURL+"/colors", nil, &response); err != nil {
		return nil, err
	}
	colors := make(map[string]bool, len(response.Event))
	for id := range response.Event {
		colors[id] = true
	}
	return colors, nil
}

func (c *Client) list(ctx context.Context, values url.Values) ([]apiEvent, string, error) {
	var result []apiEvent
	accessRole := ""
	for page := 0; page < 10000; page++ {
		target := c.eventsURL() + "?" + values.Encode()
		var response eventsResponse
		if err := c.doJSON(ctx, http.MethodGet, target, nil, &response); err != nil {
			return nil, "", err
		}
		result = append(result, response.Items...)
		if response.AccessRole != "" {
			accessRole = response.AccessRole
		}
		if response.NextPageToken == "" {
			return result, accessRole, nil
		}
		values.Set("pageToken", response.NextPageToken)
	}
	return nil, "", fmt.Errorf("google calendar pagination exceeded safety limit")
}

func (c *Client) FindCard(ctx context.Context, boardID, cardID string) (bool, error) {
	id := scheduling.DeterministicEventID(c.CalendarID, boardID, cardID)
	var event apiEvent
	err := c.doJSON(ctx, http.MethodGet, c.eventsURL()+"/"+url.PathEscape(id), nil, &event)
	if err == nil {
		// PATCH: Only a live event is scheduling evidence; deleted IDs must be retried.
		return !strings.EqualFold(event.Status, "cancelled"), nil
	}
	var apiErr *APIError
	if errors.As(err, &apiErr) && apiErr != nil && apiErr.StatusCode == http.StatusGone {
		return false, nil
	}
	if apiErr == nil || apiErr.StatusCode != http.StatusNotFound {
		return false, err
	}
	values := url.Values{
		"privateExtendedProperty": {"trelloCardId=" + cardID},
		"showDeleted":             {"false"},
		"singleEvents":            {"true"},
		"maxResults":              {"2500"},
	}
	items, _, err := c.list(ctx, values)
	if err != nil {
		return false, err
	}
	for _, item := range items {
		if !strings.EqualFold(item.Status, "cancelled") && item.ExtendedProperties.Private["trelloCardId"] == cardID {
			return true, nil
		}
	}
	return false, nil
}

// PATCH: Return the live Calendar event linked to a Trello card so the Doing
// review can move it into next week's schedule instead of creating a duplicate.
func (c *Client) FindCardEvent(ctx context.Context, boardID, cardID string) (scheduling.Event, bool, error) {
	id := scheduling.DeterministicEventID(c.CalendarID, boardID, cardID)
	var item apiEvent
	err := c.doJSON(ctx, http.MethodGet, c.eventsURL()+"/"+url.PathEscape(id), nil, &item)
	if err == nil {
		event, normalizeErr := c.normalize(item)
		if normalizeErr != nil {
			return scheduling.Event{}, false, normalizeErr
		}
		if strings.EqualFold(event.Status, "cancelled") {
			return scheduling.Event{}, false, nil
		}
		return event, true, nil
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusNotFound {
		return scheduling.Event{}, false, err
	}
	values := url.Values{
		"privateExtendedProperty": {"trelloCardId=" + cardID},
		"showDeleted":             {"false"}, "singleEvents": {"true"}, "maxResults": {"2500"},
	}
	items, _, listErr := c.list(ctx, values)
	if listErr != nil {
		return scheduling.Event{}, false, listErr
	}
	for _, event := range items {
		if strings.EqualFold(event.Status, "cancelled") {
			continue
		}
		normalized, normalizeErr := c.normalize(event)
		if normalizeErr != nil {
			return scheduling.Event{}, false, normalizeErr
		}
		return normalized, true, nil
	}
	return scheduling.Event{}, false, nil
}

// PATCH: Update only the start/end fields of a linked Calendar event while
// retaining its deterministic ID and Trello metadata.
func (c *Client) RescheduleEvent(ctx context.Context, eventID string, start, end time.Time) error {
	body := map[string]apiEventTime{
		"start": {DateTime: start.Format(time.RFC3339), TimeZone: c.Timezone.String()},
		"end":   {DateTime: end.Format(time.RFC3339), TimeZone: c.Timezone.String()},
	}
	return c.doJSON(ctx, http.MethodPatch, c.eventsURL()+"/"+url.PathEscape(eventID), body, nil)
}

func (c *Client) CreateEvent(ctx context.Context, boardID string, assignment scheduling.Assignment, titlePrefix, description, colorID string) (string, bool, error) {
	eventID := scheduling.DeterministicEventID(c.CalendarID, boardID, assignment.Card.ID)
	event := apiEvent{
		ID:          eventID,
		Summary:     titlePrefix + assignment.Card.Name,
		Description: description,
		ColorID:     colorID,
		Start:       apiEventTime{DateTime: assignment.Start.Format(time.RFC3339), TimeZone: c.Timezone.String()},
		End:         apiEventTime{DateTime: assignment.End.Format(time.RFC3339), TimeZone: c.Timezone.String()},
	}
	event.ExtendedProperties.Private = map[string]string{
		"trelloCardId":  assignment.Card.ID,
		"trelloBoardId": boardID,
		"source":        scheduling.Source,
	}
	// PATCH: POST once, then trust only strict read-back of the deterministic ID.
	status, _, err := c.doJSONOnce(ctx, http.MethodPost, c.eventsURL(), event, nil)
	if status >= 200 && status < 300 {
		if confirmErr := c.confirmEvent(ctx, eventID, boardID, assignment.Card.ID); confirmErr != nil {
			return "", false, fmt.Errorf("verify created event: %w", confirmErr)
		}
		return eventID, true, nil
	}
	// PATCH(upstream): Handle 409 Conflict — deterministic ID exists as a
	// soft-deleted event. Resurrect it via PUT instead of failing.
	var apiErr *APIError
	if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusConflict {
		if resurrectErr := c.resurrectEvent(ctx, event); resurrectErr != nil {
			return "", false, resurrectErr
		}
		if confirmErr := c.confirmEvent(ctx, eventID, boardID, assignment.Card.ID); confirmErr != nil {
			return "", false, fmt.Errorf("verify resurrected event: %w", confirmErr)
		}
		return eventID, true, nil
	}
	ambiguous := err != nil && (!errors.As(err, &apiErr) || apiErr.StatusCode == http.StatusTooManyRequests || apiErr.StatusCode >= 500)
	if !ambiguous {
		return "", false, err
	}
	if confirmErr := c.confirmEvent(ctx, eventID, boardID, assignment.Card.ID); confirmErr != nil {
		return "", false, fmt.Errorf("create event outcome ambiguous (%v); verification failed: %w", err, confirmErr)
	}
	return eventID, false, nil
}

// PATCH: Resurrect a soft-deleted (cancelled) event by PUTting the full
// representation with status "confirmed". Google Calendar retains deleted
// event IDs for a period, causing POST to 409 even when the event is gone.
func (c *Client) resurrectEvent(ctx context.Context, event apiEvent) error {
	event.Status = "confirmed"
	target := c.eventsURL() + "/" + url.PathEscape(event.ID)
	return c.doJSON(ctx, http.MethodPut, target, event, nil)
}

func (c *Client) confirmEvent(ctx context.Context, eventID, boardID, cardID string) error {
	target := c.eventsURL() + "/" + url.PathEscape(eventID)
	for attempt := 0; attempt < 4; attempt++ {
		var event apiEvent
		status, retryAfter, err := c.doJSONOnce(ctx, http.MethodGet, target, nil, &event)
		if err == nil {
			properties := event.ExtendedProperties.Private
			active := strings.EqualFold(event.Status, "confirmed") || strings.EqualFold(event.Status, "tentative")
			if event.ID != eventID || !active || properties["trelloCardId"] != cardID || properties["trelloBoardId"] != boardID {
				return fmt.Errorf("event %q failed identity, active-status, or Trello metadata verification", eventID)
			}
			return nil
		}
		var apiErr *APIError
		transient := !errors.As(err, &apiErr) || status == http.StatusNotFound || status == http.StatusTooManyRequests || status >= 500
		if !transient || attempt == 3 {
			return err
		}
		if err := c.waitForConfirmation(ctx, backoff(attempt, retryAfter)); err != nil {
			return err
		}
	}
	return fmt.Errorf("event verification exhausted retries")
}

func (c *Client) waitForConfirmation(ctx context.Context, delay time.Duration) error {
	return c.confirmWait(ctx, delay)
}

func waitForContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (c *Client) CheckAccess(ctx context.Context) error {
	start := time.Now().In(c.Timezone)
	values := url.Values{"timeMin": {start.Format(time.RFC3339)}, "timeMax": {start.Add(time.Minute).Format(time.RFC3339)}, "maxResults": {"1"}}
	_, role, err := c.list(ctx, values)
	if err != nil {
		return err
	}
	if role != "owner" && role != "writer" {
		return fmt.Errorf("calendar access role %q cannot create events", role)
	}
	return nil
}

func (c *Client) normalize(item apiEvent) (scheduling.Event, error) {
	event := scheduling.Event{ID: item.ID, Summary: item.Summary, Status: item.Status, Properties: item.ExtendedProperties.Private}
	if item.Status == "cancelled" && item.Start.Date == "" && item.Start.DateTime == "" {
		return event, nil
	}
	var err error
	if item.Start.Date != "" {
		event.AllDay = true
		event.Start, err = time.ParseInLocation("2006-01-02", item.Start.Date, c.Timezone)
		if err == nil {
			event.End, err = time.ParseInLocation("2006-01-02", item.End.Date, c.Timezone)
		}
	} else {
		event.Start, err = time.Parse(time.RFC3339Nano, item.Start.DateTime)
		if err == nil {
			event.End, err = time.Parse(time.RFC3339Nano, item.End.DateTime)
		}
		event.Start = event.Start.In(c.Timezone)
		event.End = event.End.In(c.Timezone)
	}
	if err != nil || !event.End.After(event.Start) {
		return scheduling.Event{}, fmt.Errorf("malformed Google Calendar event %s", item.ID)
	}
	return event, nil
}

func (c *Client) eventsURL() string {
	return c.BaseURL + "/calendars/" + url.PathEscape(c.CalendarID) + "/events"
}

func (c *Client) doJSON(ctx context.Context, method, target string, body, out any) error {
	var encoded []byte
	var err error
	if body != nil {
		encoded, err = json.Marshal(body)
		if err != nil {
			return err
		}
	}
	for attempt := 0; attempt < 4; attempt++ {
		c.limiter.Wait()
		req, err := http.NewRequestWithContext(ctx, method, target, bytes.NewReader(encoded))
		if err != nil {
			return err
		}
		req.Header.Set("Accept", "application/json")
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		resp, err := c.HTTPClient.Do(req)
		if err != nil {
			if attempt < 3 {
				c.Sleep(backoff(attempt, ""))
				continue
			}
			return fmt.Errorf("google calendar request: %w", err)
		}
		data, readErr := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
		resp.Body.Close()
		if readErr != nil {
			return readErr
		}
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			c.limiter.OnSuccess()
			if out != nil && len(data) > 0 {
				if err := json.Unmarshal(data, out); err != nil {
					return fmt.Errorf("decode Google Calendar response: %w", err)
				}
			}
			return nil
		}
		apiErr := &APIError{StatusCode: resp.StatusCode, Body: truncate(data)}
		if attempt < 3 && (resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500) {
			if resp.StatusCode == http.StatusTooManyRequests {
				c.limiter.OnRateLimit()
			}
			c.Sleep(backoff(attempt, resp.Header.Get("Retry-After")))
			continue
		}
		return apiErr
	}
	return fmt.Errorf("google calendar request exhausted retries")
}

func (c *Client) doJSONOnce(ctx context.Context, method, target string, body, out any) (int, string, error) {
	var encoded []byte
	var err error
	if body != nil {
		encoded, err = json.Marshal(body)
		if err != nil {
			return 0, "", err
		}
	}
	c.limiter.Wait()
	req, err := http.NewRequestWithContext(ctx, method, target, bytes.NewReader(encoded))
	if err != nil {
		return 0, "", err
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return 0, "", fmt.Errorf("google calendar request: %w", err)
	}
	data, readErr := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	resp.Body.Close()
	if readErr != nil {
		return resp.StatusCode, resp.Header.Get("Retry-After"), readErr
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if resp.StatusCode == http.StatusTooManyRequests {
			c.limiter.OnRateLimit()
		}
		return resp.StatusCode, resp.Header.Get("Retry-After"), &APIError{StatusCode: resp.StatusCode, Body: truncate(data)}
	}
	c.limiter.OnSuccess()
	if out != nil && len(data) > 0 {
		if err := json.Unmarshal(data, out); err != nil {
			return resp.StatusCode, resp.Header.Get("Retry-After"), fmt.Errorf("decode Google Calendar response: %w", err)
		}
	}
	return resp.StatusCode, resp.Header.Get("Retry-After"), nil
}

func backoff(attempt int, retryAfter string) time.Duration {
	if seconds, err := strconv.Atoi(strings.TrimSpace(retryAfter)); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	base := time.Duration(1<<attempt) * time.Second
	return base + time.Duration(rand.IntN(250))*time.Millisecond
}

func truncate(data []byte) string {
	const limit = 1024
	if len(data) > limit {
		data = data[:limit]
	}
	return strings.TrimSpace(string(data))
}
