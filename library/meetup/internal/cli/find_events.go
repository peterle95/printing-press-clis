// Copyright 2026 peter-moelzer. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"html"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const berlinLat = 52.52
const berlinLon = 13.405

const eventSearchQuery = `query($filter: EventSearchFilter!, $first: Int, $after: String, $sort: KeywordSort) {
  eventSearch(filter: $filter, first: $first, after: $after, sort: $sort) {
    totalCount
    pageInfo {
      endCursor
      hasNextPage
    }
    edges {
      cursor
      node {
        id
        title
        eventUrl
        dateTime
        endTime
        eventType
        status
        description
        group {
          id
          name
          urlname
          link
          city
          country
        }
        venue {
          id
          name
          address
          city
          country
          lat
          lon
        }
        rsvps {
          totalCount
        }
      }
    }
  }
}`

type findEventsOptions struct {
	query              string
	findURL            string
	city               string
	country            string
	lat                float64
	lon                float64
	radiusKM           float64
	limit              int
	pages              int
	pageSize           int
	withinDays         int
	eventType          string
	sortMode           string
	minRSVPs           int
	networkingOnly     bool
	includeDescription bool
}

type meetupGraphQLRequest struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables"`
}

type meetupGraphQLResponse struct {
	Data   *meetupData      `json:"data"`
	Errors []meetupGQLError `json:"errors,omitempty"`
}

type meetupGQLError struct {
	Message string `json:"message"`
}

type meetupData struct {
	EventSearch eventSearchConnection `json:"eventSearch"`
}

type eventSearchConnection struct {
	TotalCount int               `json:"totalCount"`
	PageInfo   eventPageInfo     `json:"pageInfo"`
	Edges      []eventSearchEdge `json:"edges"`
}

type eventPageInfo struct {
	EndCursor   string `json:"endCursor"`
	HasNextPage bool   `json:"hasNextPage"`
}

type eventSearchEdge struct {
	Cursor string      `json:"cursor"`
	Node   meetupEvent `json:"node"`
}

type meetupEvent struct {
	ID          string       `json:"id"`
	Title       string       `json:"title"`
	EventURL    string       `json:"eventUrl"`
	DateTime    string       `json:"dateTime"`
	EndTime     string       `json:"endTime"`
	EventType   string       `json:"eventType"`
	Status      string       `json:"status"`
	Description string       `json:"description"`
	Group       *meetupGroup `json:"group"`
	Venue       *meetupVenue `json:"venue"`
	RSVPs       meetupRSVPs  `json:"rsvps"`
}

type meetupGroup struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	URLName string `json:"urlname"`
	Link    string `json:"link"`
	City    string `json:"city"`
	Country string `json:"country"`
}

type meetupVenue struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Address string   `json:"address"`
	City    string   `json:"city"`
	Country string   `json:"country"`
	Lat     *float64 `json:"lat"`
	Lon     *float64 `json:"lon"`
}

type meetupRSVPs struct {
	TotalCount int `json:"totalCount"`
}

type findEventsResponse struct {
	Query   findEventsQuery     `json:"query"`
	Meta    findEventsMeta      `json:"meta"`
	Results []meetupEventResult `json:"results"`
}

type findEventsQuery struct {
	Keyword    string  `json:"keyword"`
	City       string  `json:"city"`
	Country    string  `json:"country"`
	Latitude   float64 `json:"latitude"`
	Longitude  float64 `json:"longitude"`
	RadiusKM   float64 `json:"radius_km"`
	EventType  string  `json:"event_type,omitempty"`
	WithinDays int     `json:"within_days,omitempty"`
	SourceURL  string  `json:"source_url,omitempty"`
}

type findEventsMeta struct {
	Source         string `json:"source"`
	TotalCount     int    `json:"total_count"`
	Fetched        int    `json:"fetched"`
	Returned       int    `json:"returned"`
	PagesFetched   int    `json:"pages_fetched"`
	NextCursor     string `json:"next_cursor,omitempty"`
	HasMore        bool   `json:"has_more"`
	Sort           string `json:"sort"`
	NetworkingOnly bool   `json:"networking_only"`
}

type meetupEventResult struct {
	ID                 string   `json:"id"`
	Title              string   `json:"title"`
	URL                string   `json:"url"`
	DateTime           string   `json:"date_time"`
	EndTime            string   `json:"end_time,omitempty"`
	EventType          string   `json:"event_type"`
	Status             string   `json:"status"`
	RSVPCount          int      `json:"rsvp_count"`
	Group              string   `json:"group"`
	GroupURL           string   `json:"group_url,omitempty"`
	Venue              string   `json:"venue,omitempty"`
	Address            string   `json:"address,omitempty"`
	City               string   `json:"city,omitempty"`
	Country            string   `json:"country,omitempty"`
	Latitude           *float64 `json:"latitude,omitempty"`
	Longitude          *float64 `json:"longitude,omitempty"`
	NetworkingScore    int      `json:"networking_score"`
	NetworkingSignals  []string `json:"networking_signals,omitempty"`
	DescriptionExcerpt string   `json:"description_excerpt,omitempty"`
	Description        string   `json:"description,omitempty"`
}

func newFindEventsCmd(flags *rootFlags) *cobra.Command {
	opts := findEventsOptions{
		query:      "developer",
		city:       "Berlin",
		country:    "de",
		lat:        berlinLat,
		lon:        berlinLon,
		radiusKM:   25,
		limit:      10,
		pages:      2,
		pageSize:   20,
		withinDays: 90,
		eventType:  "physical",
		sortMode:   "networking",
		minRSVPs:   5,
	}
	cmd := &cobra.Command{
		Use:   "find-events",
		Short: "Find Berlin software-development networking events",
		Long: `Find public Meetup events for software-development networking.

Defaults are tuned for in-person developer events in Berlin, Germany. The
command uses Meetup's public GraphQL eventSearch endpoint and does not require
authentication for the default public search.`,
		Example: `  meetup-pp-cli find-events
  meetup-pp-cli find-events --query developer --city Berlin --radius-km 25 --agent
  meetup-pp-cli find-events --url 'https://www.meetup.com/find/?keywords=developer&location=de--Berlin--Berlin&source=EVENTS' --limit 5 --json
  meetup-pp-cli find-events --query devrel --networking-only --select results.title,results.date_time,results.url --agent`,
		RunE: func(cmd *cobra.Command, args []string) error {
			applyFindURLDefaults(&opts)
			if strings.TrimSpace(opts.query) == "" {
				return usageErr(fmt.Errorf("--query must not be empty"))
			}
			if opts.limit < 1 {
				return usageErr(fmt.Errorf("--limit must be at least 1"))
			}
			if opts.pages < 1 {
				return usageErr(fmt.Errorf("--pages must be at least 1"))
			}
			if opts.pageSize < 1 {
				return usageErr(fmt.Errorf("--page-size must be at least 1"))
			}
			if opts.pageSize > 50 {
				return usageErr(fmt.Errorf("--page-size must be 50 or less"))
			}
			if opts.radiusKM <= 0 {
				return usageErr(fmt.Errorf("--radius-km must be greater than 0"))
			}
			if opts.withinDays < 0 {
				return usageErr(fmt.Errorf("--within-days must be 0 or greater"))
			}
			if _, err := normalizeEventType(opts.eventType); err != nil {
				return usageErr(err)
			}
			if _, err := normalizeSortMode(opts.sortMode); err != nil {
				return usageErr(err)
			}
			if flags.dryRun {
				return printFindEventsDryRun(cmd, flags, opts)
			}
			return runFindEvents(cmd, flags, opts)
		},
	}
	cmd.Flags().StringVar(&opts.query, "query", opts.query, "Keyword search text, for example developer, devrel, react, golang, ai")
	cmd.Flags().StringVar(&opts.findURL, "url", "", "Meetup find URL to use as a source for keyword and Berlin location defaults")
	cmd.Flags().StringVar(&opts.city, "city", opts.city, "City name")
	cmd.Flags().StringVar(&opts.country, "country", opts.country, "Country code")
	cmd.Flags().Float64Var(&opts.lat, "lat", opts.lat, "Latitude for search center")
	cmd.Flags().Float64Var(&opts.lon, "lon", opts.lon, "Longitude for search center")
	cmd.Flags().Float64Var(&opts.radiusKM, "radius-km", opts.radiusKM, "Search radius in kilometers")
	cmd.Flags().IntVar(&opts.limit, "limit", opts.limit, "Maximum events to return after filtering and ranking")
	cmd.Flags().IntVar(&opts.pages, "pages", opts.pages, "Maximum result pages to fetch from Meetup")
	cmd.Flags().IntVar(&opts.pageSize, "page-size", opts.pageSize, "Meetup page size per request, max 50")
	cmd.Flags().IntVar(&opts.withinDays, "within-days", opts.withinDays, "Only include events starting within this many days; 0 disables the upper bound")
	cmd.Flags().StringVar(&opts.eventType, "event-type", opts.eventType, "Event type: physical, online, hybrid, or all")
	cmd.Flags().StringVar(&opts.sortMode, "sort", opts.sortMode, "Sort mode: networking, date, or relevance")
	cmd.Flags().IntVar(&opts.minRSVPs, "min-rsvps", opts.minRSVPs, "Only include events with at least this many RSVPs")
	cmd.Flags().BoolVar(&opts.networkingOnly, "networking-only", false, "Only include events with networking or community signals")
	cmd.Flags().BoolVar(&opts.includeDescription, "include-description", false, "Include full event descriptions in JSON output")
	return cmd
}

func runFindEvents(cmd *cobra.Command, flags *rootFlags, opts findEventsOptions) error {
	c, err := flags.newClient()
	if err != nil {
		return err
	}
	events, meta, err := fetchMeetupEvents(c.Post, opts)
	if err != nil {
		return classifyAPIError(err, flags)
	}
	results := make([]meetupEventResult, 0, len(events))
	for _, event := range events {
		result := summarizeMeetupEvent(event, opts)
		if opts.minRSVPs > 0 && result.RSVPCount < opts.minRSVPs {
			continue
		}
		if opts.networkingOnly && result.NetworkingScore == 0 {
			continue
		}
		if !opts.includeDescription {
			result.Description = ""
		}
		results = append(results, result)
	}
	sortResults(results, opts.sortMode)
	if len(results) > opts.limit {
		results = results[:opts.limit]
	}
	response := findEventsResponse{
		Query: findEventsQuery{
			Keyword:    opts.query,
			City:       opts.city,
			Country:    strings.ToLower(opts.country),
			Latitude:   opts.lat,
			Longitude:  opts.lon,
			RadiusKM:   opts.radiusKM,
			WithinDays: opts.withinDays,
			SourceURL:  opts.findURL,
		},
		Meta: findEventsMeta{
			Source:         "meetup_graphql_event_search",
			TotalCount:     meta.totalCount,
			Fetched:        len(events),
			Returned:       len(results),
			PagesFetched:   meta.pagesFetched,
			NextCursor:     meta.nextCursor,
			HasMore:        meta.hasMore,
			Sort:           opts.sortMode,
			NetworkingOnly: opts.networkingOnly,
		},
		Results: results,
	}
	if eventType, _ := normalizeEventType(opts.eventType); eventType != "" {
		response.Query.EventType = eventType
	}
	if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
		return flags.printJSON(cmd, response)
	}
	if len(results) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No matching Meetup events found.")
		return nil
	}
	rows := make([][]string, 0, len(results))
	for _, result := range results {
		rows = append(rows, []string{
			formatMeetupDate(result.DateTime),
			result.Title,
			strconv.Itoa(result.RSVPCount),
			firstNonEmptyString(result.Venue, result.City),
			result.Group,
			result.URL,
		})
	}
	return flags.printTable(cmd, []string{"Date", "Title", "RSVPs", "Venue", "Group", "URL"}, rows)
}

type cPostFunc func(path string, body any) (json.RawMessage, int, error)

func fetchMeetupEvents(post cPostFunc, opts findEventsOptions) ([]meetupEvent, fetchEventsMeta, error) {
	filter, err := buildEventSearchFilter(opts)
	if err != nil {
		return nil, fetchEventsMeta{}, err
	}
	sortArg, err := buildEventSearchSort(opts.sortMode)
	if err != nil {
		return nil, fetchEventsMeta{}, err
	}
	events := make([]meetupEvent, 0, opts.limit)
	seen := map[string]bool{}
	var meta fetchEventsMeta
	var after string
	for page := 0; page < opts.pages; page++ {
		variables := map[string]any{
			"filter": filter,
			"first":  opts.pageSize,
			"after":  nil,
			"sort":   sortArg,
		}
		if after != "" {
			variables["after"] = after
		}
		req := meetupGraphQLRequest{Query: eventSearchQuery, Variables: variables}
		raw, _, err := post("/gql-ext", req)
		if err != nil {
			return nil, meta, err
		}
		var gql meetupGraphQLResponse
		if err := json.Unmarshal(raw, &gql); err != nil {
			return nil, meta, fmt.Errorf("parsing Meetup GraphQL response: %w", err)
		}
		if len(gql.Errors) > 0 {
			return nil, meta, fmt.Errorf("meetup GraphQL error: %s", gql.Errors[0].Message)
		}
		if gql.Data == nil {
			return nil, meta, fmt.Errorf("meetup GraphQL response had no data")
		}
		conn := gql.Data.EventSearch
		meta.totalCount = conn.TotalCount
		meta.pagesFetched++
		meta.nextCursor = conn.PageInfo.EndCursor
		meta.hasMore = conn.PageInfo.HasNextPage
		for _, edge := range conn.Edges {
			if edge.Node.ID == "" || seen[edge.Node.ID] {
				continue
			}
			seen[edge.Node.ID] = true
			events = append(events, edge.Node)
		}
		if !conn.PageInfo.HasNextPage || conn.PageInfo.EndCursor == "" || conn.PageInfo.EndCursor == after {
			break
		}
		after = conn.PageInfo.EndCursor
	}
	return events, meta, nil
}

type fetchEventsMeta struct {
	totalCount   int
	pagesFetched int
	nextCursor   string
	hasMore      bool
}

func buildEventSearchFilter(opts findEventsOptions) (map[string]any, error) {
	eventType, err := normalizeEventType(opts.eventType)
	if err != nil {
		return nil, err
	}
	filter := map[string]any{
		"query":   opts.query,
		"lat":     opts.lat,
		"lon":     opts.lon,
		"radius":  opts.radiusKM,
		"city":    opts.city,
		"country": strings.ToLower(opts.country),
	}
	if eventType != "" {
		filter["eventType"] = eventType
	}
	if opts.withinDays > 0 {
		filter["startDateRange"] = time.Now().Format(time.RFC3339)
		filter["endDateRange"] = time.Now().AddDate(0, 0, opts.withinDays).Format(time.RFC3339)
	}
	return filter, nil
}

func buildEventSearchSort(sortMode string) (map[string]string, error) {
	mode, err := normalizeSortMode(sortMode)
	if err != nil {
		return nil, err
	}
	switch mode {
	case "date":
		return map[string]string{"sortField": "DATETIME", "sortOrder": "ASC"}, nil
	case "relevance", "networking":
		return map[string]string{"sortField": "RELEVANCE", "sortOrder": "DESC"}, nil
	default:
		return nil, nil
	}
}

func printFindEventsDryRun(cmd *cobra.Command, flags *rootFlags, opts findEventsOptions) error {
	filter, err := buildEventSearchFilter(opts)
	if err != nil {
		return usageErr(err)
	}
	sortArg, err := buildEventSearchSort(opts.sortMode)
	if err != nil {
		return usageErr(err)
	}
	payload := map[string]any{
		"method": "POST",
		"url":    "https://api.meetup.com/gql-ext",
		"body": meetupGraphQLRequest{
			Query: eventSearchQuery,
			Variables: map[string]any{
				"filter": filter,
				"first":  opts.pageSize,
				"sort":   sortArg,
			},
		},
	}
	return flags.printJSON(cmd, payload)
}

func summarizeMeetupEvent(event meetupEvent, opts findEventsOptions) meetupEventResult {
	text := strings.Join([]string{event.Title, event.Description, groupName(event.Group)}, " ")
	score, signals := networkingScore(text)
	result := meetupEventResult{
		ID:                 event.ID,
		Title:              event.Title,
		URL:                event.EventURL,
		DateTime:           event.DateTime,
		EndTime:            event.EndTime,
		EventType:          event.EventType,
		Status:             event.Status,
		RSVPCount:          event.RSVPs.TotalCount,
		Group:              groupName(event.Group),
		GroupURL:           groupURL(event.Group),
		NetworkingScore:    score,
		NetworkingSignals:  signals,
		DescriptionExcerpt: excerpt(cleanEventText(event.Description), 240),
		Description:        cleanEventText(event.Description),
	}
	if event.Venue != nil {
		result.Venue = event.Venue.Name
		result.Address = strings.TrimSpace(strings.Join(nonEmptyStrings([]string{event.Venue.Address, event.Venue.City}), ", "))
		result.City = event.Venue.City
		result.Country = strings.ToLower(event.Venue.Country)
		result.Latitude = event.Venue.Lat
		result.Longitude = event.Venue.Lon
	}
	if result.City == "" && event.Group != nil {
		result.City = event.Group.City
	}
	if result.Country == "" && event.Group != nil {
		result.Country = strings.ToLower(event.Group.Country)
	}
	if result.NetworkingScore == 0 && strings.EqualFold(opts.query, "developer") {
		result.NetworkingScore = 1
		result.NetworkingSignals = append(result.NetworkingSignals, "developer keyword")
	}
	return result
}

func networkingScore(text string) (int, []string) {
	lower := strings.ToLower(text)
	terms := []struct {
		term   string
		signal string
		weight int
	}{
		{"networking", "networking", 3},
		{"network ", "network", 2},
		{"connect", "connect", 2},
		{"community", "community", 2},
		{"meetup", "meetup", 1},
		{"social", "social", 1},
		{"drinks", "drinks", 1},
		{"pizza", "pizza", 1},
		{"developer", "developer", 2},
		{"software", "software", 2},
		{"engineering", "engineering", 2},
		{"devrel", "devrel", 2},
		{"react", "react", 1},
		{"frontend", "frontend", 1},
		{"mobile", "mobile", 1},
		{"kotlin", "kotlin", 1},
		{"flutter", "flutter", 1},
		{"golang", "golang", 1},
		{"python", "python", 1},
		{"c++", "c++", 1},
		{"open source", "open source", 1},
		{"ai", "ai", 1},
		{"cloud", "cloud", 1},
	}
	score := 0
	signals := []string{}
	seen := map[string]bool{}
	for _, term := range terms {
		if strings.Contains(lower, term.term) {
			score += term.weight
			if !seen[term.signal] {
				signals = append(signals, term.signal)
				seen[term.signal] = true
			}
		}
	}
	return score, signals
}

func sortResults(results []meetupEventResult, sortMode string) {
	mode, _ := normalizeSortMode(sortMode)
	sort.SliceStable(results, func(i, j int) bool {
		switch mode {
		case "date":
			return parseEventTime(results[i].DateTime).Before(parseEventTime(results[j].DateTime))
		case "relevance":
			if results[i].RSVPCount == results[j].RSVPCount {
				return parseEventTime(results[i].DateTime).Before(parseEventTime(results[j].DateTime))
			}
			return results[i].RSVPCount > results[j].RSVPCount
		default:
			if results[i].NetworkingScore == results[j].NetworkingScore {
				return parseEventTime(results[i].DateTime).Before(parseEventTime(results[j].DateTime))
			}
			return results[i].NetworkingScore > results[j].NetworkingScore
		}
	})
}

func parseEventTime(value string) time.Time {
	t, err := time.Parse(time.RFC3339, value)
	if err == nil {
		return t
	}
	return time.Time{}
}

func formatMeetupDate(value string) string {
	t := parseEventTime(value)
	if t.IsZero() {
		return value
	}
	return t.Format("2006-01-02 15:04")
}

func normalizeEventType(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "all", "any":
		return "", nil
	case "physical", "in-person", "in_person", "offline":
		return "PHYSICAL", nil
	case "online", "virtual":
		return "ONLINE", nil
	case "hybrid":
		return "HYBRID", nil
	default:
		return "", fmt.Errorf("--event-type must be physical, online, hybrid, or all")
	}
}

func normalizeSortMode(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "networking":
		return "networking", nil
	case "date", "time", "soonest":
		return "date", nil
	case "relevance", "rsvp":
		return "relevance", nil
	default:
		return "", fmt.Errorf("--sort must be networking, date, or relevance")
	}
}

func applyFindURLDefaults(opts *findEventsOptions) {
	if strings.TrimSpace(opts.findURL) == "" {
		return
	}
	u, err := url.Parse(opts.findURL)
	if err != nil {
		return
	}
	q := u.Query()
	if keywords := strings.TrimSpace(q.Get("keywords")); keywords != "" {
		opts.query = keywords
	}
	location := q.Get("location")
	if strings.HasPrefix(location, "de--Berlin") {
		opts.country = "de"
		opts.city = "Berlin"
		if opts.lat == 0 {
			opts.lat = berlinLat
		}
		if opts.lon == 0 {
			opts.lon = berlinLon
		}
	}
}

func cleanEventText(value string) string {
	value = html.UnescapeString(value)
	replacements := []string{"\r", " ", "\n", " ", "\t", " ", "**", "", "__", "", "\\-", "-", "[", "", "]", ""}
	r := strings.NewReplacer(replacements...)
	value = r.Replace(value)
	for strings.Contains(value, "  ") {
		value = strings.ReplaceAll(value, "  ", " ")
	}
	return strings.TrimSpace(value)
}

func excerpt(value string, max int) string {
	runes := []rune(value)
	if len(runes) <= max {
		return value
	}
	if max < 4 {
		return string(runes[:max])
	}
	return strings.TrimSpace(string(runes[:max-3])) + "..."
}

func groupName(group *meetupGroup) string {
	if group == nil {
		return ""
	}
	return group.Name
}

func groupURL(group *meetupGroup) string {
	if group == nil {
		return ""
	}
	return group.Link
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func nonEmptyStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			out = append(out, strings.TrimSpace(value))
		}
	}
	return out
}
