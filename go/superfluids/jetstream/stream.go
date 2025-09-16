package jetstream

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/pkg/errors"
	"nathejk.dk/superfluids/streaminterface"
)

var (
	_ streaminterface.Stream = &stream{}
	//_ StreamStatistics             = &stream{}
	_ streaminterface.Subscription = &subscription{}
)

type stream struct {
	ctx context.Context
	nc  *nats.Conn
	js  jetstream.JetStream
}

// https://github.com/nats-io/nats.go/blob/main/jetstream/README.md
func New(url string) (*stream, error) {
	s := stream{
		ctx: context.Background(),
	}

	//url := os.Getenv("NATS_URL")
	if url == "" {
		url = nats.DefaultURL
	}
	var err error
	s.nc, err = nats.Connect(url)
	if err != nil {
		return nil, err
	}
	//	defer jsc.nc.Drain()

	s.js, err = jetstream.New(s.nc)
	if err != nil {
		return nil, err
	}
	/*
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		_ = cancel
		//defer cancel()
		ss, e := s.js.CreateStream(ctx, jetstream.StreamConfig{
			Name:     "NATHEJK",
			Subjects: []string{"NATHEJK.>"},
		})
		if e != nil {
			log.Printf("s=%q, e=%q", ss, e)
			return nil, e
		}
		/*
			_, er := s.js.Publish(ctx, "NATHEJK.new", []byte("hello message 12"))
			if er != nil {
				log.Printf("XXXXXXX e=%q", er)
				return nil, er
			}*/

	log.Printf("Connected to JetStream %q, Stream created 'NATHEJK'", url)
	return &s, nil
}

func (s *stream) MessageFunc() streaminterface.MessageFunc {
	return func(subject streaminterface.Subject) streaminterface.MutableMessage {
		m := NewMessage()
		m.SetSubject(subject)
		return m
	}
}

type Identifiable interface {
	EventID() EventID
	CorrelationID() EventID
	CausationID() EventID
}

func (s *stream) Publish(m streaminterface.Message) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	subject := fmt.Sprintf("%s.%s", strings.ToUpper(m.Subject().Domain()), m.Subject().Type())
	ID, ok := m.(Identifiable)
	if !ok {
		return errors.New("Message does not implement 'Identifiable' interface")
	}
	buf, err := json.Marshal(jetstreamMessage{
		EventID:       ID.EventID(),
		CorrelationID: ID.CorrelationID(),
		CausationID:   ID.CausationID(),
		Version:       1,
		Time:          m.Time(),
		Body:          m.RawBody().(json.RawMessage),
		Meta:          m.RawMeta().(json.RawMessage),
	})
	if err != nil {
		return errors.Wrap(err, "encode message")
	}

	ack, err := s.js.Publish(ctx, subject, buf)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("publish message to %q", subject))
	}
	log.Printf("Published message %#v", ack)
	return nil
}

func (s *stream) LastMessage(subject streaminterface.Subject) (*message, error) {
	consumer, err := s.js.OrderedConsumer(s.ctx, "NATHEJK", jetstream.OrderedConsumerConfig{
		FilterSubjects: []string{subject.Subject()},
		DeliverPolicy:  jetstream.DeliverLastPolicy,
	})
	if err != nil {
		return nil, err
	}
	msgs, err := consumer.FetchNoWait(1)
	if err != nil {
		return nil, err
	}
	for msg := range msgs.Messages() {
		return createMessage(msg)
	}
	if msgs.Error() != nil {
		return nil, msgs.Error()
	}
	return nil, fmt.Errorf("no messages found with subject %q", subject.Subject())
}

func (s *stream) Subscribe(subjects []streaminterface.Subject, h streaminterface.MessageHandler) (streaminterface.Subscription, error) {
	domains := map[string][]string{}
	for _, subject := range subjects {
		domains[subject.Domain()] = append(domains[subject.Domain()], subject.Domain()+"."+subject.Type())
	}
	ccs := consumeContexts{}
	//spew.Dump(domains)
	for stream, fs := range domains {
		consumer, err := s.js.OrderedConsumer(s.ctx, stream, jetstream.OrderedConsumerConfig{
			// Filter results from "ORDERS" stream by specific subject
			FilterSubjects: fs,
		})
		if err != nil {
			return nil, err
		}
		contxt, err := consumer.Consume(func(msg jetstream.Msg) {
			m, err := createMessage(msg)
			if err != nil {
				log.Println(err)
				return
			}
			err = h.HandleMessage(m)
			if err != nil {
				log.Printf("Error consuming handling %q", err)
				return
			}
			//fmt.Printf("Received a JetStream message: %s\n", string(msg.Data()))
		})
		if err != nil {
			s.Close()
			return nil, err
		}
		ccs = append(ccs, contxt)
		//	s.consumers = append(s.consumers, consumer)
		//log.Printf("Subsribed to jetstream %#v", subjects)
	}
	return ccs, nil
	/*
		return s.js.OrderedConsumer(ctx, "ORDERS", jetstream.OrderedConsumerConfig{
			// Filter results from "ORDERS" stream by specific subject
			FilterSubjects: []string{"ORDERS.A"},
		})
	*/
}
func createMessage(msg jetstream.Msg) (*message, error) {
	var data jetstreamMessage
	if err := json.Unmarshal(msg.Data(), &data); err != nil {
		return nil, fmt.Errorf("error unmarshaling message with subject %q: %w", msg.Subject(), err)
	}
	meta, err := msg.Metadata()
	if err != nil {
		return nil, fmt.Errorf("error getting metadata from message with subject %q: %w", msg.Subject(), err)
	}
	m := &message{
		sequence:      meta.Sequence.Stream,
		eventID:       data.EventID,
		correlationID: data.CorrelationID,
		causationID:   data.CausationID,
		version:       data.Version,
		time:          data.Time,
		subject:       streaminterface.SubjectFromStr(msg.Subject()),
		body:          data.Body,
		meta:          data.Meta,
	}
	return m, nil
}
func (s *stream) Create(name string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	stream, err := s.js.CreateStream(ctx, jetstream.StreamConfig{
		Name:     name,
		Subjects: []string{fmt.Sprintf("%s.>", name)},
	})
	if err != nil {
		return err
	}
	_ = stream
	return nil
	/*
	   js.Publish(ctx, "events.1", nil)
	   js.Publish(ctx, "events.2", nil)
	   js.Publish(ctx, "events.3", nil)

	   cons, _ := stream.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{})

	   wg := sync.WaitGroup{}
	   wg.Add(3)

	   	cc, _ := cons.Consume(func(msg jetstream.Msg) {
	   		msg.Ack()
	   		fmt.Println("received msg on", msg.Subject())
	   		wg.Done()
	   	})

	   wg.Wait()

	   cc.Stop()

	   js.Publish(ctx, "events.1", nil)
	   js.Publish(ctx, "events.2", nil)
	   js.Publish(ctx, "events.3", nil)

	   msgs, _ := cons.Fetch(2)
	   var i int
	   for msg := range msgs.Messages() {

	   		msg.Ack()
	   		i++
	   	}

	   fmt.Printf("got %d messages\n", i)

	   msgs, _ = cons.FetchNoWait(100)
	   i = 0

	   	for msg := range msgs.Messages() {
	   		msg.Ack()
	   		i++
	   	}

	   fmt.Printf("got %d messages\n", i)

	   fetchStart := time.Now()
	   msgs, _ = cons.Fetch(1, jetstream.FetchMaxWait(time.Second))
	   i = 0

	   	for msg := range msgs.Messages() {
	   		msg.Ack()
	   		i++
	   	}

	   fmt.Printf("got %d messages in %v\n", i, time.Since(fetchStart))

	   	dur, _ := stream.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{
	   		Durable: "processor",
	   	})

	   msgs, _ = dur.Fetch(1)
	   msg := <-msgs.Messages()
	   fmt.Printf("received %q from durable consumer\n", msg.Subject())

	   stream.DeleteConsumer(ctx, "processor")

	   _, err := stream.Consumer(ctx, "processor")

	   fmt.Println("consumer deleted:", errors.Is(err, jetstream.ErrConsumerNotFound))
	*/
}
func (s *stream) Close() error {
	s.nc.Drain()
	return nil
}
