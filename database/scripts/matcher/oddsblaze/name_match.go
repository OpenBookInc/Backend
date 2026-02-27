package oddsblaze

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// commonSuffixes are name suffixes that vendors may omit (e.g., OddsBlaze drops "Jr." from "Kevin Porter Jr.").
var commonSuffixes = []string{"JR", "SR", "II", "III", "IV", "V"}

// removeAccents decomposes unicode characters into base + combining marks (NFD),
// strips the combining marks (accents), then recomposes (NFC).
// For example, "ë" (U+00EB) → "e" + combining diaeresis → "e".
func removeAccents(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range norm.NFD.String(s) {
		if !unicode.Is(unicode.Mn, r) { // Mn = Mark, Nonspacing (combining accents)
			b.WriteRune(r)
		}
	}
	return b.String()
}

// normalizeName uppercases, removes periods, and strips accents from a name.
func normalizeName(name string) string {
	return strings.ToUpper(removeAccents(strings.ReplaceAll(name, ".", "")))
}

// stripSuffix removes a trailing common name suffix (Jr, Sr, II, III, IV, V) from a normalized name.
// Returns the trimmed name and whether a suffix was stripped.
func stripSuffix(normalized string) (string, bool) {
	for _, suffix := range commonSuffixes {
		if strings.HasSuffix(normalized, " "+suffix) {
			return strings.TrimSpace(normalized[:len(normalized)-len(suffix)]), true
		}
	}
	return normalized, false
}

// extractLastName returns the last whitespace-delimited token from a normalized name.
func extractLastName(normalized string) string {
	parts := strings.Fields(normalized)
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}

// namesMatch reports whether two names (one from OddsBlaze, one from the DB) refer to the same person.
//
// Strategy (applied to normalized, suffix-stripped names):
//  1. Exact match after suffix stripping (e.g., "Kevin Porter Jr." == "Kevin Porter").
//  2. Last-name match + first-initial match (e.g., "Ron Holland" == "Ronald Holland" — both "R" + "Holland").
func namesMatch(oddsBlazeRaw, dbRaw string) bool {
	obNorm := normalizeName(oddsBlazeRaw)
	dbNorm := normalizeName(dbRaw)

	// Strip suffixes from both sides (OddsBlaze usually already lacks them, but be safe).
	obStripped, _ := stripSuffix(obNorm)
	dbStripped, _ := stripSuffix(dbNorm)

	// 1. Exact match after suffix stripping.
	if obStripped != "" && obStripped == dbStripped {
		return true
	}

	// 2. Last-name match + first-initial match.
	obParts := strings.Fields(obStripped)
	dbParts := strings.Fields(dbStripped)
	if len(obParts) == 0 || len(dbParts) == 0 {
		return false
	}

	obLast := obParts[len(obParts)-1]
	dbLast := dbParts[len(dbParts)-1]
	if obLast != dbLast {
		return false
	}

	// Compare first initials.
	return obParts[0][0] == dbParts[0][0]
}

// jerseyAndLastNameMatch reports whether two individuals share the same jersey number
// and last name. This is the fallback when namesMatch returns false.
func jerseyAndLastNameMatch(obName, obJersey, dbName, dbJersey string) bool {
	if obJersey == "" || dbJersey == "" || obJersey != dbJersey {
		return false
	}

	obNorm := normalizeName(obName)
	dbNorm := normalizeName(dbName)
	dbStripped, _ := stripSuffix(dbNorm)
	obStripped, _ := stripSuffix(obNorm)

	return extractLastName(obStripped) == extractLastName(dbStripped)
}
