package utils

import (
	"errors"
	"fmt"
	"strings"
)

// PortionToMultiplier converts a pool portion to a multiplier
// multiplier = (totalUnits / portion - 1) * multiplierScale (truncated)
func PortionToMultiplier(portion, totalUnits, multiplierScale uint64) (uint64, error) {
	if portion == 0 {
		return 0, errors.New("portion cannot be zero")
	}
	if totalUnits == 0 {
		return 0, errors.New("totalUnits cannot be zero")
	}
	if multiplierScale == 0 {
		return 0, errors.New("multiplierScale cannot be zero")
	}
	if portion > totalUnits {
		return 0, errors.New("portion cannot exceed totalUnits")
	}

	// Calculate: (totalUnits / portion - 1) * multiplierScale
	// Rearrange to: (totalUnits - portion) * multiplierScale / portion
	// This avoids intermediate overflow and maintains precision
	numerator := (totalUnits - portion) * multiplierScale
	multiplier := numerator / portion

	return multiplier, nil
}

// MultiplierToPortion converts a multiplier to a pool portion
// portion = totalUnits * multiplierScale / (multiplier + multiplierScale) (truncated)
func MultiplierToPortion(multiplier, totalUnits, multiplierScale uint64) (uint64, error) {
	if totalUnits == 0 {
		return 0, errors.New("totalUnits cannot be zero")
	}
	if multiplierScale == 0 {
		return 0, errors.New("multiplierScale cannot be zero")
	}

	// Calculate: totalUnits * multiplierScale / (multiplier + multiplierScale)
	denominator := multiplier + multiplierScale
	if denominator == 0 {
		return 0, errors.New("denominator cannot be zero")
	}

	numerator := totalUnits * multiplierScale
	portion := numerator / denominator

	if portion == 0 {
		return 0, errors.New("calculated portion is zero")
	}

	return portion, nil
}

// MultiplierToString converts a multiplier to a string representation
// Truncates (does not round) to the specified number of decimals
func MultiplierToString(multiplier, multiplierScale uint64, decimals int) (string, error) {
	if multiplierScale == 0 {
		return "", errors.New("multiplierScale cannot be zero")
	}
	if decimals < 0 {
		return "", errors.New("decimals cannot be negative")
	}

	// Calculate the integer part
	integerPart := multiplier / multiplierScale
	remainder := multiplier % multiplierScale

	if decimals == 0 {
		return fmt.Sprintf("%d", integerPart), nil
	}

	// Calculate decimal part by scaling up the remainder
	decimalScale := uint64(1)
	for i := 0; i < decimals; i++ {
		decimalScale *= 10
	}

	decimalPart := (remainder * decimalScale) / multiplierScale

	// Format with leading zeros if needed
	formatStr := fmt.Sprintf("%%d.%%0%dd", decimals)
	return fmt.Sprintf(formatStr, integerPart, decimalPart), nil
}

// CentsToUnits converts cents to units by scaling
func CentsToUnits(cents, unitScale uint64) uint64 {
	return cents * unitScale
}

// UnitsToQuantity converts units to quantity given a portion
// quantity = units / portion (floor division)
func UnitsToQuantity(units, portion uint64) (uint64, error) {
	if portion == 0 {
		return 0, errors.New("portion cannot be zero")
	}

	quantity := units / portion
	return quantity, nil
}

// QuantityToUnits converts quantity to units given a portion
func QuantityToUnits(quantity, portion uint64) uint64 {
	return quantity * portion
}

// UnitsToCents converts units back to cents by dividing by scale
// Uses floor division
func UnitsToCents(units, unitScale uint64) (uint64, error) {
	if unitScale == 0 {
		return 0, errors.New("unitScale cannot be zero")
	}

	cents := units / unitScale
	return cents, nil
}

// CentsToString converts scaled cents to a dollar string format
// Rounds up any fractional cent (ceiling operation)
func CentsToString(scaledCents, centScale uint64) (string, error) {
	if centScale == 0 {
		return "", errors.New("centScale cannot be zero")
	}

	// Calculate whole cents with ceiling
	wholeCents := scaledCents / centScale
	remainder := scaledCents % centScale

	// If there's any remainder, round up
	if remainder > 0 {
		wholeCents++
	}

	// Convert to dollars and cents
	dollars := wholeCents / 100
	cents := wholeCents % 100

	return fmt.Sprintf("$%d.%02d", dollars, cents), nil
}

// Helper function to format with specific decimal places
func formatWithDecimals(value uint64, scale uint64, decimals int) string {
	intPart := value / scale
	remainder := value % scale

	if decimals == 0 {
		return fmt.Sprintf("%d", intPart)
	}

	decimalScale := uint64(1)
	for i := 0; i < decimals; i++ {
		decimalScale *= 10
	}

	decimalPart := (remainder * decimalScale) / scale
	formatStr := fmt.Sprintf("%%d.%%0%dd", decimals)

	return fmt.Sprintf(formatStr, intPart, decimalPart)
}

// PadLeft pads a string with zeros on the left
func padLeft(s string, length int) string {
	if len(s) >= length {
		return s
	}
	return strings.Repeat("0", length-len(s)) + s
}
