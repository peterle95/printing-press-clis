// Package instruments owns canonical instrument identity and metadata.
package instruments

import (
	"fmt"
	"strings"
	"time"
)

type Instrument struct {
	ISIN            string    `json:"isin"`
	Name            string    `json:"name"`
	Kind            string    `json:"kind,omitempty"`
	Symbol          string    `json:"symbol,omitempty"`
	Country         string    `json:"country,omitempty"`
	Sector          string    `json:"sector,omitempty"`
	Domicile        string    `json:"domicile,omitempty"`
	BaseCurrency    string    `json:"base_currency,omitempty"`
	TradingCurrency string    `json:"trading_currency,omitempty"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type Alias struct {
	Alias string `json:"alias"`
	ISIN  string `json:"isin"`
	Kind  string `json:"kind"`
}

func NormalizeISIN(value string) string {
	return strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(value), " ", ""))
}

// ValidateISIN verifies the ISO 6166 length/character rules and Luhn check
// digit. ISIN remains the canonical identity; symbols are aliases only.
func ValidateISIN(value string) error {
	value = NormalizeISIN(value)
	if len(value) != 12 {
		return fmt.Errorf("invalid ISIN %q: expected 12 characters", value)
	}
	digits := make([]byte, 0, 24)
	for _, r := range value {
		switch {
		case r >= '0' && r <= '9':
			digits = append(digits, byte(r-'0'))
		case r >= 'A' && r <= 'Z':
			n := int(r-'A') + 10
			digits = append(digits, byte(n/10), byte(n%10))
		default:
			return fmt.Errorf("invalid ISIN %q: unsupported character", value)
		}
	}
	sum := 0
	double := false
	for i := len(digits) - 1; i >= 0; i-- {
		n := int(digits[i])
		if double {
			n *= 2
			if n > 9 {
				n -= 9
			}
		}
		sum += n
		double = !double
	}
	if sum%10 != 0 {
		return fmt.Errorf("invalid ISIN %q: check digit mismatch", value)
	}
	return nil
}
