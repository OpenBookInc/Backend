package utils

import (
	"testing"
)

func TestUUIDFromUint64RoundTrip(t *testing.T) {
	tests := []struct {
		upper uint64
		lower uint64
	}{
		{0, 0},
		{1, 2},
		{0xFFFFFFFFFFFFFFFF, 0xFFFFFFFFFFFFFFFF},
		{0x0123456789ABCDEF, 0xFEDCBA9876543210},
	}

	for _, tt := range tests {
		u := UUIDFromUint64(tt.upper, tt.lower)
		if u.Upper() != tt.upper {
			t.Errorf("Upper() = %d, want %d", u.Upper(), tt.upper)
		}
		if u.Lower() != tt.lower {
			t.Errorf("Lower() = %d, want %d", u.Lower(), tt.lower)
		}
	}
}

func TestUUIDString(t *testing.T) {
	u := UUIDFromUint64(0x0123456789ABCDEF, 0xFEDCBA9876543210)
	expected := "01234567-89ab-cdef-fedc-ba9876543210"
	if u.String() != expected {
		t.Errorf("String() = %q, want %q", u.String(), expected)
	}
}

func TestParseUUID(t *testing.T) {
	tests := []struct {
		input string
		upper uint64
		lower uint64
	}{
		{"01234567-89ab-cdef-fedc-ba9876543210", 0x0123456789ABCDEF, 0xFEDCBA9876543210},
		{"0123456789abcdeffedcba9876543210", 0x0123456789ABCDEF, 0xFEDCBA9876543210},
		{"00000000-0000-0000-0000-000000000000", 0, 0},
	}

	for _, tt := range tests {
		u, err := ParseUUID(tt.input)
		if err != nil {
			t.Errorf("ParseUUID(%q) error: %v", tt.input, err)
			continue
		}
		if u.Upper() != tt.upper {
			t.Errorf("ParseUUID(%q).Upper() = %d, want %d", tt.input, u.Upper(), tt.upper)
		}
		if u.Lower() != tt.lower {
			t.Errorf("ParseUUID(%q).Lower() = %d, want %d", tt.input, u.Lower(), tt.lower)
		}
	}
}

func TestParseUUIDInvalid(t *testing.T) {
	invalid := []string{
		"",
		"not-a-uuid",
		"01234567-89ab-cdef-fedc-ba987654321",  // too short
		"01234567-89ab-cdef-fedc-ba98765432100", // too long
		"0123456789abcdeffedcba987654321g",       // invalid hex
	}

	for _, s := range invalid {
		_, err := ParseUUID(s)
		if err == nil {
			t.Errorf("ParseUUID(%q) expected error, got nil", s)
		}
	}
}

func TestParseUUIDRoundTrip(t *testing.T) {
	original := "01234567-89ab-cdef-fedc-ba9876543210"
	u, err := ParseUUID(original)
	if err != nil {
		t.Fatalf("ParseUUID(%q) error: %v", original, err)
	}
	if u.String() != original {
		t.Errorf("round-trip: got %q, want %q", u.String(), original)
	}
}
