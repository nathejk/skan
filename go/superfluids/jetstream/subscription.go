package jetstream

import (
	"github.com/nats-io/nats.go/jetstream"
)

type consumeContexts []jetstream.ConsumeContext

func (cc consumeContexts) Close() error {
	return nil
}

type subscription struct {
	// consumers []jetstream.Consumer
	// handler   streaminterface.MessageHandler
}

/*
	func (s *subscription) Sub(subjects []streaminterface.Subject, h streaminterface.MessageHandler) (streaminterface.Subscription, error) {
		ctx := context.Background()
		domains := map[string][]string{}
		for _, subject := range subjects {
			domains[subject.Domain()] = append(domains[subject.Domain()], subject.Type())
		}
		for stream, fs := range domains {
			consumer, err := s.js.OrderedConsumer(ctx, stream, jetstream.OrderedConsumerConfig{
				// Filter results from "ORDERS" stream by specific subject
				FilterSubjects: fs,
			})
			if err != nil {
				return nil, err
			}
			contxt, err := nsumer.Consume(func(msg jetstream.Msg) {
				fmt.Printf("Received a JetStream message: %s\n", string(msg.Data()))
			})
			if err != nil {
				s.Close()
				return nil, err
			}

			s.consumers = append(s.consumers, consumer)
		}
	}
*/
func (s *subscription) Close() error {
	return nil
}
