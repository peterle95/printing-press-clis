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
