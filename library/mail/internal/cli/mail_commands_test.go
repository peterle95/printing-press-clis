package cli

import "testing"

func TestValidateReadableBodyRejectsLongSingleLineLetter(t *testing.T) {
	body := "Hallo Daniela, vielen Dank fuer die Einladung zum Kennenlernen. Ich habe am Freitag um 11:00 Uhr Zeit fuer den Video-Call. Beste Gruesse, Peter"
	if err := validateReadableBody(body, false); err == nil {
		t.Fatal("expected long single-line body to be rejected")
	}
}

func TestNormalizeBodyTextConvertsEscapedNewlines(t *testing.T) {
	got := normalizeBodyText(`Hallo Daniela,\n\nDanke.\n\nBeste Gruesse,\nPeter`)
	want := "Hallo Daniela,\n\nDanke.\n\nBeste Gruesse,\nPeter\n"
	if got != want {
		t.Fatalf("body = %q, want %q", got, want)
	}
}
