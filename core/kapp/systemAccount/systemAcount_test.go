package systemAccount

import (
	"errors"
	"fmt"
	"testing"

	"github.com/klever-io/klever-go/common"
	commonMock "github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/data/state"
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
