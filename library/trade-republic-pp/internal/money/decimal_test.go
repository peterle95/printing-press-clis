package money

import (
	"encoding/json"
	"testing"
)

func TestParseAndFormat(t *testing.T) {
	t.Parallel()
	tests := map[string]string{
		"0":              "0",
		"12.34000000":    "12.34",
		"-0.00000001":    "-0.00000001",
		"1.234567894":    "1.23456789",
		"1.234567895":    "1.2345679",
		"-1.234567895":   "-1.2345679",
		"1e-5":           "0.00001",
		"123456789.1234": "123456789.1234",
	}
	for input, want := range tests {
		input, want := input, want
		t.Run(input, func(t *testing.T) {
			t.Parallel()
			got, err := Parse(input)
			if err != nil {
				t.Fatal(err)
			}
			if got.String() != want {
				t.Fatalf("Parse(%q) = %q, want %q", input, got.String(), want)
			}
		})
	}
}

func TestJSONUsesExactString(t *testing.T) {
	t.Parallel()
	var got Decimal
	if err := json.Unmarshal([]byte(`0.10000001`), &got); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if string(encoded) != `"0.10000001"` {
		t.Fatalf("encoded = %s", encoded)
	}
}

func TestArithmetic(t *testing.T) {
	t.Parallel()
	if got := MustParse("12.50").Mul(MustParse("2.4")); got.String() != "30" {
		t.Fatalf("product = %s", got)
	}
	got, err := MustParse("1").Div(MustParse("4"))
	if err != nil || got.String() != "0.25" {
		t.Fatalf("division = %s, %v", got, err)
	}
}
