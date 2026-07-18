// Package money implements fixed-scale decimal arithmetic for financial data.
package money

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
)

const (
	Scale       int64 = 100_000_000
	ScaleDigits       = 8
)

// Decimal stores eight fractional decimal places. It deliberately avoids
// binary floating-point so imported prices, quantities, fees, and taxes remain
// stable across repeated syncs.
type Decimal int64

func Parse(value string) (Decimal, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, nil
	}
	r, ok := new(big.Rat).SetString(value)
	if !ok {
		return 0, fmt.Errorf("invalid decimal %q", value)
	}
	scaled := new(big.Rat).Mul(r, big.NewRat(Scale, 1))
	q := new(big.Int)
	rem := new(big.Int)
	q.QuoRem(scaled.Num(), scaled.Denom(), rem)
	// Round half away from zero when more than eight fractional places exist.
	absRem := new(big.Int).Abs(rem)
	twiceRem := new(big.Int).Lsh(absRem, 1)
	if twiceRem.Cmp(scaled.Denom()) >= 0 {
		if scaled.Sign() < 0 {
			q.Sub(q, big.NewInt(1))
		} else {
			q.Add(q, big.NewInt(1))
		}
	}
	if !q.IsInt64() {
		return 0, fmt.Errorf("decimal %q is out of range", value)
	}
	return Decimal(q.Int64()), nil
}

func MustParse(value string) Decimal {
	d, err := Parse(value)
	if err != nil {
		panic(err)
	}
	return d
}

func FromInt(value int64) Decimal { return Decimal(value * Scale) }

func (d Decimal) String() string {
	n := int64(d)
	negative := n < 0
	if negative {
		n = -n
	}
	whole := n / Scale
	fraction := n % Scale
	value := fmt.Sprintf("%d.%0*d", whole, ScaleDigits, fraction)
	value = strings.TrimRight(strings.TrimRight(value, "0"), ".")
	if value == "" {
		value = "0"
	}
	if negative && value != "0" {
		value = "-" + value
	}
	return value
}

func (d Decimal) Fixed(places int) string {
	if places < 0 {
		places = 0
	}
	if places > ScaleDigits {
		places = ScaleDigits
	}
	n := int64(d)
	negative := n < 0
	if negative {
		n = -n
	}
	divisor := int64(1)
	for range ScaleDigits - places {
		divisor *= 10
	}
	rounded := (n + divisor/2) / divisor
	unit := Scale / divisor
	var value string
	if places == 0 {
		value = fmt.Sprintf("%d", rounded)
	} else {
		value = fmt.Sprintf("%d.%0*d", rounded/unit, places, rounded%unit)
	}
	if negative && rounded != 0 {
		value = "-" + value
	}
	return value
}

func (d Decimal) Add(other Decimal) Decimal { return d + other }
func (d Decimal) Sub(other Decimal) Decimal { return d - other }
func (d Decimal) Neg() Decimal              { return -d }
func (d Decimal) IsZero() bool              { return d == 0 }
func (d Decimal) Cmp(other Decimal) int {
	if d < other {
		return -1
	}
	if d > other {
		return 1
	}
	return 0
}
func (d Decimal) Abs() Decimal {
	if d < 0 {
		return -d
	}
	return d
}

func (d Decimal) Mul(other Decimal) Decimal {
	product := new(big.Int).Mul(big.NewInt(int64(d)), big.NewInt(int64(other)))
	quotient, remainder := new(big.Int), new(big.Int)
	quotient.QuoRem(product, big.NewInt(Scale), remainder)
	if new(big.Int).Lsh(new(big.Int).Abs(remainder), 1).Cmp(big.NewInt(Scale)) >= 0 {
		if product.Sign() < 0 {
			quotient.Sub(quotient, big.NewInt(1))
		} else {
			quotient.Add(quotient, big.NewInt(1))
		}
	}
	return Decimal(quotient.Int64())
}

func (d Decimal) Div(other Decimal) (Decimal, error) {
	if other == 0 {
		return 0, fmt.Errorf("division by zero")
	}
	return Parse(new(big.Rat).SetFrac(big.NewInt(int64(d)), big.NewInt(int64(other))).FloatString(ScaleDigits))
}

func (d Decimal) MarshalJSON() ([]byte, error) { return json.Marshal(d.String()) }

func (d *Decimal) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if bytes.Equal(data, []byte("null")) {
		*d = 0
		return nil
	}
	var value string
	if len(data) > 0 && data[0] == '"' {
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
	} else {
		value = string(data)
	}
	parsed, err := Parse(value)
	if err != nil {
		return err
	}
	*d = parsed
	return nil
}

func (d *Decimal) UnmarshalText(text []byte) error {
	parsed, err := Parse(string(text))
	if err != nil {
		return err
	}
	*d = parsed
	return nil
}

func (d Decimal) MarshalText() ([]byte, error) { return []byte(d.String()), nil }
