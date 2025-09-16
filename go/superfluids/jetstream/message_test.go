package jetstream_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"nathejk.dk/superfluids/jetstream"
)

type mytype struct {
	Data string
}

type mytypetwo struct {
	Data string
}

func TestMessageStruct(t *testing.T) {
	type Inner struct {
		Value int
	}

	type embedded struct {
		Inner
		Explicit *Inner
	}

	exp := embedded{
		Inner:    Inner{Value: 1},
		Explicit: &Inner{Value: 2},
	}

	m := jetstream.NewMessage()
	m.SetBody(&exp)
	var got embedded
	if err := m.Body(&got); err != nil {
		t.Fatal(err)
	}
	if got.Value != exp.Value || got.Explicit.Value != exp.Explicit.Value {
		t.Fatalf("exp %v, got %v\n", exp, got)
	}
}

func TestMessageTypeChange(t *testing.T) {
	exp := mytype{Data: "Hello"}
	m := jetstream.NewMessage()
	m.SetBody(&exp)
	var got mytypetwo
	if err := m.Body(&got); err != nil {
		t.Fatal(err)
	}
	if got.Data != exp.Data {
		t.Fatalf("exp %s, got %s\n", exp.Data, got.Data)
	}
}

func BenchmarkMessageTypeAssert(b *testing.B) {
	exp := mytype{Data: "Hello"}

	for i := 0; i < b.N; i++ {
		m := jetstream.NewMessage()
		m.SetBody(&exp)

		var got mytype
		if err := m.Body(&got); err != nil {
			b.Fatal(err)
		}

		if got.Data != exp.Data {
			b.Fatalf("exp %s, got %s\n", exp.Data, got.Data)
		}
	}
}

func TestEventMessage(t *testing.T) {
	assert := assert.New(t)

	msg := jetstream.NewMessage()
	assert.NotEqual("", msg.EventID(), "Event id should not be empty")
	assert.Equal(msg.EventID(), msg.CausationID(), "Causation id should match event id")
	assert.Equal(msg.EventID(), msg.CorrelationID(), "Correlation id should match event id")

	var err error

	type TestBody struct {
		Hello string `json:"hello"`
	}
	body := TestBody{"world"}
	msg.SetBody(&body)

	type TestMeta struct {
		Goodbye string `json:"goodbye"`
	}
	meta := TestMeta{"universe"}
	err = msg.SetMeta(meta)
	assert.Nil(err)
	assert.Equal("{\"goodbye\":\"universe\"}", string(msg.RawMeta().(json.RawMessage)))

	err = msg.SetMeta("WORLD")
	assert.Nil(err)
	assert.Equal("\"WORLD\"", string(msg.RawMeta().(json.RawMessage)))
}

func TestMessageValidBodyAndMeta(t *testing.T) {
	assert := assert.New(t)

	type TestBody struct {
		Hello string `json:"hello"`
	}
	type TestMeta struct {
		Goodbye string `json:"goodbye"`
	}

	msg := jetstream.NewMessage()
	msg.SetBody(&TestBody{Hello: "world"})
	msg.SetMeta(&TestMeta{Goodbye: "universe"})

	var body TestBody
	if err := msg.Body(&body); err != nil {
		t.Fatal(err)
	}
	assert.Equal(TestBody{Hello: "world"}, body)

	var meta TestMeta
	msg.Meta(&meta)
	assert.Equal(TestMeta{Goodbye: "universe"}, meta)
}

func TestMessageInvalidBodyAndMeta(t *testing.T) {
	assert := assert.New(t)
	msg := jetstream.NewMessage()

	err := msg.SetMeta(make(chan int))
	assert.NotNil(err)
}

/*
func TestMessageEncoding(t *testing.T) {
	assert := assert.New(t)

	exp := `{
	"type": "",
	"eventId": "event-54a839b5-805c-4421-ac0c-9925f2dd5e78",
	"correlationId": "event-54a839b5-805c-4421-ac0c-9925f2dd5e78",
	"causationId": "event-54a839b5-805c-4421-ac0c-9925f2dd5e78",
	"datetime": "2019-10-31T07:26:18.6200067Z",
	"body": {
		"hello": "world"
	},
	"meta": {
		"goodbye": "universe"
	}
}`
	rm := jetstream.NewMessage()
	err := rm.DecodeData([]byte(exp))
	assert.Nil(err)

	got, err := json.Marshal(rm)
	assert.Nil(err)

	assert.JSONEq(exp, string(got))
}*/
