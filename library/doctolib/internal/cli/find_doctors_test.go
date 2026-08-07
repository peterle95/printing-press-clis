// Copyright 2026 peter-moelzer. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"doctolib-pp-cli/internal/client"
)

// PATCH: regression coverage for local proctology alias customization.
func TestNormalizeReasonSlugProctologyAliases(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"proktologe":  "proktologie",
		"Proktologin": "proktologie",
		"Proktologen": "proktologie",
		"Proktologie": "proktologie",
	}
	for in, want := range cases {
		in, want := in, want
		t.Run(in, func(t *testing.T) {
			t.Parallel()
			if got := normalizeReasonSlug(in); got != want {
				t.Fatalf("normalizeReasonSlug(%q) = %q, want %q", in, got, want)
			}
		})
	}
}

func TestResolveSearchURLUsesProctologyAlias(t *testing.T) {
	t.Parallel()
	gotURL, gotReason, gotLocation, err := resolveSearchURL("https://www.doctolib.de", findDoctorsOptions{
		reason:   "proktologe",
		location: "Berlin",
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotReason != "proktologie" {
		t.Fatalf("reason = %q, want %q", gotReason, "proktologie")
	}
	if gotLocation != "berlin" {
		t.Fatalf("location = %q, want %q", gotLocation, "berlin")
	}
	if want := "https://www.doctolib.de/search?city=berlin&keyword=proktologie"; gotURL != want {
		t.Fatalf("url = %q, want %q", gotURL, want)
	}
}

func TestResolveSearchURLUsesCurrentGynecologyAlias(t *testing.T) {
	t.Parallel()
	for _, reason := range []string{"frauenarzt", "Gynäkologe", "gynaekologe"} {
		gotURL, gotReason, gotLocation, err := resolveSearchURL("https://www.doctolib.de", findDoctorsOptions{
			reason:   reason,
			location: "Berlin",
		})
		if err != nil {
			t.Fatal(err)
		}
		if gotReason != "frauenarzt" || gotLocation != "berlin" {
			t.Fatalf("resolveSearchURL(%q) = reason %q, location %q", reason, gotReason, gotLocation)
		}
		if want := "https://www.doctolib.de/search?city=berlin&keyword=frauenarzt"; gotURL != want {
			t.Fatalf("resolveSearchURL(%q) = %q, want %q", reason, gotURL, want)
		}
	}
}

func TestResolveSearchURLUsesCurrentUrologyAlias(t *testing.T) {
	t.Parallel()
	gotURL, gotReason, gotLocation, err := resolveSearchURL("https://www.doctolib.de", findDoctorsOptions{
		reason:   "Urologe",
		location: "Berlin",
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotReason != "urologie" || gotLocation != "berlin" {
		t.Fatalf("resolveSearchURL = reason %q, location %q", gotReason, gotLocation)
	}
	if want := "https://www.doctolib.de/search?city=berlin&keyword=urologie"; gotURL != want {
		t.Fatalf("url = %q, want %q", gotURL, want)
	}
}

// PATCH: cover current Doctolib provider search and strict public-insurance filtering.
func TestFetchProvidersPageUsesCurrentEndpointAndPublicFilter(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/patient-health-search/api/v1/hcp/search" {
			http.NotFound(w, r)
			return
		}
		if got := r.URL.Query().Get("page"); got != "2" {
			t.Errorf("page = %q, want %q", got, "2")
		}
		var body struct {
			Keyword string `json:"keyword"`
			Filters struct {
				InsuranceSector string `json:"insuranceSector"`
			} `json:"filters"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body.Keyword != "hautarzt" || body.Filters.InsuranceSector != "public" {
			t.Errorf("body = %+v, want hautarzt/public", body)
		}
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write([]byte(`{"total":1,"healthcareProviders":[]}`))
	}))
	defer server.Close()

	ctx := searchContext{Place: json.RawMessage(`{"id":137}`), URL: server.URL}
	ctx.Keyword.Slug = "hautarzt"
	got, err := fetchProvidersPage(server.Client(), &client.Client{BaseURL: server.URL}, ctx, findDoctorsOptions{
		insuranceSector: "public",
		withinDays:      14,
	}, 2)
	if err != nil {
		t.Fatal(err)
	}
	if got.Total != 1 {
		t.Fatalf("total = %d, want 1", got.Total)
	}
}

func TestProviderMatchesRejectsWrongInsuranceSector(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		sector          string
		motive          string
		privatePractice bool
		privateLink     bool
		requestedSector string
		want            bool
	}{
		"public covered":        {sector: "PUBLIC", motive: "Dermatologische Sprechstunde", want: true},
		"private":               {sector: "PRIVATE", motive: "Dermatologische Sprechstunde", want: false},
		"self pay":              {sector: "PUBLIC", motive: "Hautproblem Beratung - SELBSTZAHLER / SELF-PAYING", want: false},
		"priced":                {sector: "PUBLIC", motive: "Hautkrebsscreening mit digitaler Fotodokumentation – 139 €", want: false},
		"private practice":      {sector: "PUBLIC", motive: "Dermatologische Sprechstunde", privatePractice: true, want: false},
		"private practice link": {sector: "PUBLIC", motive: "Dermatologische Sprechstunde", privateLink: true, want: false},
		"exclusive":             {sector: "PUBLIC", motive: "curavie - Exklusivtermin", want: false},
		"private covered":       {sector: "PRIVATE", motive: "Privatsprechstunde", privatePractice: true, requestedSector: "private", want: true},
	} {
		t.Run(name, func(t *testing.T) {
			organizationStatus := ""
			if tc.privatePractice {
				organizationStatus = `,"organizationStatus":{"slug":"privatpraxis"}`
			}
			var provider healthcareProvider
			if err := json.Unmarshal([]byte(`{
				"onlineBooking":{"agendaIds":[1]},
				"matchedVisitMotive":{"visitMotiveId":1,"agendaIds":[1],"allowNewPatients":true,"insuranceSector":{"type":"`+tc.sector+`"},"name":"`+tc.motive+`"}`+organizationStatus+`
			}`), &provider); err != nil {
				t.Fatal(err)
			}
			if tc.privateLink {
				provider.Link = "/privatpraxis/berlin/test"
			}
			requestedSector := tc.requestedSector
			if requestedSector == "" {
				requestedSector = "public"
			}
			if got := providerMatches(provider, findDoctorsOptions{insuranceSector: requestedSector}); got != tc.want {
				t.Fatalf("providerMatches(%s) = %t, want %t", tc.motive, got, tc.want)
			}
		})
	}
}
