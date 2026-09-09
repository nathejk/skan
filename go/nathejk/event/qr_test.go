package event

import (
	"encoding/json"
	"testing"

	"github.com/nathejk/shared-go/messages"
)

// TestQrRegisteredCarriesMapID is the contract: the sheet handed over travels with the
// registration event, alongside the shared fields rather than replacing them.
func TestQrRegisteredCarriesMapID(t *testing.T) {
	body := QrRegistered{MapID: "kort-42"}
	body.QrID = "7"
	body.TeamID = "team-1"
	body.TeamNumber = "42"
	body.ScannerID = "scanner-1"

	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshalling: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshalling: %v", err)
	}

	if got["mapId"] != "kort-42" {
		t.Fatalf("mapId = %v, want kort-42\n%s", got["mapId"], raw)
	}
	// The embedded fields must still be at the top level, not nested under a struct
	// name, or every existing consumer breaks.
	for key, want := range map[string]any{
		"qrId":       "7",
		"teamId":     "team-1",
		"teamNumber": "42",
		"scannerId":  "scanner-1",
	} {
		if got[key] != want {
			t.Errorf("%s = %v, want %v", key, got[key], want)
		}
	}

	// A consumer that only knows the shared body must still decode it.
	var shared messages.NathejkQrRegistered
	if err := json.Unmarshal(raw, &shared); err != nil {
		t.Fatalf("shared body cannot decode our event: %v", err)
	}
	if shared.QrID != "7" || shared.TeamNumber != "42" {
		t.Fatalf("shared decode lost fields: %+v", shared)
	}
}

// TestQrRegisteredOmitsUnknownMap keeps "we do not know which sheet" distinguishable from
// a sheet whose id is the empty string.
func TestQrRegisteredOmitsUnknownMap(t *testing.T) {
	raw, err := json.Marshal(QrRegistered{})
	if err != nil {
		t.Fatalf("marshalling: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshalling: %v", err)
	}
	if _, present := got["mapId"]; present {
		t.Fatalf("mapId should be omitted when unset: %s", raw)
	}
}

// TestSeniorUpdatedRestoresTeamID guards the field shared-go dropped while publishers
// kept sending it. Without this the klan link — and so a bandit's LOK label — silently
// empties on every replay.
func TestSeniorUpdatedRestoresTeamID(t *testing.T) {
	const raw = `{"memberId":"m1","teamId":"klan-9","name":"Ada","phone":"12345678"}`

	var body SeniorUpdated
	if err := json.Unmarshal([]byte(raw), &body); err != nil {
		t.Fatalf("unmarshalling: %v", err)
	}
	if body.TeamID != "klan-9" {
		t.Fatalf("TeamID = %q, want klan-9", body.TeamID)
	}
	if string(body.MemberID) != "m1" || body.Name != "Ada" {
		t.Fatalf("shared fields lost: %+v", body)
	}
}
