package utils

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
)

// UUID represents a 128-bit universally unique identifier.
type UUID [16]byte

// UUIDFromUint64 creates a UUID from upper and lower 64-bit halves (big-endian).
func UUIDFromUint64(upper, lower uint64) UUID {
	var u UUID
	binary.BigEndian.PutUint64(u[:8], upper)
	binary.BigEndian.PutUint64(u[8:], lower)
	return u
}

// Upper returns the upper 64 bits of the UUID (big-endian).
func (u UUID) Upper() uint64 {
	return binary.BigEndian.Uint64(u[:8])
}

// Lower returns the lower 64 bits of the UUID (big-endian).
func (u UUID) Lower() uint64 {
	return binary.BigEndian.Uint64(u[8:])
}

// String returns the standard UUID string representation (xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx).
func (u UUID) String() string {
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		u[:4], u[4:6], u[6:8], u[8:10], u[10:])
}

// ParseUUID parses a UUID from its standard string representation.
// Accepts both "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx" and "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx" formats.
func ParseUUID(s string) (UUID, error) {
	// Strip hyphens
	clean := make([]byte, 0, 32)
	for i := 0; i < len(s); i++ {
		if s[i] != '-' {
			clean = append(clean, s[i])
		}
	}

	if len(clean) != 32 {
		return UUID{}, fmt.Errorf("invalid UUID string: %q", s)
	}

	var u UUID
	_, err := hex.Decode(u[:], clean)
	if err != nil {
		return UUID{}, fmt.Errorf("invalid UUID hex: %w", err)
	}
	return u, nil
}
