# Stream

Stream defines a publisher—subscriber messaging interface and
related types used to compose data-pipelines.

## How to structure packages

 - stream: defines interfaces for working with event stream.
 - flux/pubsub: a in-memory implementation of a Stream.
 - flux/valuemessage: implementation of messages.
 - nats/v2: a Stream that uses NATS as the persistent storage.
 - db/{mongo,postgres,badger}: stream handlers that consume messages and write to db.

Reading remote messages of NATS or persisting state in a database is
different than the event-passing we do internal in a service. We can think
of it in layers;

      NATS             |
     ----------------- |  Code knows about NATS and Stream. Data abstraction layer
      nats client  |
     ----------------- —
      stream router    |
     ----------------- |  Code knows about stream interfaces
      stream handlers  |
     ----------------- —
      db handlers      |
     ----------------- |  Code knows about MongoDB and Stream. Data abstraction layer
      MongoDB          |

We should decode and validate data when we read from a source. It sets a
clear boundary and expectation; a boundary that leaves transportation
encoding outside business logic and an expectation to the requirements the
to the form of the data.

We should handle business logic when we handle internal messages in stream
handlers. Stream router logic should not know about NATS and message codec
(such as JSON). Messages don't need to be encoded/decoded when handled
internally — all we have to guarantee is memory safety.

When encoding data to save it in a database it's the database handlers job
to encode messages to format that meets the requirements of the DB.

The stream package doesn't know about database packages that implement its
interfaces.

### Composing programs

Internal services are built with data-pipelines. An input (usally from NATS)
is consumed and processed by multiple subscribers. Subscribers transform
data and push their respective output to a shared internal
publisher-subscriber (Stream). Other handlers subscribe to this Stream, and
repeat the process. When the NATS event is published we only know about the
first subscriber that consumes the message, and this subscriber doesn't know
about other consumers of it's produced messages.

    [handler]--||--[handler]--||--[handler]

In-between each handler there is a routing component. This component is
responsible of routing messages from publishers to subscribers.

