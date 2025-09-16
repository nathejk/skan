// package stream defines a publisher—subscriber messaging interface and
// related types used to compose data-pipelines.
package streaminterface

import "time"

// Message contains meta-data and the value for a given event on the
// event-stream. This interface is a read-only interface to an event.
type Message interface {
	// Subject() returns the subject to which this event was published.
	Subject() Subject

	// Time returns the time when this message was submitted to the stream or
	// when the event occurred.
	Time() time.Time

	// Sequence
	Sequence() uint64

	// Body copies the value data onto the type pointed to from dst (dst is
	// expected to be a struct pointer).
	//
	// If the type of the value is a pointer to a struct its content is copied
	// to a new struct of the same type, and dst is pointed to this new value.
	// If the concrete types don't match Value returns an error. However, other
	// mechanisms to store and copy the value may exist - that don't have the
	// same semantics of requiring types to match. This may be if the
	// implementation stores a byte slice, data is is held in a intermediate
	// form, is decoded onto the destination type.
	//
	// dst should be considered a mutable pointer to some T that the Value
	// method borrows for the duration of the call.
	Body(dst interface{}) error

	// Meta is optional meta data about the message. It follows the same
	// semantic as the Body() method.
	Meta(dst interface{}) error

	RawBody() interface{}
	RawMeta() interface{}
}

type MessageFunc func(Subject) MutableMessage

type MutableMessage interface {
	Message
	SetSubject(Subject)
	SetBody(interface{}) error
	SetMeta(interface{}) error
	SetTime(time.Time) error
}

// A Handler receives events of an event stream.
type MessageHandler interface {
	// HandleMessage accepts a Message. With the Message type and channel we can can
	// deterime type of the Message body—and decode the data to its type.
	HandleMessage(Message) error
}

// MessageHandlerFunc is an adapter to allow the use of ordinary functions as Message
// handlers. If f is a function with the appropriate signature, HandlerFunc(f)
// is a Handler that calls f.
type MessageHandlerFunc func(Message) error

// Handle calls f(msg).
func (f MessageHandlerFunc) HandleMessage(msg Message) error {
	return f(msg)
}

// HandlerFunc is a Handler.
var _ MessageHandler = (*MessageHandlerFunc)(nil)

// Subscriber is an interface that wraps the Subscribe method.
type Subscriber interface {
	// Subscribe will subcribe handler for the given subject it's interested
	// in. It returns a Subscription or an error.
	//
	// To unsubscribe simply call Close() on the subscription.
	Subscribe(subjects []Subject, h MessageHandler) (Subscription, error)
}

// Subscription represents interest in a given subject.
type Subscription interface {
	// Close stops unsubscribes subscriber.
	Close() error
}

// Publisher is an interface that wraps the Publish method.
type Publisher interface {
	// Publish publishes the provided message to a given subject.
	//
	// Publish can be async or synchronous.
	// Publish should be thread-safe.
	Publish(msg Message) error

	// MessageFunc returns a MessageFunc used by the Publisher
	MessageFunc() MessageFunc
}

// PublisherFunc type is an adapter to allow the use of ordinary functions as
// Publishers.
type PublisherFunc func(Message) error

// Publish calls f(g)
func (f PublisherFunc) Publish(msg Message) error {
	return f(msg)
}

// satisfy Publisher interface
func (f PublisherFunc) MessageFunc() MessageFunc {
	panic("not implemented")
}

var _ Publisher = (*PublisherFunc)(nil)

// Stream defines a publisher—subscriber messaging interface.
type Stream interface {
	Publisher
	Subscriber

	// Close stops publisher from accepting messages and unsubscribes all
	// subscribers.
	Close() error
}

// CatchupListener is an optional interface that can be useful to implement on
// types that are used in a router context. i.e. a Consumer.
type CatchupListener interface {
	// CaughtUp will be called when the subjects that the handler is subscribed
	// to are read up until the state that the data store had before beginning
	// to read events of the event store. This allows the programmer to
	// implement optimizations that use this knowledge to for example discard
	// duplicate events for a given identifier.
	CaughtUp()
}

// Producer is a meta-data interface that helps creating a graph of
// publisher-subscribers.
type Producer interface {
	// Produces defines a set of channels to which it may write to. it returns
	// a vector of subjects.
	Produces() []Subject
}

// A Consumer represents interest in a set of subjects.
type Consumer interface {
	MessageHandler

	// Consumes returns a vector that of subjects.
	Consumes() []Subject
}
