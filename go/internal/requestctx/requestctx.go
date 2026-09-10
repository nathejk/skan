// Package requestctx carries the acting user on a request's context.
//
// # Why it exists in skan
//
// It is hq's package, and skan needs it only because the copied `checkpoint` and
// `checkpersonnel` projections import it: their command sides stamp
// `messages.Metadata{UserID: u.ID}` so an event records who caused it. skan never calls
// those commands — both projections are constructed with a nil publisher, because hq plans
// the posts and the rosters — but the import has to resolve for the packages to compile.
//
// So this is deliberately minimal: enough to satisfy the contract those files use, and no
// more. If skan ever does publish on behalf of a user, this is the seam to fill in properly.
//
// # Why it does not reuse internal/login.User
//
// It would create an import cycle. `internal/login` depends on `internal/data`, which
// depends on the projection packages, which depend on this one. Keeping this package free of
// skan-specific types also matches how the projections are written: they name a user by id
// and nothing else.
package requestctx

import (
	"context"

	"github.com/nathejk/shared-go/types"
)

// User is the acting user, as a projection's command side needs them: an id to stamp on an
// event's metadata.
type User struct {
	ID types.UserID
}

// contextKey is unexported so no other package can collide with this key, which is the
// reason not to use a bare string.
type contextKey struct{}

var userKey = contextKey{}

// WithUser returns a context carrying the acting user.
func WithUser(ctx context.Context, u User) context.Context {
	return context.WithValue(ctx, userKey, u)
}

// UserFrom returns the acting user, and whether there was one.
//
// The boolean is not decoration: the callers treat its absence as an error and refuse to
// publish, which is the right behaviour — an event that cannot say who caused it is worse
// than no event.
func UserFrom(ctx context.Context) (User, bool) {
	u, ok := ctx.Value(userKey).(User)
	return u, ok
}
