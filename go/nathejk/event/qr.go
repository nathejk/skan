// Package event holds this service's own event bodies.
//
// It exists for one narrow reason: skan needs to record *how* a scan's position was
// obtained and *how good* it is, and the shared body in
// github.com/nathejk/shared-go has nowhere to put either. Rather than edit another
// module from here, the fields are added as additive JSON properties alongside the
// shared ones.
//
// Additive is safe in both directions. A consumer decoding into
// messages.NathejkQrScanned ignores the extra property, and skan decoding an older
// event simply sees an empty source. When the field is upstreamed into shared-go, this
// package should collapse into it.
package event

import "github.com/nathejk/shared-go/messages"

// How a scan's position was arrived at.
const (
	// LocationSourceGPS is a position the browser's geolocation API supplied.
	LocationSourceGPS = "gps"

	// LocationSourceManual is a position the scanner placed on a map by hand, because
	// the browser would not or could not give one. Worth distinguishing: a hand-placed
	// marker is only as good as the scanner's sense of where they are, and anything
	// reading these positions back — a map of the race, or a dispute about a catch —
	// should be able to tell the two apart.
	LocationSourceManual = "manual"
)

// NormalizeSource maps a claimed source onto the known set, or "" if it is neither.
//
// The value arrives from the browser, so it is an assertion rather than a fact and must
// not be written through unchecked. "" means "unknown", which is also what scans
// recorded before this field existed carry — and unknown must never be presented as a
// GPS fix.
func NormalizeSource(s string) string {
	switch s {
	case LocationSourceGPS, LocationSourceManual:
		return s
	default:
		return ""
	}
}

// QrScanned is the shared scanned-event body plus the position's provenance.
//
// The shared struct is embedded rather than copied, so its fields stay defined in one
// place and a change upstream cannot silently diverge from what this service publishes.
type QrScanned struct {
	messages.NathejkQrScanned

	// LocationSource is "gps" or "manual". Empty on events published before this field
	// existed, which is why it is omitempty: absent and "unknown" are the same thing,
	// and neither should be reported as GPS.
	LocationSource string `json:"locationSource,omitempty"`

	// LocationAccuracy is the radius of confidence in metres, as the browser reported
	// it. Empty when unknown — which includes every hand-placed marker, and every scan
	// recorded before this field existed.
	LocationAccuracy string `json:"locationAccuracy,omitempty"`
}

// Position is a scan's location as a handler knows it.
type Position struct {
	Latitude  string
	Longitude string

	// Source is how the position was obtained: LocationSourceGPS,
	// LocationSourceManual, or "" when it is not known.
	Source string

	// Accuracy is the radius of confidence in metres, or "" when unknown. A GPS fix in
	// a forest can be hundreds of metres out, so a position without this is a weaker
	// claim than it looks — keep it with the coordinates rather than inferring quality
	// from the source alone.
	Accuracy string
}
