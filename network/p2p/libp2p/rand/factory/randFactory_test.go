package factory_test

import (
	"crypto/rand"
	"reflect"
	"testing"

	rand2 "github.com/klever-io/klever-go/network/p2p/libp2p/rand"
	"github.com/klever-io/klever-go/network/p2p/libp2p/rand/factory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRandFactory_EmptySeedShouldReturnCryptoRand(t *testing.T) {
	t.Parallel()

	r, err := factory.NewRandFactory("")

	assert.Nil(t, err)
	assert.True(t, r == rand.Reader)
}

func TestNewRandFactory_NotEmptySeedShouldSeedRandReader(t *testing.T) {
	t.Parallel()

	seed := "seed"
	srrExpected, err := rand2.NewSeedRandReader([]byte(seed))
	require.NoError(t, err)

	r, err := factory.NewRandFactory(seed)
	require.NoError(t, err)

	assert.Equal(t, reflect.TypeOf(r), reflect.TypeOf(srrExpected))
}

func TestNewLegacyRandFactory_EmptySeedShouldReturnCryptoRand(t *testing.T) {
	t.Parallel()

	r, err := factory.NewLegacyRandFactory("")

	assert.Nil(t, err)
	assert.True(t, r == rand.Reader)
}

func TestNewLegacyRandFactory_NotEmptySeedShouldReturnLegacySeedRandReader(t *testing.T) {
	t.Parallel()

	seed := "seed"
	srrExpected, err := rand2.NewLegacySeedRandReader([]byte(seed))
	require.NoError(t, err)

	r, err := factory.NewLegacyRandFactory(seed)
	require.NoError(t, err)

	assert.Equal(t, reflect.TypeOf(r), reflect.TypeOf(srrExpected))
}
