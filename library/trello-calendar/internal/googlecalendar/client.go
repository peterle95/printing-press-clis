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
	BaseURL    string
	CalendarID string
	Timezone   *time.Location
	HTTPClient *http.Client
	Sleep      func(time.Duration)
	limiter    *cliutil.AdaptiveLimiter
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

type colorsResponse struct { Event map[string]json.RawMessage `json:"event"` }

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
	return &Client{BaseURL: strings.TrimRight(baseURL, "/"), CalendarID: calendarID, Timezone: location, HTTPClient: httpClient, Sleep: time.Sleep, limiter: cliutil.NewAdaptiveLimiter(0)}
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
	if err := c.doJSON(ctx, http.MethodGet, c.BaseURL+"/colors", nil, &response); err != nil { return nil, err }
	colors := make(map[string]bool, len(response.Event))
	for id := range response.Event { colors[id] = true }
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
		return true, nil
	}
	var apiErr *APIError
	if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusGone {
		return true, nil
	} else if apiErr == nil || apiErr.StatusCode != http.StatusNotFound {
		return false, err
	}
	values := url.Values{
		"privateExtendedProperty": {"trelloCardId=" + cardID},
		"showDeleted":             {"true"},
		"singleEvents":            {"true"},
		"maxResults":              {"2500"},
	}
	items, _, err := c.list(ctx, values)
	return len(items) > 0, err
}

func (c *Client) CreateEvent(ctx context.Context, boardID string, assignment scheduling.Assignment, titlePrefix, description, colorID string) (string, bool, error) {
	event := apiEvent{
		ID:          scheduling.DeterministicEventID(c.CalendarID, boardID, assignment.Card.ID),
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
	var created apiEvent
	err := c.doJSON(ctx, http.MethodPost, c.eventsURL(), event, &created)
	if err == nil {
		return created.ID, true, nil
	}
	var apiErr *APIError
	if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusConflict {
		exists, checkErr := c.FindCard(ctx, boardID, assignment.Card.ID)
		if checkErr == nil && exists {
			return event.ID, false, nil
		}
	}
	return "", false, err
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
