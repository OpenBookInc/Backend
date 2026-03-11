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

func TestUUIDScan(t *testing.T) {
	expected := UUIDFromUint64(0x0123456789ABCDEF, 0xFEDCBA9876543210)

	// Scan from string
	var u1 UUID
	err := u1.Scan("01234567-89ab-cdef-fedc-ba9876543210")
	if err != nil {
		t.Fatalf("Scan(string) error: %v", err)
	}
	if u1 != expected {
		t.Errorf("Scan(string) = %v, want %v", u1, expected)
	}

	// Scan from []byte (16 bytes raw)
	var u2 UUID
	err = u2.Scan(expected[:])
	if err != nil {
		t.Fatalf("Scan([]byte raw) error: %v", err)
	}
	if u2 != expected {
		t.Errorf("Scan([]byte raw) = %v, want %v", u2, expected)
	}

	// Scan from []byte (string form)
	var u3 UUID
	err = u3.Scan([]byte("01234567-89ab-cdef-fedc-ba9876543210"))
	if err != nil {
		t.Fatalf("Scan([]byte string) error: %v", err)
	}
	if u3 != expected {
		t.Errorf("Scan([]byte string) = %v, want %v", u3, expected)
	}

	// Scan nil
	var u4 UUID
	err = u4.Scan(nil)
	if err != nil {
		t.Fatalf("Scan(nil) error: %v", err)
	}
	if u4 != (UUID{}) {
		t.Errorf("Scan(nil) = %v, want zero UUID", u4)
	}
}

func TestUUIDValue(t *testing.T) {
	u := UUIDFromUint64(0x0123456789ABCDEF, 0xFEDCBA9876543210)
	v, err := u.Value()
	if err != nil {
		t.Fatalf("Value() error: %v", err)
	}
	s, ok := v.(string)
	if !ok {
		t.Fatalf("Value() returned %T, want string", v)
	}
	if s != "01234567-89ab-cdef-fedc-ba9876543210" {
		t.Errorf("Value() = %q, want %q", s, "01234567-89ab-cdef-fedc-ba9876543210")
	}
}
