package systemAccount

import (
	"errors"
	"fmt"
	"math"
	"testing"

	"github.com/klever-io/klever-go/common"
	commonMock "github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/tools/marshal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSFTGetMeta_NilTrieReturnsEmptyMeta(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		err  error
	}{
		{"direct ErrNilTrie", common.ErrNilTrie},
		{"wrapped ErrNilTrie", fmt.Errorf("load: %w", common.ErrNilTrie)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := &systemAccountKApp{}
			require.NoError(t, s.SetAccountsCacher(&commonMock.AccountsCacherStub{
				LoadKAppCalled: func(address []byte) (state.KAppAccountHandler, error) {
					return nil, c.err
				},
			}))

			meta, err := s.SFTGetMeta([]byte("ASSET"), nil)
			require.NoError(t, err)
			require.NotNil(t, meta)
			require.NotNil(t, meta.Metadata)
		})
	}
}

func TestSFTGetMeta_OtherErrorPropagates(t *testing.T) {
	t.Parallel()

	other := errors.New("disk failure")
	s := &systemAccountKApp{}
	require.NoError(t, s.SetAccountsCacher(&commonMock.AccountsCacherStub{
		LoadKAppCalled: func(address []byte) (state.KAppAccountHandler, error) {
			return nil, other
		},
	}))

	meta, err := s.SFTGetMeta([]byte("ASSET"), nil)
	assert.Nil(t, meta)
	require.Error(t, err)
	require.True(t, errors.Is(err, other))
}

func TestSFTGetMetaUncached_ConcurrentWithProcessingWrites(t *testing.T) {
	t.Parallel()

	// block processing mutates the cached system account KApp on SFT/NFT
	// operations; external readers must not race on its dirty-data map
	sharedApp, err := state.NewKAppAccount([]byte("system-account-addr-............"))
	require.NoError(t, err)

	s := &systemAccountKApp{marshalizer: &commonMock.MarshalizerMock{}}
	require.NoError(t, s.SetAccountsCacher(&commonMock.AccountsCacherStub{
		LoadKAppCalled: func(address []byte) (state.KAppAccountHandler, error) {
			return sharedApp, nil
		},
		LoadKAppUncachedCalled: func(address []byte) (state.KAppAccountHandler, error) {
			return state.NewKAppAccount(address)
		},
	}))

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 500; i++ {
			_ = sharedApp.SetStorage([]byte(fmt.Sprintf("key-%d", i)), []byte("meta"))
		}
	}()

	for i := 0; i < 500; i++ {
		meta, errGet := s.SFTGetMetaUncached([]byte("ASSET"), []byte("1"))
		require.NoError(t, errGet)
		require.NotNil(t, meta)
	}
	<-done
}

func newFixSystemAccountKApp(t *testing.T, fixActive bool) *systemAccountKApp {
	t.Helper()

	marshalizer := &marshal.ProtoMarshalizer{}
	store := make(map[string][]byte)

	tracker := &commonMock.DataTrieTrackerStub{
		RetrieveValueCalled: func(key []byte) ([]byte, error) {
			return store[string(key)], nil
		},
		SaveKeyValueCalled: func(key []byte, value []byte) error {
			store[string(key)] = value
			return nil
		},
	}

	kappAccount := &commonMock.KAppAccountHandlerStub{
		DataTrieTrackerCalled: func() state.DataTrieTracker {
			return tracker
		},
	}

	fc := commonMock.NewForkControllerStub()
	fc.SetFork("FixMarketBuyOverflow", fixActive)

	s := &systemAccountKApp{marshalizer: marshalizer, forkController: fc}
	require.NoError(t, s.SetAccountsCacher(&commonMock.AccountsCacherStub{
		LoadKAppCalled: func(address []byte) (state.KAppAccountHandler, error) {
			return kappAccount, nil
		},
	}))

	return s
}

func readMetaFix(t *testing.T, s *systemAccountKApp, asset, nonce []byte) *kapps.MetaV2 {
	t.Helper()
	meta, err := s.SFTGetMeta(asset, nonce)
	require.NoError(t, err)
	require.NotNil(t, meta)
	return meta
}

func TestFix_SFTCirculationOverflowRejected(t *testing.T) {
	asset := []byte("SFTASSET")
	nonce := []byte{0x01}

	const maxSupply = int64(1000)
	const startCirculation = int64(5)
	const overflowAmount = int64(math.MaxInt64)

	t.Run("fix_on_overflow_rejected", func(t *testing.T) {
		s := newFixSystemAccountKApp(t, true)
		require.NoError(t, s.SFTCreateMeta(asset, nonce, maxSupply, []byte("hash")))
		require.NoError(t, s.SFTAddCirculation(asset, nonce, startCirculation))

		err := s.SFTAddCirculation(asset, nonce, overflowAmount)
		require.ErrorIs(t, err, common.ErrMaxSupplyExceeded,
			"an overflowing add must be rejected, not wrap past the cap")

		after := readMetaFix(t, s, asset, nonce)
		require.Equal(t, startCirculation, after.Circulation,
			"rejected overflow must not persist new circulation")
		require.GreaterOrEqual(t, after.Circulation, int64(0),
			"circulation must never wrap negative")
	})

	t.Run("fix_off_legacy_overflow_preserved", func(t *testing.T) {
		s := newFixSystemAccountKApp(t, false)
		require.NoError(t, s.SFTCreateMeta(asset, nonce, maxSupply, []byte("hash")))
		require.NoError(t, s.SFTAddCirculation(asset, nonce, startCirculation))

		err := s.SFTAddCirculation(asset, nonce, overflowAmount)
		require.NoError(t, err,
			"pre-fork behavior is unchanged: the overflow still wraps and is accepted")

		after := readMetaFix(t, s, asset, nonce)
		require.Negative(t, after.Circulation,
			"legacy path leaves circulation wrapped negative")
	})

	t.Run("fix_on_normal_overcap_still_rejected", func(t *testing.T) {
		s := newFixSystemAccountKApp(t, true)
		require.NoError(t, s.SFTCreateMeta(asset, nonce, maxSupply, []byte("hash")))
		require.NoError(t, s.SFTAddCirculation(asset, nonce, startCirculation))

		err := s.SFTAddCirculation(asset, nonce, 2000)
		require.ErrorIs(t, err, common.ErrMaxSupplyExceeded)

		after := readMetaFix(t, s, asset, nonce)
		require.Equal(t, startCirculation, after.Circulation)
	})

	t.Run("fix_on_mint_and_burn_unaffected", func(t *testing.T) {
		s := newFixSystemAccountKApp(t, true)
		require.NoError(t, s.SFTCreateMeta(asset, nonce, maxSupply, []byte("hash")))
		require.NoError(t, s.SFTAddCirculation(asset, nonce, 100))
		require.NoError(t, s.SFTAddCirculation(asset, nonce, -40))

		after := readMetaFix(t, s, asset, nonce)
		require.Equal(t, int64(60), after.Circulation,
			"in-range mint and burn still work normally under the fork")
	})
}
