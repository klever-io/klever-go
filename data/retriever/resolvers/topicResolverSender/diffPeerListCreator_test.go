package topicResolverSender_test

import (
	"errors"
	"testing"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/data/retriever/resolvers/topicResolverSender"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/stretchr/testify/assert"
)

const mainTopic = "mainTopic"
const intraTopic = "intraTopic"
const emptyTopic = ""

func TestNewPeerListCreator_NilMessengerShouldErr(t *testing.T) {
	t.Parallel()

	dplc, err := topicResolverSender.NewPeerListCreator(
		nil,
		mainTopic,
		intraTopic,
	)

	assert.True(t, check.IfNil(dplc))
	assert.Equal(t, common.ErrNilMessenger, err)
}

func TestNewPeerListCreator_EmptyMainTopicShouldErr(t *testing.T) {
	t.Parallel()

	dplc, err := topicResolverSender.NewPeerListCreator(
		&mock.MessageHandlerStub{},
		emptyTopic,
		intraTopic,
	)

	assert.True(t, check.IfNil(dplc))
	assert.True(t, errors.Is(err, common.ErrEmptyString))
}

func TestNewPeerListCreator_EmptyIntraTopicShouldErr(t *testing.T) {
	t.Parallel()

	dplc, err := topicResolverSender.NewPeerListCreator(
		&mock.MessageHandlerStub{},
		mainTopic,
		emptyTopic,
	)

	assert.True(t, check.IfNil(dplc))
	assert.True(t, errors.Is(err, common.ErrEmptyString))
}

func TestNewPeerListCreator_ShouldWork(t *testing.T) {
	t.Parallel()

	dplc, err := topicResolverSender.NewPeerListCreator(
		&mock.MessageHandlerStub{},
		mainTopic,
		intraTopic,
	)

	assert.Nil(t, err)
	assert.False(t, check.IfNil(dplc))
	assert.Equal(t, mainTopic, dplc.MainTopic())
}

//------- PeersList

func TestDiffPeerListCreator_PeersListEmptyMainListShouldRetEmpty(t *testing.T) {
	t.Parallel()

	dplc, _ := topicResolverSender.NewPeerListCreator(
		&mock.MessageHandlerStub{
			ConnectedPeersOnTopicCalled: func(topic string) []core.PeerID {
				return make([]core.PeerID, 0)
			},
		},
		mainTopic,
		intraTopic,
	)

	assert.Empty(t, dplc.PeerList())
}

func TestDiffPeerListCreator_PeersListNoExcludedTopicSetShouldRetPeersOnMain(t *testing.T) {
	t.Parallel()

	pID1 := core.PeerID("peer1")
	pID2 := core.PeerID("peer2")
	peersOnMain := []core.PeerID{pID1, pID2}
	dplc, _ := topicResolverSender.NewPeerListCreator(
		&mock.MessageHandlerStub{
			ConnectedPeersOnTopicCalled: func(topic string) []core.PeerID {
				return peersOnMain
			},
		},
		mainTopic,
		intraTopic,
	)

	assert.Equal(t, peersOnMain, dplc.PeerList())
}

func TestDiffPeerListCreator_IntraShardPeersList(t *testing.T) {
	t.Parallel()

	peerList := []core.PeerID{"pid1", "pid2"}
	dplc, _ := topicResolverSender.NewPeerListCreator(
		&mock.MessageHandlerStub{
			ConnectedPeersOnTopicCalled: func(topic string) []core.PeerID {
				if topic == intraTopic {
					return peerList
				}

				return nil
			},
		},
		mainTopic,
		intraTopic,
	)

	assert.Equal(t, peerList, dplc.ConsensusPeerList())
}
