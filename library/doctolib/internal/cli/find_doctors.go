// Copyright 2026 peter-moelzer. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"math"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"doctolib-pp-cli/internal/client"
	"github.com/spf13/cobra"
)

type findDoctorsOptions struct {
	reason            string
	location          string
	searchURL         string
	visitReason       string
	insuranceSector   string
	withinDays        int
	limit             int
	pages             int
	lat               float64
	lng               float64
	radiusKM          float64
	hasCoordinates    bool
	includeNoSlots    bool
	includeTelehealth bool
}

type searchContext struct {
	Keyword struct {
		ID   int    `json:"id"`
		Slug string `json:"slug"`
		Name string `json:"name"`
	} `json:"keyword"`
	Place json.RawMessage `json:"place"`
	CSRF  string          `json:"csrf"`
	URL   string          `json:"url"`
}

type phsResponse struct {
	Total               int                  `json:"total"`
	HealthcareProviders []healthcareProvider `json:"healthcareProviders"`
}

type healthcareProvider struct {
	ID        string `json:"id"`
	Link      string `json:"link"`
	Name      string `json:"name"`
	FirstName string `json:"firstName"`
	Title     string `json:"title"`
	Type      string `json:"type"`
	Location  struct {
		Address          string   `json:"address"`
		City             string   `json:"city"`
		Zipcode          string   `json:"zipcode"`
		Lat              *float64 `json:"lat"`
		Lng              *float64 `json:"lng"`
		DistanceInMeters *float64 `json:"distanceInMeters"`
	} `json:"location"`
	OnlineBooking *struct {
		AgendaIDs  []int `json:"agendaIds"`
		Telehealth bool  `json:"telehealth"`
	} `json:"onlineBooking"`
	OrganizationStatus *struct {
		Name string `json:"name"`
		Slug string `json:"slug"`
	} `json:"organizationStatus"`
	References struct {
		ID         int    `json:"id"`
		PracticeID int    `json:"practiceId"`
		Type       string `json:"type"`
	} `json:"references"`
	RegulationSector string `json:"regulationSector"`
	Speciality       *struct {
		Name string `json:"name"`
		Slug string `json:"slug"`
	} `json:"speciality"`
	MatchedVisitMotive *struct {
		VisitMotiveID int    `json:"visitMotiveId"`
		Name          string `json:"name"`
		AgendaIDs     []int  `json:"agendaIds"`
		Insurance     *struct {
			Type string `json:"type"`
		} `json:"insuranceSector"`
		AllowNewPatients bool `json:"allowNewPatients"`
	} `json:"matchedVisitMotive"`
}

type availabilityResponse struct {
	Availabilities []availabilityDay `json:"availabilities"`
	Total          int               `json:"total"`
	NextSlot       any               `json:"next_slot"`
	Reason         any               `json:"reason"`
}

type availabilityDay struct {
	Date  string            `json:"date"`
	Slots []json.RawMessage `json:"slots"`
}

type doctorResult struct {
	Name             string   `json:"name"`
	Type             string   `json:"type,omitempty"`
	Specialty        string   `json:"specialty,omitempty"`
	VisitReason      string   `json:"visit_reason,omitempty"`
	ProfileURL       string   `json:"profile_url"`
	Address          string   `json:"address,omitempty"`
	City             string   `json:"city,omitempty"`
	Zipcode          string   `json:"zipcode,omitempty"`
	DistanceKM       *float64 `json:"distance_km,omitempty"`
	Telehealth       bool     `json:"telehealth"`
	InsuranceSector  string   `json:"insurance_sector,omitempty"`
	AllowNewPatients bool     `json:"allow_new_patients"`
	FirstAvailable   string   `json:"first_available,omitempty"`
	Slots            []string `json:"slots,omitempty"`
	TotalSlots       int      `json:"total_slots"`
	VisitMotiveID    int      `json:"visit_motive_id,omitempty"`
	AgendaIDs        []int    `json:"agenda_ids,omitempty"`
	PracticeID       int      `json:"practice_id,omitempty"`
}

type findDoctorsResponse struct {
	Query struct {
		Reason          string  `json:"reason"`
		ResolvedReason  string  `json:"resolved_reason"`
		Location        string  `json:"location"`
		WithinDays      int     `json:"within_days"`
		VisitReason     string  `json:"visit_reason,omitempty"`
		InsuranceSector string  `json:"insurance_sector,omitempty"`
		Latitude        float64 `json:"latitude,omitempty"`
		Longitude       float64 `json:"longitude,omitempty"`
		RadiusKM        float64 `json:"radius_km,omitempty"`
		SearchURL       string  `json:"search_url"`
	} `json:"query"`
	TotalAvailable int            `json:"total_available"`
	Results        []doctorResult `json:"results"`
}

func newFindDoctorsCmd(flags *rootFlags) *cobra.Command {
	opts := findDoctorsOptions{
		withinDays: 14,
		limit:      10,
		pages:      1,
	}
	cmd := &cobra.Command{
		Use:   "find-doctors",
		Short: "Find Doctolib doctors with real appointment slots",
		Long: `Find Doctolib doctors near a location for a specialty or reason.

The command uses Doctolib's public search page to establish the same browser
session context as the website, then checks each matching provider's slot
availability endpoint. Booking is intentionally not implemented yet.`,
		Example: `  doctolib-pp-cli find-doctors --reason allgemeinmedizin --location berlin
  doctolib-pp-cli find-doctors --reason hausarzt --location berlin --visit-reason akut --within-days 7
  doctolib-pp-cli find-doctors --url https://www.doctolib.de/allgemeinmedizin/berlin --lat 52.52 --lng 13.40 --radius-km 5 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.dryRun {
				return printFindDoctorsDryRun(cmd, flags, opts)
			}
			if opts.searchURL == "" {
				if strings.TrimSpace(opts.reason) == "" {
					return usageErr(fmt.Errorf("required flag %q not set", "reason"))
				}
				if strings.TrimSpace(opts.location) == "" {
					return usageErr(fmt.Errorf("required flag %q not set", "location"))
				}
			}
			if opts.withinDays < 1 {
				return usageErr(fmt.Errorf("--within-days must be at least 1"))
			}
			if opts.limit < 1 {
				return usageErr(fmt.Errorf("--limit must be at least 1"))
			}
			if opts.pages < 1 {
				return usageErr(fmt.Errorf("--pages must be at least 1"))
			}
			if cmd.Flags().Changed("lat") != cmd.Flags().Changed("lng") {
				return usageErr(fmt.Errorf("--lat and --lng must be provided together"))
			}
			opts.hasCoordinates = cmd.Flags().Changed("lat") && cmd.Flags().Changed("lng")
			if opts.radiusKM > 0 && !opts.hasCoordinates {
				return usageErr(fmt.Errorf("--radius-km requires --lat and --lng"))
			}
			return runFindDoctors(cmd, flags, opts)
		},
	}
	cmd.Flags().StringVar(&opts.reason, "reason", "", "Specialty/reason slug or common name, for example allgemeinmedizin or hausarzt")
	cmd.Flags().StringVar(&opts.location, "location", "", "Doctolib location slug or city, for example berlin")
	cmd.Flags().StringVar(&opts.searchURL, "url", "", "Full Doctolib search URL to use instead of --reason/--location")
	cmd.Flags().StringVar(&opts.visitReason, "visit-reason", "", "Only keep visit motives containing this text")
	cmd.Flags().StringVar(&opts.insuranceSector, "insurance-sector", "", "Filter by insurance sector: public or private")
	cmd.Flags().IntVar(&opts.withinDays, "within-days", opts.withinDays, "Only include providers with slots within this many days")
	cmd.Flags().IntVar(&opts.limit, "limit", opts.limit, "Maximum available providers to return")
	cmd.Flags().IntVar(&opts.pages, "pages", opts.pages, "Maximum Doctolib result pages to inspect")
	cmd.Flags().Float64Var(&opts.lat, "lat", 0, "Latitude for distance filtering")
	cmd.Flags().Float64Var(&opts.lng, "lng", 0, "Longitude for distance filtering")
	cmd.Flags().Float64Var(&opts.radiusKM, "radius-km", 0, "Maximum distance from --lat/--lng in kilometers")
	cmd.Flags().BoolVar(&opts.includeNoSlots, "include-no-slots", false, "Include providers even when no slots are returned")
	cmd.Flags().BoolVar(&opts.includeTelehealth, "include-telehealth", true, "Include telehealth providers")
	return cmd
}

func runFindDoctors(cmd *cobra.Command, flags *rootFlags, opts findDoctorsOptions) error {
	c, err := flags.newClient()
	if err != nil {
		return err
	}
	jar, err := cookiejar.New(nil)
	if err != nil {
		return err
	}
	httpClient := &http.Client{Timeout: flags.timeout, Jar: jar}
	searchURL, reasonSlug, locationSlug, err := resolveSearchURL(c.BaseURL, opts)
	if err != nil {
		return usageErr(err)
	}
	ctx, err := fetchSearchContext(httpClient, c, searchURL)
	if err != nil {
		return classifyAPIError(err, flags)
	}
	results := make([]doctorResult, 0, opts.limit)
	seen := map[string]bool{}
	for page := 0; page < opts.pages && len(results) < opts.limit; page++ {
		resp, err := fetchProvidersPage(httpClient, c, ctx, opts, page)
		if err != nil {
			return classifyAPIError(err, flags)
		}
		if len(resp.HealthcareProviders) == 0 {
			break
		}
		for _, provider := range resp.HealthcareProviders {
			if len(results) >= opts.limit {
				break
			}
			if seen[provider.ID] {
				continue
			}
			seen[provider.ID] = true
			if !providerMatches(provider, opts) {
				continue
			}
			result := summarizeProvider(c.BaseURL, provider, opts)
			if opts.radiusKM > 0 && (result.DistanceKM == nil || *result.DistanceKM > opts.radiusKM) {
				continue
			}
			avail, err := fetchProviderAvailability(httpClient, c, provider, opts)
			if err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: skipping %s availability: %v\n", result.Name, err)
				continue
			}
			applyAvailability(&result, avail)
			if !opts.includeNoSlots && result.TotalSlots == 0 {
				continue
			}
			results = append(results, result)
			sleepForRate(flags.rateLimit)
		}
	}
	var out findDoctorsResponse
	out.Query.Reason = opts.reason
	out.Query.ResolvedReason = firstNonEmpty(ctx.Keyword.Slug, reasonSlug)
	out.Query.Location = firstNonEmpty(locationSlug, opts.location)
	out.Query.WithinDays = opts.withinDays
	out.Query.VisitReason = opts.visitReason
	out.Query.InsuranceSector = opts.insuranceSector
	out.Query.SearchURL = ctx.URL
	if opts.hasCoordinates {
		out.Query.Latitude = opts.lat
		out.Query.Longitude = opts.lng
		out.Query.RadiusKM = opts.radiusKM
	}
	out.TotalAvailable = len(results)
	out.Results = results
	if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
		return flags.printJSON(cmd, out)
	}
	rows := make([][]string, 0, len(results))
	for _, r := range results {
		location := strings.TrimSpace(strings.Join(nonEmpty([]string{r.Address, r.Zipcode, r.City}), ", "))
		if r.DistanceKM != nil {
			location = fmt.Sprintf("%s (%.1f km)", location, *r.DistanceKM)
		}
		rows = append(rows, []string{
			r.Name,
			r.Specialty,
			r.VisitReason,
			r.FirstAvailable,
			location,
			r.ProfileURL,
		})
	}
	if len(rows) == 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "No matching available appointments found within %d days.\n", opts.withinDays)
		return nil
	}
	return flags.printTable(cmd, []string{"Name", "Specialty", "Visit reason", "First slot", "Location", "URL"}, rows)
}

func printFindDoctorsDryRun(cmd *cobra.Command, flags *rootFlags, opts findDoctorsOptions) error {
	c, err := flags.newClient()
	if err != nil {
		return err
	}
	searchURL, reasonSlug, locationSlug, err := resolveSearchURL(c.BaseURL, opts)
	if err != nil && opts.searchURL == "" {
		return usageErr(err)
	}
	payload := map[string]any{
		"search_url":       searchURL,
		"reason_slug":      reasonSlug,
		"location_slug":    locationSlug,
		"within_days":      opts.withinDays,
		"provider_pages":   opts.pages,
		"provider_limit":   opts.limit,
		"availability_url": strings.TrimRight(c.BaseURL, "/") + "/search/availabilities.json",
	}
	return flags.printJSON(cmd, payload)
}

func resolveSearchURL(baseURL string, opts findDoctorsOptions) (string, string, string, error) {
	if opts.searchURL != "" {
		parsed, err := url.Parse(opts.searchURL)
		if err != nil {
			return "", "", "", err
		}
		parts := splitPath(parsed.Path)
		if len(parts) < 2 {
			return opts.searchURL, "", "", nil
		}
		return opts.searchURL, parts[0], parts[1], nil
	}
	reason := normalizeReasonSlug(opts.reason)
	location := slugify(opts.location)
	if reason == "" || location == "" {
		return "", "", "", fmt.Errorf("could not resolve --reason and --location into Doctolib slugs")
	}
	u, err := url.Parse(strings.TrimRight(baseURL, "/"))
	if err != nil {
		return "", "", "", err
	}
	u.Path = "/" + url.PathEscape(reason) + "/" + url.PathEscape(location)
	return u.String(), reason, location, nil
}

func fetchSearchContext(httpClient *http.Client, c *client.Client, searchURL string) (searchContext, error) {
	req, err := http.NewRequest(http.MethodGet, searchURL, nil)
	if err != nil {
		return searchContext{}, err
	}
	applyBrowserHeaders(req, c, "")
	resp, body, err := doRequest(httpClient, req)
	if err != nil {
		return searchContext{}, err
	}
	if resp.StatusCode >= 400 {
		return searchContext{}, &client.APIError{Method: http.MethodGet, Path: req.URL.Path, StatusCode: resp.StatusCode, Body: string(body)}
	}
	text := string(body)
	keywordJSON, ok := extractBetween(text, "window.keyword = ", ";window.place")
	if !ok {
		return searchContext{}, fmt.Errorf("could not find Doctolib keyword data in search page")
	}
	placeJSON, ok := extractBetween(text, "window.place = ", ";window.searchType")
	if !ok {
		return searchContext{}, fmt.Errorf("could not find Doctolib place data in search page")
	}
	var ctx searchContext
	if err := json.Unmarshal([]byte(keywordJSON), &ctx.Keyword); err != nil {
		return searchContext{}, fmt.Errorf("parsing keyword data: %w", err)
	}
	if !json.Valid([]byte(placeJSON)) {
		return searchContext{}, fmt.Errorf("parsing place data: invalid JSON")
	}
	ctx.Place = json.RawMessage(placeJSON)
	ctx.CSRF = extractCSRF(text)
	ctx.URL = searchURL
	return ctx, nil
}

func fetchProvidersPage(httpClient *http.Client, c *client.Client, ctx searchContext, opts findDoctorsOptions, page int) (phsResponse, error) {
	base := strings.TrimRight(c.BaseURL, "/") + "/phs_proxy/raw"
	u, err := url.Parse(base)
	if err != nil {
		return phsResponse{}, err
	}
	q := u.Query()
	q.Set("page", strconv.Itoa(page))
	u.RawQuery = q.Encode()
	body := map[string]any{
		"keyword": ctx.Keyword.Slug,
		"location": map[string]any{
			"place": json.RawMessage(ctx.Place),
		},
	}
	filters := map[string]any{}
	if opts.withinDays > 0 {
		filters["availabilitiesBefore"] = availabilityCutoff(opts.withinDays)
	}
	if opts.insuranceSector != "" && opts.insuranceSector != "all" {
		filters["insuranceSector"] = strings.ToLower(opts.insuranceSector)
	}
	if !opts.includeTelehealth {
		filters["telehealth"] = false
	}
	body["filters"] = filters
	payload, err := json.Marshal(body)
	if err != nil {
		return phsResponse{}, err
	}
	req, err := http.NewRequest(http.MethodPost, u.String(), bytes.NewReader(payload))
	if err != nil {
		return phsResponse{}, err
	}
	applyBrowserHeaders(req, c, ctx.URL)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if ctx.CSRF != "" {
		req.Header.Set("X-CSRF-Token", ctx.CSRF)
	}
	resp, data, err := doRequest(httpClient, req)
	if err != nil {
		return phsResponse{}, err
	}
	if resp.StatusCode >= 400 {
		return phsResponse{}, &client.APIError{Method: http.MethodPost, Path: req.URL.Path, StatusCode: resp.StatusCode, Body: string(data)}
	}
	var out phsResponse
	if err := json.Unmarshal(data, &out); err != nil {
		return phsResponse{}, fmt.Errorf("parsing provider search response: %w", err)
	}
	return out, nil
}

func fetchProviderAvailability(httpClient *http.Client, c *client.Client, provider healthcareProvider, opts findDoctorsOptions) (availabilityResponse, error) {
	var merged availabilityResponse
	remaining := opts.withinDays
	offset := 0
	for remaining > 0 {
		windowDays := remaining
		if windowDays > 7 {
			windowDays = 7
		}
		part, err := fetchProviderAvailabilityWindow(httpClient, c, provider, opts, offset, windowDays)
		if err != nil {
			return availabilityResponse{}, err
		}
		merged.Total += part.Total
		merged.Availabilities = append(merged.Availabilities, part.Availabilities...)
		if merged.NextSlot == nil {
			merged.NextSlot = part.NextSlot
		}
		offset += windowDays
		remaining -= windowDays
		if merged.Total > 0 {
			break
		}
	}
	return merged, nil
}

func fetchProviderAvailabilityWindow(httpClient *http.Client, c *client.Client, provider healthcareProvider, opts findDoctorsOptions, offsetDays int, windowDays int) (availabilityResponse, error) {
	if provider.MatchedVisitMotive == nil {
		return availabilityResponse{}, fmt.Errorf("provider has no matched visit motive")
	}
	agendaIDs := provider.MatchedVisitMotive.AgendaIDs
	if len(agendaIDs) == 0 && provider.OnlineBooking != nil {
		agendaIDs = provider.OnlineBooking.AgendaIDs
	}
	if len(agendaIDs) == 0 {
		return availabilityResponse{}, fmt.Errorf("provider has no agenda ids")
	}
	u, err := url.Parse(strings.TrimRight(c.BaseURL, "/") + "/search/availabilities.json")
	if err != nil {
		return availabilityResponse{}, err
	}
	q := u.Query()
	q.Set("telehealth", strconv.FormatBool(provider.OnlineBooking != nil && provider.OnlineBooking.Telehealth))
	q.Set("limit", strconv.Itoa(windowDays))
	q.Set("start_date_time", time.Now().AddDate(0, 0, offsetDays).Format("2006-01-02T15:04:05.000-07:00"))
	q.Set("visit_motive_id", strconv.Itoa(provider.MatchedVisitMotive.VisitMotiveID))
	q.Set("agenda_ids", joinInts(agendaIDs, "-"))
	if provider.References.PracticeID != 0 {
		q.Set("practice_ids", strconv.Itoa(provider.References.PracticeID))
	}
	sector := strings.ToLower(opts.insuranceSector)
	if sector == "" && provider.MatchedVisitMotive.Insurance != nil {
		sector = strings.ToLower(provider.MatchedVisitMotive.Insurance.Type)
	}
	if sector != "" && sector != "all" {
		q.Set("insurance_sector", sector)
	}
	u.RawQuery = q.Encode()
	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return availabilityResponse{}, err
	}
	applyBrowserHeaders(req, c, "")
	req.Header.Set("Accept", "application/json")
	resp, data, err := doRequest(httpClient, req)
	if err != nil {
		return availabilityResponse{}, err
	}
	if resp.StatusCode >= 400 {
		return availabilityResponse{}, &client.APIError{Method: http.MethodGet, Path: req.URL.Path, StatusCode: resp.StatusCode, Body: string(data)}
	}
	var out availabilityResponse
	if err := json.Unmarshal(data, &out); err != nil {
		return availabilityResponse{}, fmt.Errorf("parsing availability response: %w", err)
	}
	return out, nil
}

func summarizeProvider(baseURL string, p healthcareProvider, opts findDoctorsOptions) doctorResult {
	name := strings.Join(nonEmpty([]string{p.Title, p.FirstName, p.Name}), " ")
	if strings.TrimSpace(name) == "" {
		name = p.Name
	}
	result := doctorResult{
		Name:       name,
		Type:       p.Type,
		ProfileURL: absoluteURL(baseURL, p.Link),
		Address:    p.Location.Address,
		City:       p.Location.City,
		Zipcode:    p.Location.Zipcode,
		PracticeID: p.References.PracticeID,
	}
	if p.Speciality != nil {
		result.Specialty = p.Speciality.Name
	}
	if p.OnlineBooking != nil {
		result.Telehealth = p.OnlineBooking.Telehealth
		result.AgendaIDs = p.OnlineBooking.AgendaIDs
	}
	if p.MatchedVisitMotive != nil {
		result.VisitReason = p.MatchedVisitMotive.Name
		result.VisitMotiveID = p.MatchedVisitMotive.VisitMotiveID
		result.AllowNewPatients = p.MatchedVisitMotive.AllowNewPatients
		if len(p.MatchedVisitMotive.AgendaIDs) > 0 {
			result.AgendaIDs = p.MatchedVisitMotive.AgendaIDs
		}
		if p.MatchedVisitMotive.Insurance != nil {
			result.InsuranceSector = strings.ToLower(p.MatchedVisitMotive.Insurance.Type)
		}
	}
	if opts.hasCoordinates && p.Location.Lat != nil && p.Location.Lng != nil {
		d := haversineKM(opts.lat, opts.lng, *p.Location.Lat, *p.Location.Lng)
		result.DistanceKM = &d
	} else if p.Location.DistanceInMeters != nil {
		d := *p.Location.DistanceInMeters / 1000
		result.DistanceKM = &d
	}
	return result
}

func providerMatches(p healthcareProvider, opts findDoctorsOptions) bool {
	if p.MatchedVisitMotive == nil || p.OnlineBooking == nil {
		return false
	}
	if !opts.includeTelehealth && p.OnlineBooking.Telehealth {
		return false
	}
	if opts.visitReason != "" && !containsFold(p.MatchedVisitMotive.Name, opts.visitReason) {
		return false
	}
	return true
}

func applyAvailability(result *doctorResult, availability availabilityResponse) {
	result.TotalSlots = availability.Total
	for _, day := range availability.Availabilities {
		for _, raw := range day.Slots {
			slot := parseSlot(raw)
			if slot == "" {
				continue
			}
			if result.FirstAvailable == "" {
				result.FirstAvailable = slot
			}
			if len(result.Slots) < 5 {
				result.Slots = append(result.Slots, slot)
			}
		}
	}
}

func parseSlot(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var obj map[string]any
	if json.Unmarshal(raw, &obj) != nil {
		return ""
	}
	for _, key := range []string{"start_date", "startDate", "date"} {
		if v, ok := obj[key].(string); ok {
			return v
		}
	}
	return ""
}

func applyBrowserHeaders(req *http.Request, c *client.Client, referer string) {
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0 Safari/537.36")
	req.Header.Set("Accept-Language", "de-DE,de;q=0.9,en;q=0.8")
	if referer != "" {
		req.Header.Set("Referer", referer)
		req.Header.Set("Origin", strings.TrimRight(c.BaseURL, "/"))
	}
	if c.Config != nil {
		for k, v := range c.Config.Headers {
			req.Header.Set(k, v)
		}
	}
}

func doRequest(httpClient *http.Client, req *http.Request) (*http.Response, []byte, error) {
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp, nil, err
	}
	return resp, body, nil
}

func availabilityCutoff(days int) string {
	now := time.Now()
	y, m, d := now.AddDate(0, 0, days).Date()
	localMidnight := time.Date(y, m, d, 0, 0, 0, 0, now.Location())
	return localMidnight.UTC().Format("2006-01-02T15:04:05.000Z")
}

func extractBetween(s, startMarker, endMarker string) (string, bool) {
	start := strings.Index(s, startMarker)
	if start < 0 {
		return "", false
	}
	start += len(startMarker)
	end := strings.Index(s[start:], endMarker)
	if end < 0 {
		return "", false
	}
	return strings.TrimSpace(s[start : start+end]), true
}

func extractCSRF(s string) string {
	re := regexp.MustCompile(`<meta name="csrf-token" content="([^"]+)"`)
	match := re.FindStringSubmatch(s)
	if len(match) < 2 {
		return ""
	}
	return html.UnescapeString(match[1])
}

func normalizeReasonSlug(reason string) string {
	slug := slugify(reason)
	aliases := map[string]string{
		"allgemeinmediziner": "allgemeinmedizin",
		"allgemeinmedizin":   "allgemeinmedizin",
		"hausarzt":           "allgemeinmedizin",
		"hausaerzte":         "allgemeinmedizin",
		"hausaerztin":        "allgemeinmedizin",
		"hautarzt":           "hautarzt-dermatologe",
		"dermatologe":        "hautarzt-dermatologe",
		"dermatologie":       "hautarzt-dermatologe",
		"zahnarzt":           "zahnarzt",
		"gynaekologe":        "frauenarzt-gynakologe",
		"frauenarzt":         "frauenarzt-gynakologe",
		"kinderarzt":         "kinderarzt",
		"orthopaede":         "orthopade",
		// PATCH: Doctolib uses the specialty slug "proktologie" for proctologists.
		"proktologe":  "proktologie",
		"proktologin": "proktologie",
		"proktologen": "proktologie",
	}
	if v, ok := aliases[slug]; ok {
		return v
	}
	return slug
}

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	replacements := strings.NewReplacer(
		"\u00e4", "ae",
		"\u00f6", "oe",
		"\u00fc", "ue",
		"\u00df", "ss",
		"\u00e9", "e",
		"\u00e8", "e",
		"\u00e1", "a",
		"\u00e0", "a",
	)
	s = replacements.Replace(s)
	var b strings.Builder
	lastDash := false
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

func splitPath(path string) []string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func absoluteURL(baseURL, link string) string {
	u, err := url.Parse(link)
	if err != nil {
		return link
	}
	if u.IsAbs() {
		return u.String()
	}
	base, err := url.Parse(strings.TrimRight(baseURL, "/"))
	if err != nil {
		return link
	}
	return base.ResolveReference(u).String()
}

func joinInts(values []int, sep string) string {
	parts := make([]string, 0, len(values))
	for _, v := range values {
		parts = append(parts, strconv.Itoa(v))
	}
	return strings.Join(parts, sep)
}

func nonEmpty(values []string) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func containsFold(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

func haversineKM(lat1, lon1, lat2, lon2 float64) float64 {
	const earthRadiusKM = 6371.0
	toRad := func(deg float64) float64 { return deg * math.Pi / 180 }
	dLat := toRad(lat2 - lat1)
	dLon := toRad(lon2 - lon1)
	lat1Rad := toRad(lat1)
	lat2Rad := toRad(lat2)
	a := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(lat1Rad)*math.Cos(lat2Rad)*math.Sin(dLon/2)*math.Sin(dLon/2)
	return earthRadiusKM * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}

func sleepForRate(rate float64) {
	if rate <= 0 {
		return
	}
	time.Sleep(time.Duration(float64(time.Second) / rate))
}
