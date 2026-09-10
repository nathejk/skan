package event

import "github.com/nathejk/shared-go/messages"

// SeniorUpdated is the shared senior body with `teamId` restored.
//
// # Why this is here
//
// shared-go's NathejkSeniorUpdated carried a TeamID until
// v0.0.0-20260907212133-4445c9538f2e, which dropped it. The field is still on the wire:
// 1429 of 1430 senior rows in the live stream have a non-empty teamId, so publishers
// never stopped sending it — only the Go struct stopped describing it.
//
// The reason it looks droppable is that only the *first* event for a senior carries it,
// that one acting as "senior added", and a senior may not change klan afterwards. But
// "the value never changes" is an argument for sending it once, not for a consumer being
// unable to read it: skan learns a senior's klan from that first event and from nowhere
// else.
//
// skan needs it. A senior's team is their klan, and the klan is what gives a bandit a
// LOK label in the /geo export; without it every bandit's position loses the group it
// belongs to. Decoding into the shared struct would silently discard a value that is
// present in the JSON, which is the worst of the available failures — no error, just
// emptier data every replay.
//
// This should be **restored upstream** rather than carried here indefinitely. It is
// declared as an addition rather than a copy so that everything else about the body
// stays defined in shared-go.
type SeniorUpdated struct {
	messages.NathejkSeniorUpdated

	// TeamID is the senior's klan.
	//
	// Only the **first** senior.updated for a member carries it — that event is
	// effectively "senior added" — because a senior may not change klan, so later events
	// omit a value that cannot have changed. Consumers must therefore treat an empty
	// TeamID as "not stated here" and keep what they already know, never as "no klan".
	TeamID string `json:"teamId"`
}
