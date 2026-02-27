package oddsblaze

import (
	"testing"
)

func TestNormalizeName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Kevin Porter Jr.", "KEVIN PORTER JR"},
		{"R.Holland", "RHOLLAND"},
		{"LeBron James", "LEBRON JAMES"},
		{"", ""},
		{"Egor Dëmin", "EGOR DEMIN"},
		{"Nikola Jokić", "NIKOLA JOKIC"},
		{"Luka Dončić", "LUKA DONCIC"},
		{"José Álvarez", "JOSE ALVAREZ"},
	}
	for _, tt := range tests {
		got := normalizeName(tt.input)
		if got != tt.want {
			t.Errorf("normalizeName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestStripSuffix(t *testing.T) {
	tests := []struct {
		input        string
		wantName     string
		wantStripped bool
	}{
		{"KEVIN PORTER JR", "KEVIN PORTER", true},
		{"RONALD HOLLAND II", "RONALD HOLLAND", true},
		{"JAREN JACKSON JR", "JAREN JACKSON", true},
		{"WENDELL CARTER JR", "WENDELL CARTER", true},
		{"LARRY NANCE JR", "LARRY NANCE", true},
		{"GARY TRENT JR", "GARY TRENT", true},
		{"JABARI SMITH JR", "JABARI SMITH", true},
		{"MARCUS MORRIS SR", "MARCUS MORRIS", true},
		{"LEBRON JAMES", "LEBRON JAMES", false},
		{"SHAI GILGEOUS-ALEXANDER", "SHAI GILGEOUS-ALEXANDER", false},
		{"", "", false},
	}
	for _, tt := range tests {
		gotName, gotStripped := stripSuffix(tt.input)
		if gotName != tt.wantName || gotStripped != tt.wantStripped {
			t.Errorf("stripSuffix(%q) = (%q, %v), want (%q, %v)",
				tt.input, gotName, gotStripped, tt.wantName, tt.wantStripped)
		}
	}
}

func TestExtractLastName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"KEVIN PORTER", "PORTER"},
		{"LEBRON JAMES", "JAMES"},
		{"SHAI GILGEOUS-ALEXANDER", "GILGEOUS-ALEXANDER"},
		{"NENE", "NENE"},
		{"", ""},
	}
	for _, tt := range tests {
		got := extractLastName(tt.input)
		if got != tt.want {
			t.Errorf("extractLastName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestNamesMatch(t *testing.T) {
	tests := []struct {
		name       string
		oddsBlaze  string
		db         string
		wantMatch  bool
	}{
		// Exact match
		{"exact match", "LeBron James", "LeBron James", true},
		// Suffix stripping
		{"Jr suffix in DB", "Kevin Porter", "Kevin Porter Jr.", true},
		{"Jr suffix both sides", "Kevin Porter Jr.", "Kevin Porter Jr.", true},
		{"II suffix in DB", "Ronald Holland", "Ronald Holland II", true},
		{"SR suffix in DB", "Marcus Morris", "Marcus Morris Sr.", true},
		// Nickname / first-initial match
		{"Ron vs Ronald same last name", "Ron Holland", "Ronald Holland II", true},
		{"Bobby vs Robert same last name", "Bobby Portis", "Robert Portis", false}, // B != R
		// Case insensitivity
		{"case insensitive", "kevin porter", "KEVIN PORTER JR.", true},
		// Accent stripping
		{"accent in DB name", "Egor Demin", "Egor Dëmin", true},
		{"accent in DB name Jokic", "Nikola Jokic", "Nikola Jokić", true},
		// No match
		{"completely different names", "Kevin Porter", "Myles Turner", false},
		{"same first name different last", "Kevin Durant", "Kevin Porter Jr.", false},
		{"same last name different first initial", "Tim Porter", "Kevin Porter Jr.", false},
		// Edge cases
		{"empty OddsBlaze name", "", "Kevin Porter Jr.", false},
		{"empty DB name", "Kevin Porter", "", false},
		{"both empty", "", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := namesMatch(tt.oddsBlaze, tt.db)
			if got != tt.wantMatch {
				t.Errorf("namesMatch(%q, %q) = %v, want %v",
					tt.oddsBlaze, tt.db, got, tt.wantMatch)
			}
		})
	}
}

func TestJerseyAndLastNameMatch(t *testing.T) {
	tests := []struct {
		name      string
		obName    string
		obJersey  string
		dbName    string
		dbJersey  string
		wantMatch bool
	}{
		// Same jersey + same last name (after suffix strip)
		{"Ron Holland jersey match", "Ron Holland", "5", "Ronald Holland II", "5", true},
		{"Kevin Porter jersey match", "Kevin Porter", "3", "Kevin Porter Jr.", "3", true},
		// Same jersey + different last name → reject
		{"Kevin Porter vs Myles Turner", "Kevin Porter", "3", "Myles Turner", "3", false},
		// Different jersey + same last name → reject
		{"same name different jersey", "Kevin Porter", "3", "Kevin Porter Jr.", "7", false},
		// Empty jersey numbers → reject
		{"empty OB jersey", "Kevin Porter", "", "Kevin Porter Jr.", "3", false},
		{"empty DB jersey", "Kevin Porter", "3", "Kevin Porter Jr.", "", false},
		{"both empty jerseys", "Kevin Porter", "", "Kevin Porter Jr.", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := jerseyAndLastNameMatch(tt.obName, tt.obJersey, tt.dbName, tt.dbJersey)
			if got != tt.wantMatch {
				t.Errorf("jerseyAndLastNameMatch(%q, %q, %q, %q) = %v, want %v",
					tt.obName, tt.obJersey, tt.dbName, tt.dbJersey, got, tt.wantMatch)
			}
		})
	}
}
