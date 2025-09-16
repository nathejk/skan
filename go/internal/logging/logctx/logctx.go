package logctx

import (
	"context"
	"log/slog"
)

type contextKey struct{}

// Attributes is a list of slog.Attr
type Attributes []slog.Attr

// NewContextHandler returns a Handler that adds attributes
// from the context to each log record
func NewContextHandler(handler slog.Handler) *ContextHandler {
	return &ContextHandler{handler}
}

// ContextHandler adds attributes from the context
type ContextHandler struct {
	slog.Handler
}

// Handle adds attributes from the context
func (h *ContextHandler) Handle(ctx context.Context, r slog.Record) error {
	attr, ok := ctx.Value(contextKey{}).(Attributes)
	if ok {
		r.AddAttrs(attr...)
	}
	return h.Handler.Handle(ctx, r)
}

// Enabled reports whether the handler handles records at the given level.
// The handler ignores records whose level is lower.
func (h *ContextHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.Handler.Enabled(ctx, level)
}

// WithAttrs returns a new [ContextHandler] whose attributes consists
// of h's attributes followed by attrs.
func (h *ContextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &ContextHandler{Handler: h.Handler.WithAttrs(attrs)}
}

func (h *ContextHandler) WithGroup(name string) slog.Handler {
	return &ContextHandler{Handler: h.Handler.WithGroup(name)}
}

// WithAttr adds an attribute to the context
func WithAttr(ctx context.Context, a ...slog.Attr) context.Context {
	attrs, ok := ctx.Value(contextKey{}).(Attributes)
	if !ok {
		attrs = Attributes{}
	}

	for _, attr := range a {
		attrs = appendAttr(attrs, attr)
	}

	return context.WithValue(ctx, contextKey{}, attrs)
}

// appendAttr adds an attribute to the list if it is not already present.
func appendAttr(attrs Attributes, a slog.Attr) Attributes {
	for _, attr := range attrs {
		if attr.Equal(a) {
			return attrs
		}
	}

	attrs = append(attrs, a)
	return attrs
}

// Error creates a slog.Attr for an error that can then be passed into a log line as needed.
func Error(err error) slog.Attr {
	return slog.String("error", err.Error())
}
