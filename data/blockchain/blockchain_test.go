package blockchain_test

import (
	"fmt"
	"runtime"
	"testing"
	"time"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/blockchain"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewBlockChain_ShouldWork(t *testing.T) {
	t.Parallel()

	bc := blockchain.NewBlockChain()

	assert.False(t, check.IfNil(bc))
}

func TestBlockChain_SetNilAppStatusHandlerShouldErr(t *testing.T) {
	t.Parallel()

	bc := blockchain.NewBlockChain()

	err := bc.SetAppStatusHandler(nil)

	assert.Equal(t, blockchain.ErrNilAppStatusHandler, err)
}

func TestBlockChain_SetAppStatusHandlerShouldWork(t *testing.T) {
	t.Parallel()

	bc := blockchain.NewBlockChain()

	ash := &mock.AppStatusHandlerStub{}
	err := bc.SetAppStatusHandler(ash)

	assert.Nil(t, err)
}

func TestBlockChain_SettersAndGetters(t *testing.T) {
	t.Parallel()

	bc := blockchain.NewBlockChain()

	hdr := &block.Block{
		Header: &block.BlockHeader{
			Nonce: 4,
		},
	}
	genesis := &block.Block{
		Header: &block.BlockHeader{
			Nonce: 0,
		},
	}
	hdrHash := []byte("hash")
	genesisHash := []byte("genesis hash")

	bc.SetCurrentBlockHeaderHash(hdrHash)
	bc.SetGenesisHeaderHash(genesisHash)

	err := bc.SetGenesisHeader(genesis)
	assert.Nil(t, err)

	err = bc.SetCurrentBlockHeader(hdr)
	assert.Nil(t, err)

	assert.Equal(t, hdr.Clone(), bc.GetCurrentBlockHeader())
	assert.False(t, hdr == bc.GetCurrentBlockHeader())

	assert.Equal(t, genesis.Clone(), bc.GetGenesisHeader())
	assert.False(t, genesis == bc.GetGenesisHeader())

	assert.Equal(t, hdrHash, bc.GetCurrentBlockHeaderHash())
	assert.Equal(t, genesisHash, bc.GetGenesisHeaderHash())
}

func TestBlockChain_SettersAndGettersNilValues(t *testing.T) {
	t.Parallel()

	bc := blockchain.NewBlockChain()

	err := bc.SetGenesisHeader(nil)
	assert.Nil(t, err)

	err = bc.SetCurrentBlockHeader(nil)
	assert.Nil(t, err)

	assert.Nil(t, bc.GetGenesisHeader())
	assert.Nil(t, bc.GetCurrentBlockHeader())
}

func TestBlockChain_CreateNewHeader(t *testing.T) {
	t.Parallel()

	bc := blockchain.NewBlockChain()

	assert.Equal(t, &block.Block{}, bc.CreateNewHeader())
}

func TestBlockChain_SettersWithInvalidBlock(t *testing.T) {
	t.Parallel()

	bc := blockchain.NewBlockChain()

	err := bc.SetCurrentBlockHeader(&mock.HeaderHandlerStub{})
	assert.Equal(t, common.ErrInvalidHeaderType, err)

	err = bc.SetGenesisHeader(&mock.HeaderHandlerStub{})
	assert.Equal(t, common.ErrInvalidHeaderType, err)
}

func TestBlockChain_SetCurrentBlockHeaderAndHashShouldWork(t *testing.T) {
	t.Parallel()

	bc := blockchain.NewBlockChain()

	hdr := &block.Block{
		Header: &block.BlockHeader{
			Nonce: 7,
		},
	}
	hash := []byte("matching-hash")

	err := bc.SetCurrentBlockHeaderAndHash(hdr, hash)
	assert.Nil(t, err)

	assert.Equal(t, hdr.Clone(), bc.GetCurrentBlockHeader())
	assert.Equal(t, hash, bc.GetCurrentBlockHeaderHash())
}

func TestBlockChain_SetCurrentBlockHeaderAndHashNilHeader(t *testing.T) {
	t.Parallel()

	bc := blockchain.NewBlockChain()

	hdr := &block.Block{
		Header: &block.BlockHeader{
			Nonce: 3,
		},
	}
	require.NoError(t, bc.SetCurrentBlockHeader(hdr))
	bc.SetCurrentBlockHeaderHash([]byte("old"))

	err := bc.SetCurrentBlockHeaderAndHash(nil, []byte("new"))
	assert.Nil(t, err)

	assert.Nil(t, bc.GetCurrentBlockHeader())
	assert.Equal(t, []byte("new"), bc.GetCurrentBlockHeaderHash())
}

func TestBlockChain_SetCurrentBlockHeaderAndHashInvalidHeaderType(t *testing.T) {
	t.Parallel()

	bc := blockchain.NewBlockChain()

	hdr := &block.Block{
		Header: &block.BlockHeader{
			Nonce: 1,
		},
	}
	hash := []byte("preserved")
	require.NoError(t, bc.SetCurrentBlockHeader(hdr))
	bc.SetCurrentBlockHeaderHash(hash)

	err := bc.SetCurrentBlockHeaderAndHash(&mock.HeaderHandlerStub{}, []byte("ignored"))
	assert.Equal(t, common.ErrInvalidHeaderType, err)

	assert.Equal(t, hdr.Clone(), bc.GetCurrentBlockHeader())
	assert.Equal(t, hash, bc.GetCurrentBlockHeaderHash())
}

func TestBlockChain_GetCurrentBlockHeaderAndHashAtomicPairs(t *testing.T) {
	t.Parallel()

	bc := blockchain.NewBlockChain()

	hdrA := &block.Block{Header: &block.BlockHeader{Nonce: 10}}
	hashA := []byte("hash-10")
	hdrB := &block.Block{Header: &block.BlockHeader{Nonce: 9}}
	hashB := []byte("hash-9")

	// The pair (nonce, hash) must always be one of these two valid combinations.
	pairs := map[uint64]string{
		10: "hash-10",
		9:  "hash-9",
	}
	require.NoError(t, bc.SetCurrentBlockHeaderAndHash(hdrA, hashA))

	stop := make(chan struct{})
	mismatch := make(chan string, 1)
	done := make(chan struct{}, 2)

	go func() {
		defer func() { done <- struct{}{} }()
		for {
			select {
			case <-stop:
				return
			default:
			}
			h, hash := bc.GetCurrentBlockHeaderAndHash()
			if h != nil {
				want, ok := pairs[h.GetNonce()]
				if !ok || want != string(hash) {
					select {
					case mismatch <- fmt.Sprintf("nonce=%d hash=%q", h.GetNonce(), string(hash)):
					default:
					}
					return
				}
			}
			runtime.Gosched()
		}
	}()

	go func() {
		defer func() { done <- struct{}{} }()
		for {
			select {
			case <-stop:
				return
			default:
			}
			_ = bc.SetCurrentBlockHeaderAndHash(hdrB, hashB)
			_ = bc.SetCurrentBlockHeaderAndHash(hdrA, hashA)
			runtime.Gosched()
		}
	}()

	time.Sleep(100 * time.Millisecond)
	close(stop)
	<-done
	<-done

	select {
	case m := <-mismatch:
		t.Fatalf("observed mismatched (header, hash) pair via paired-read getter: %s", m)
	default:
	}
}

func TestBlockChain_GetCurrentBlockHeaderAndHashEmpty(t *testing.T) {
	t.Parallel()

	bc := blockchain.NewBlockChain()

	h, hash := bc.GetCurrentBlockHeaderAndHash()
	assert.Nil(t, h)
	assert.Nil(t, hash)
}

func TestBlockChain_SetCurrentBlockHeaderAndHashRaceClean(t *testing.T) {
	t.Parallel()

	bc := blockchain.NewBlockChain()

	hdrA := &block.Block{Header: &block.BlockHeader{Nonce: 10}}
	hashA := []byte("hash-10")
	hdrB := &block.Block{Header: &block.BlockHeader{Nonce: 9}}
	hashB := []byte("hash-9")
	require.NoError(t, bc.SetCurrentBlockHeaderAndHash(hdrA, hashA))

	stop := make(chan struct{})
	done := make(chan struct{}, 2)

	go func() {
		defer func() { done <- struct{}{} }()
		for {
			select {
			case <-stop:
				return
			default:
			}
			_ = bc.GetCurrentBlockHeader()
			_ = bc.GetCurrentBlockHeaderHash()
			runtime.Gosched()
		}
	}()

	go func() {
		defer func() { done <- struct{}{} }()
		for {
			select {
			case <-stop:
				return
			default:
			}
			_ = bc.SetCurrentBlockHeaderAndHash(hdrB, hashB)
			_ = bc.SetCurrentBlockHeaderAndHash(hdrA, hashA)
			runtime.Gosched()
		}
	}()

	time.Sleep(50 * time.Millisecond)
	close(stop)
	<-done
	<-done
}

func TestBlockChain_GetCurrentBlockRootHash(t *testing.T) {
	t.Parallel()

	bc := blockchain.NewBlockChain()

	// empty blockchain should return empty root hash
	assert.Equal(t, []byte(nil), bc.GetCurrentBlockRootHash())

	// set a block header and check the root hash
	hdr := &block.Block{
		Header: &block.BlockHeader{
			TrieRoot: []byte("root hash"),
		},
	}
	require.NoError(t, bc.SetCurrentBlockHeader(hdr))

	assert.Equal(t, []byte("root hash"), bc.GetCurrentBlockRootHash())
}
