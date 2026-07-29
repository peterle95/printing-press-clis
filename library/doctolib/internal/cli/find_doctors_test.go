// Copyright 2026 peter-moelzer. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

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
	if want := "https://www.doctolib.de/proktologie/berlin"; gotURL != want {
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
		if want := "https://www.doctolib.de/frauenarzt/berlin"; gotURL != want {
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
	if want := "https://www.doctolib.de/urologie/berlin"; gotURL != want {
		t.Fatalf("url = %q, want %q", gotURL, want)
	}
}
