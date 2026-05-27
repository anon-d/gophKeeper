package crypto

import "testing"

func TestValidateLuhn(t *testing.T) {
	tests := []struct {
		name   string
		number string
		want   bool
	}{
		{"Visa valid", "4532015112830366", true},
		{"Visa with spaces", "4532 0151 1283 0366", true},
		{"Visa with dashes", "4532-0151-1283-0366", true},
		{"Mastercard valid", "5425233430109903", true},
		{"Mir valid", "2202200000000000", false}, // не проходит Луна
		{"Invalid number", "1234567890123456", false},
		{"All zeros", "0000000000000000", true}, // технически проходит
		{"Too short", "123", false},
		{"Too long", "12345678901234567890", false},
		{"Letters", "4532abcd11283036", false},
		{"Empty", "", false},
		{"Single digit change", "4532015112830367", false}, // последняя цифра изменена
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ValidateLuhn(tt.number)
			if got != tt.want {
				t.Errorf("ValidateLuhn(%q) = %v, want %v", tt.number, got, tt.want)
			}
		})
	}
}
