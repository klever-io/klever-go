package data

import (
	"fmt"
	"testing"

	"github.com/klever-io/klever-go/tools/marshal"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"
)

func TestAuthMessage_MarshalUnmarshalShouldWork(t *testing.T) {
	llw := generateAuthMessage()

	for marshName, marsh := range marshal.MarshalizersAvailableForTesting {
		testMarshalUnmarshal(t, marshName, marsh, llw)
	}
}

func generateAuthMessage() *AuthMessage {
	return &AuthMessage{
		AuthMessagePb: &AuthMessagePb{
			Message:   []byte("test message"),
			Sig:       []byte("sig"),
			Pubkey:    []byte("pubkey"),
			Timestamp: 11223344,
		},
	}
}

func testMarshalUnmarshal(t *testing.T, marshName string, marsh marshal.Marshalizer, am *AuthMessage) {
	objCopyForAssert := am

	buff, err := marsh.Marshal(am)
	assert.Nil(t, err)

	objRecovered := &AuthMessage{
		&AuthMessagePb{},
	}
	err = marsh.Unmarshal(objRecovered, buff)
	objCopyForAssert.sizeCache = 0 //TODO: Check other ways to equalize the structures
	assert.Nil(t, err)
	assert.True(t, proto.Equal(objCopyForAssert, objRecovered), fmt.Sprintf("for marshalizer %v", marshName))
}
