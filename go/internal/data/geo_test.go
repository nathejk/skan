package data

import (
	"database/sql"
	"testing"
)

// ns is a present value; absent is a NULL, i.e. "the LEFT JOIN found no row".
func ns(v string) sql.NullString { return sql.NullString{String: v, Valid: true} }

var absent = sql.NullString{}

// TestResolveScanner pins the identity precedence the /geo export has always used.
//
// The distinction that matters throughout is **presence vs emptiness**: a row that exists
// with a blank value is not the same as no row, and the old per-scan lookups branched on
// `!= nil`. Getting this wrong would silently relabel scans on the race map.
func TestResolveScanner(t *testing.T) {
	tests := []struct {
		name                                              string
		seniorName, klanLok, personName, personAdditional sql.NullString
		wantScanner, wantRole, wantLok                    string
	}{
		{
			name:        "senior with a klan is a bandit at a lok",
			seniorName:  ns("Kasper"),
			klanLok:     ns("4"),
			personName:  absent,
			wantScanner: "Kasper",
			wantRole:    "Bandit",
			wantLok:     "LOK 4",
		},
		{
			// A klan row that exists with a blank lok still yields "LOK ", exactly as the
			// per-scan version did. Deliberate: the formatting keys off the row existing.
			name:        "senior whose klan has a blank lok",
			seniorName:  ns("Kasper"),
			klanLok:     ns(""),
			personName:  absent,
			wantScanner: "Kasper",
			wantRole:    "Bandit",
			wantLok:     "LOK ",
		},
		{
			name:        "senior with no klan row has no lok",
			seniorName:  ns("Kasper"),
			klanLok:     absent,
			personName:  absent,
			wantScanner: "Kasper",
			wantRole:    "Bandit",
			wantLok:     "",
		},
		{
			// Crew: the department in additionals becomes the role.
			name:             "personnel with a department",
			seniorName:       absent,
			klanLok:          absent,
			personName:       ns("Jacob"),
			personAdditional: ns(`{"department":"Andet"}`),
			wantScanner:      "Jacob",
			wantRole:         "Andet",
		},
		{
			name:             "personnel without a department has no role",
			seniorName:       absent,
			klanLok:          absent,
			personName:       ns("Jacob"),
			personAdditional: ns(`{"tshirt":"L"}`),
			wantScanner:      "Jacob",
			wantRole:         "",
		},
		{
			// Someone in both projections is a data error, but the export has always
			// preferred the personnel name, and the lok from the klan survives.
			name:             "personnel overrides the senior name and role",
			seniorName:       ns("Kasper"),
			klanLok:          ns("4"),
			personName:       ns("Jacob"),
			personAdditional: ns(`{"department":"Andet"}`),
			wantScanner:      "Jacob",
			wantRole:         "Andet",
			wantLok:          "LOK 4",
		},
		{
			// A personnel row with no department must not clear the Bandit role: the old
			// code only assigned role when the department parsed.
			name:             "personnel without a department keeps the bandit role",
			seniorName:       ns("Kasper"),
			klanLok:          absent,
			personName:       ns("Jacob"),
			personAdditional: ns(`{}`),
			wantScanner:      "Jacob",
			wantRole:         "Bandit",
		},
		{
			// An unknown scanner still yields a row: the scan is a fact on its own.
			name:        "scanner in neither projection",
			seniorName:  absent,
			klanLok:     absent,
			personName:  absent,
			wantScanner: "",
			wantRole:    "",
			wantLok:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scanner, role, lok := resolveScanner(tt.seniorName, tt.klanLok, tt.personName, tt.personAdditional)
			if scanner != tt.wantScanner {
				t.Errorf("scanner = %q, want %q", scanner, tt.wantScanner)
			}
			if role != tt.wantRole {
				t.Errorf("role = %q, want %q", role, tt.wantRole)
			}
			if lok != tt.wantLok {
				t.Errorf("lok = %q, want %q", lok, tt.wantLok)
			}
		})
	}
}

// TestDepartmentOf: additionals holds whatever the signup form collected, so anything
// unparseable is ordinary and must not fail the export.
func TestDepartmentOf(t *testing.T) {
	tests := []struct {
		name, additionals, want string
		wantOK                  bool
	}{
		{name: "a department", additionals: `{"department":"Andet"}`, want: "Andet", wantOK: true},
		{name: "empty string", additionals: ``, wantOK: false},
		{name: "empty object", additionals: `{}`, wantOK: false},
		{name: "not json", additionals: `department=Andet`, wantOK: false},
		{name: "json but not an object", additionals: `["Andet"]`, wantOK: false},
		// A non-string value must not be coerced into a role.
		{name: "department is not a string", additionals: `{"department":7}`, wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := departmentOf(tt.additionals)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}
