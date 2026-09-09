// Package event holds this service's own event bodies.
//
// It exists for one narrow reason: skan needs to record *how* a scan's position was
// obtained, and the shared body in github.com/nathejk/shared-go has nowhere to put
// that. Rather than edit another module from here, the field is added as an additive
// JSON property alongside the shared fields.
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
}

// Position is a scan's location as a handler knows it.
type Position struct {
	Latitude  string
	Longitude string

	// Manual is true when the scanner placed a marker on a map themselves.
	Manual bool
}

// Source renders the position's provenance for the event body.
func (p Position) Source() string {
	if p.Manual {
		return LocationSourceManual
	}
	return LocationSourceGPS
}
