//go:generate protoc -I=proto -I=$GOPATH/src -I=$GOPATH/src/github.com/klever-io/klever-go/protobuf --go_out=. bootstrapData.proto
package bootstrapStorage

import (
	"bytes"
	"errors"
	"strconv"
	"sync/atomic"

	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/storage"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/tools/marshal"
)

// ErrNilMarshalizer signals that an operation has been attempted to or with a nil marshalizer implementation
var ErrNilMarshalizer = errors.New("nil marshalizer")

// ErrNilBootStorer signals that an operation has been attempted to or with a nil storer implementation
var ErrNilBootStorer = errors.New("nil boot storer")

// ErrBootDataHeaderMismatch signals that a boot record does not describe the expected header
var ErrBootDataHeaderMismatch = errors.New("boot data header mismatch")

// ErrFinalNonceAboveHeader signals that a final nonce is higher than the nonce of the header in the boot record
var ErrFinalNonceAboveHeader = errors.New("final nonce above boot data header nonce")

type bootstrapStorer struct {
	store       storage.Storer
	marshalizer marshal.Marshalizer
	lastSlot    int64
}

// NewBootstrapStorer will return an instance of bootstrap storer
func NewBootstrapStorer(
	marshalizer marshal.Marshalizer,
	store storage.Storer,
) (*bootstrapStorer, error) {
	if check.IfNil(marshalizer) {
		return nil, ErrNilMarshalizer
	}
	if check.IfNil(store) {
		return nil, ErrNilBootStorer
	}

	bootStorer := &bootstrapStorer{
		store:       store,
		marshalizer: marshalizer,
	}
	bootStorer.lastSlot = bootStorer.GetHighestSlot()

	return bootStorer, nil
}

// Put will save bootData in storage
func (bs *bootstrapStorer) Put(slot int64, bootData *BootstrapData) error {
	bootData.LastSlot = atomic.LoadInt64(&bs.lastSlot)

	// save bootstrap slot information
	bootDataBytes, err := bs.marshalizer.Marshal(bootData)
	if err != nil {
		return err
	}

	key := []byte(strconv.FormatInt(slot, 10))
	err = bs.store.Put(key, bootDataBytes)
	if err != nil {
		return err
	}

	// save slot with a static key
	slotBytes, err := bs.marshalizer.Marshal(&SlotNum{Num: slot})
	if err != nil {
		return err
	}

	err = bs.store.Put([]byte(core.HighestSlotFromBootStorage), slotBytes)
	if err != nil {
		return err
	}

	atomic.StoreInt64(&bs.lastSlot, slot)

	return nil
}

// Get will read data from storage
func (bs *bootstrapStorer) Get(slot int64) (*BootstrapData, error) {
	key := []byte(strconv.FormatInt(slot, 10))
	bootstrapDataBytes, err := bs.store.Get(key)
	if err != nil {
		return &BootstrapData{}, err
	}

	var bootData BootstrapData
	err = bs.marshalizer.Unmarshal(&bootData, bootstrapDataBytes)
	if err != nil {
		return &BootstrapData{}, err
	}

	return &bootData, nil
}

// GetHighestSlot will return highest slot saved in storage
func (bs *bootstrapStorer) GetHighestSlot() int64 {
	slotBytes, err := bs.store.Get([]byte(core.HighestSlotFromBootStorage))
	if err != nil {
		return 0
	}

	var slot SlotNum
	err = bs.marshalizer.Unmarshal(&slot, slotBytes)
	if err != nil {
		return 0
	}

	return slot.Num
}

// SaveLastSlot will save the last slot
func (bs *bootstrapStorer) SaveLastSlot(slot int64) error {
	atomic.StoreInt64(&bs.lastSlot, slot)

	// save slot with a static key
	slotBytes, err := bs.marshalizer.Marshal(&SlotNum{Num: slot})
	if err != nil {
		return err
	}

	err = bs.store.Put([]byte(core.HighestSlotFromBootStorage), slotBytes)
	if err != nil {
		return err
	}

	return nil
}

// RaiseHighestFinalBlockNonce sets the final nonce of the record saved at the given slot to nonce if it is higher,
// it never lowers it. The record must describe the header with the given hash and nonce can not be above that header
func (bs *bootstrapStorer) RaiseHighestFinalBlockNonce(slot int64, nonce uint64, headerHash []byte) error {
	bootData, err := bs.Get(slot)
	if err != nil {
		return err
	}

	lastHeader := bootData.LastHeader
	if lastHeader == nil || !bytes.Equal(lastHeader.Hash, headerHash) {
		return ErrBootDataHeaderMismatch
	}
	if nonce > lastHeader.Nonce {
		return ErrFinalNonceAboveHeader
	}
	if bootData.HighestFinalBlockNonce >= nonce {
		return nil
	}

	bootData.HighestFinalBlockNonce = nonce

	bootDataBytes, err := bs.marshalizer.Marshal(bootData)
	if err != nil {
		return err
	}

	return bs.store.PutInEpoch([]byte(strconv.FormatInt(slot, 10)), bootDataBytes, lastHeader.Epoch)
}

// IsInterfaceNil returns true if there is no value under the interface
func (bs *bootstrapStorer) IsInterfaceNil() bool {
	return bs == nil
}
