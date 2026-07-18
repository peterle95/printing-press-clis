package instruments

import "testing"

func TestValidateISIN(t *testing.T) {
	t.Parallel()
	for _, value := range []string{"IE00B4L5Y983", "US5949181045", "DE0007164600"} {
		if err := ValidateISIN(value); err != nil {
			t.Errorf("ValidateISIN(%q): %v", value, err)
		}
	}
	for _, value := range []string{"ASML", "IE00B4L5Y984", "IE00B4L5Y98!"} {
		if err := ValidateISIN(value); err == nil {
			t.Errorf("ValidateISIN(%q) unexpectedly succeeded", value)
		}
	}
}
