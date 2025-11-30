package utils

import (
	"testing"
)

func TestPortionToMultiplier(t *testing.T) {
	tests := []struct {
		name            string
		portion         uint64
		totalUnits      uint64
		multiplierScale uint64
		wantMultiplier  uint64
		wantErr         bool
	}{
		{
			name:            "example from spec",
			portion:         5119,
			totalUnits:      10000,
			multiplierScale: 1000000,
			wantMultiplier:  953506,
			wantErr:         false,
		},
		{
			name:            "even split",
			portion:         5000,
			totalUnits:      10000,
			multiplierScale: 1000000,
			wantMultiplier:  1000000,
			wantErr:         false,
		},
		{
			name:            "zero portion",
			portion:         0,
			totalUnits:      10000,
			multiplierScale: 1000000,
			wantErr:         true,
		},
		{
			name:            "zero totalUnits",
			portion:         5000,
			totalUnits:      0,
			multiplierScale: 1000000,
			wantErr:         true,
		},
		{
			name:            "zero scale",
			portion:         5000,
			totalUnits:      10000,
			multiplierScale: 0,
			wantErr:         true,
		},
		{
			name:            "portion exceeds totalUnits",
			portion:         15000,
			totalUnits:      10000,
			multiplierScale: 1000000,
			wantErr:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := PortionToMultiplier(tt.portion, tt.totalUnits, tt.multiplierScale)
			if (err != nil) != tt.wantErr {
				t.Errorf("PortionToMultiplier() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.wantMultiplier {
				t.Errorf("PortionToMultiplier() = %v, want %v", got, tt.wantMultiplier)
			}
		})
	}
}

func TestMultiplierToPortion(t *testing.T) {
	tests := []struct {
		name            string
		multiplier      uint64
		totalUnits      uint64
		multiplierScale uint64
		wantPortion     uint64
		wantErr         bool
	}{
		{
			name:            "example from spec",
			multiplier:      953506,
			totalUnits:      10000,
			multiplierScale: 1000000,
			wantPortion:     5119,
			wantErr:         false,
		},
		{
			name:            "zero totalUnits",
			multiplier:      1000000,
			totalUnits:      0,
			multiplierScale: 1000000,
			wantErr:         true,
		},
		{
			name:            "zero scale",
			multiplier:      1000000,
			totalUnits:      10000,
			multiplierScale: 0,
			wantErr:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := MultiplierToPortion(tt.multiplier, tt.totalUnits, tt.multiplierScale)
			if (err != nil) != tt.wantErr {
				t.Errorf("MultiplierToPortion() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.wantPortion {
				t.Errorf("MultiplierToPortion() = %v, want %v", got, tt.wantPortion)
			}
		})
	}
}

func TestPortionMultiplierRoundTrip(t *testing.T) {
	tests := []struct {
		name            string
		initialPortion  uint64
		totalUnits      uint64
		multiplierScale uint64
	}{
		{
			name:            "spec example",
			initialPortion:  5119,
			totalUnits:      10000,
			multiplierScale: 1000000,
		},
		{
			name:            "small portion",
			initialPortion:  100,
			totalUnits:      10000,
			multiplierScale: 1000000,
		},
		{
			name:            "large portion",
			initialPortion:  9900,
			totalUnits:      10000,
			multiplierScale: 1000000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Portion -> Multiplier -> Portion
			multiplier, err := PortionToMultiplier(tt.initialPortion, tt.totalUnits, tt.multiplierScale)
			if err != nil {
				t.Fatalf("PortionToMultiplier() error = %v", err)
			}

			finalPortion, err := MultiplierToPortion(multiplier, tt.totalUnits, tt.multiplierScale)
			if err != nil {
				t.Fatalf("MultiplierToPortion() error = %v", err)
			}

			// Final portion should be <= initial portion
			if finalPortion > tt.initialPortion {
				t.Errorf("Round trip portion increased: initial=%v, final=%v", tt.initialPortion, finalPortion)
			}

			// Multiplier -> Portion -> Multiplier
			portion2, err := MultiplierToPortion(multiplier, tt.totalUnits, tt.multiplierScale)
			if err != nil {
				t.Fatalf("MultiplierToPortion() error = %v", err)
			}

			finalMultiplier, err := PortionToMultiplier(portion2, tt.totalUnits, tt.multiplierScale)
			if err != nil {
				t.Fatalf("PortionToMultiplier() error = %v", err)
			}

			// Final multiplier should be >= initial multiplier
			if finalMultiplier < multiplier {
				t.Errorf("Round trip multiplier decreased: initial=%v, final=%v", multiplier, finalMultiplier)
			}
		})
	}
}

func TestMultiplierToString(t *testing.T) {
	tests := []struct {
		name            string
		multiplier      uint64
		multiplierScale uint64
		decimals        int
		want            string
		wantErr         bool
	}{
		{
			name:            "spec example",
			multiplier:      953506,
			multiplierScale: 1000000,
			decimals:        2,
			want:            "0.95",
			wantErr:         false,
		},
		{
			name:            "no decimals",
			multiplier:      1500000,
			multiplierScale: 1000000,
			decimals:        0,
			want:            "1",
			wantErr:         false,
		},
		{
			name:            "more decimals than scale precision",
			multiplier:      1234,
			multiplierScale: 10000,
			decimals:        6,
			want:            "0.123400",
			wantErr:         false,
		},
		{
			name:            "truncation test",
			multiplier:      123456,
			multiplierScale: 1000000,
			decimals:        2,
			want:            "0.12",
			wantErr:         false,
		},
		{
			name:            "zero scale",
			multiplier:      100,
			multiplierScale: 0,
			decimals:        2,
			wantErr:         true,
		},
		{
			name:            "negative decimals",
			multiplier:      100,
			multiplierScale: 1000,
			decimals:        -1,
			wantErr:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := MultiplierToString(tt.multiplier, tt.multiplierScale, tt.decimals)
			if (err != nil) != tt.wantErr {
				t.Errorf("MultiplierToString() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("MultiplierToString() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCentsToUnits(t *testing.T) {
	tests := []struct {
		name      string
		cents     uint64
		unitScale uint64
		want      uint64
	}{
		{
			name:      "spec example",
			cents:     158,
			unitScale: 1000000,
			want:      158000000,
		},
		{
			name:      "zero cents",
			cents:     0,
			unitScale: 1000000,
			want:      0,
		},
		{
			name:      "one cent",
			cents:     1,
			unitScale: 1000000,
			want:      1000000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CentsToUnits(tt.cents, tt.unitScale)
			if got != tt.want {
				t.Errorf("CentsToUnits() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUnitsToQuantity(t *testing.T) {
	tests := []struct {
		name     string
		units    uint64
		portion  uint64
		want     uint64
		wantErr  bool
	}{
		{
			name:    "spec example",
			units:   158000000,
			portion: 5110,
			want:    30919,
			wantErr: false,
		},
		{
			name:    "zero portion",
			units:   1000000,
			portion: 0,
			wantErr: true,
		},
		{
			name:    "exact division",
			units:   10000,
			portion: 100,
			want:    100,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := UnitsToQuantity(tt.units, tt.portion)
			if (err != nil) != tt.wantErr {
				t.Errorf("UnitsToQuantity() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("UnitsToQuantity() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestQuantityToUnits(t *testing.T) {
	tests := []struct {
		name     string
		quantity uint64
		portion  uint64
		want     uint64
	}{
		{
			name:     "spec example",
			quantity: 30919,
			portion:  5110,
			want:     157996090,
		},
		{
			name:     "simple multiplication",
			quantity: 100,
			portion:  1000,
			want:     100000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := QuantityToUnits(tt.quantity, tt.portion)
			if got != tt.want {
				t.Errorf("QuantityToUnits() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCentsQuantityRoundTrip(t *testing.T) {
	tests := []struct {
		name      string
		cents     uint64
		portion   uint64
		unitScale uint64
	}{
		{
			name:      "spec example",
			cents:     158,
			portion:   5110,
			unitScale: 1000000,
		},
		{
			name:      "small cents",
			cents:     1,
			portion:   100,
			unitScale: 1000000,
		},
		{
			name:      "large cents",
			cents:     10000,
			portion:   2500,
			unitScale: 1000000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Cents -> Units -> Quantity -> Units -> Cents
			units := CentsToUnits(tt.cents, tt.unitScale)
			quantity, err := UnitsToQuantity(units, tt.portion)
			if err != nil {
				t.Fatalf("UnitsToQuantity() error = %v", err)
			}

			finalUnits := QuantityToUnits(quantity, tt.portion)
			finalCents, err := UnitsToCents(finalUnits, tt.unitScale)
			if err != nil {
				t.Fatalf("UnitsToCents() error = %v", err)
			}

			// Final cents should be <= initial cents
			if finalCents > tt.cents {
				t.Errorf("Round trip cents increased: initial=%v, final=%v", tt.cents, finalCents)
			}

			// Quantity -> Units -> Quantity
			units2 := QuantityToUnits(quantity, tt.portion)
			finalQuantity, err := UnitsToQuantity(units2, tt.portion)
			if err != nil {
				t.Fatalf("UnitsToQuantity() error = %v", err)
			}

			// Final quantity should be >= initial quantity
			if finalQuantity < quantity {
				t.Errorf("Round trip quantity decreased: initial=%v, final=%v", quantity, finalQuantity)
			}
		})
	}
}

func TestCentsToString(t *testing.T) {
	tests := []struct {
		name        string
		scaledCents uint64
		centScale   uint64
		want        string
		wantErr     bool
	}{
		{
			name:        "exact cents",
			scaledCents: 158000000,
			centScale:   1000000,
			want:        "$1.58",
			wantErr:     false,
		},
		{
			name:        "ceiling fractional cent",
			scaledCents: 158000001,
			centScale:   1000000,
			want:        "$1.59",
			wantErr:     false,
		},
		{
			name:        "zero cents",
			scaledCents: 0,
			centScale:   1000000,
			want:        "$0.00",
			wantErr:     false,
		},
		{
			name:        "large dollar amount",
			scaledCents: 100000000000,
			centScale:   1000000,
			want:        "$1000.00",
			wantErr:     false,
		},
		{
			name:        "zero scale",
			scaledCents: 158,
			centScale:   0,
			wantErr:     true,
		},
		{
			name:        "99 cents",
			scaledCents: 99000000,
			centScale:   1000000,
			want:        "$0.99",
			wantErr:     false,
		},
		{
			name:        "ceiling to next dollar",
			scaledCents: 99999999,
			centScale:   1000000,
			want:        "$1.00",
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CentsToString(tt.scaledCents, tt.centScale)
			if (err != nil) != tt.wantErr {
				t.Errorf("CentsToString() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("CentsToString() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUnitsToCents(t *testing.T) {
	tests := []struct {
		name      string
		units     uint64
		unitScale uint64
		want      uint64
		wantErr   bool
	}{
		{
			name:      "exact conversion",
			units:     158000000,
			unitScale: 1000000,
			want:      158,
			wantErr:   false,
		},
		{
			name:      "floor fractional cents",
			units:     158096090,
			unitScale: 1000000,
			want:      158,
			wantErr:   false,
		},
		{
			name:      "zero scale",
			units:     158000000,
			unitScale: 0,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := UnitsToCents(tt.units, tt.unitScale)
			if (err != nil) != tt.wantErr {
				t.Errorf("UnitsToCents() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("UnitsToCents() = %v, want %v", got, tt.want)
			}
		})
	}
}
