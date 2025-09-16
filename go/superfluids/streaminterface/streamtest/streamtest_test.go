package streamtest_test

import (
	"testing"

	"nathejk.dk/superfluids/streaminterface"
	"nathejk.dk/superfluids/streaminterface/streamtest"

	"github.com/stretchr/testify/assert"
)

type testmodel struct {
	result string
}

func (m *testmodel) Consumes() []string { return []string{"channel1:type1", "channel2:type2"} }
func (m *testmodel) HandleMessage(msg streaminterface.Message) error {
	m.result += "+" + msg.Subject().Subject()
	return nil
}

func TestModel(t *testing.T) {
	assert := assert.New(t)

	m := &testmodel{}

	streamtest.SeedModel(m,
		streamtest.StubBody("channel1", "type1", nil),
		streamtest.StubBody("channel2", "type2", nil),
	)

	assert.Equal("+channel1.type1+channel2.type2", m.result)
}
