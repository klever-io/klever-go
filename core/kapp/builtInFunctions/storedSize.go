package builtInFunctions

import (
	"bytes"
	"encoding/binary"

	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/kvm/math"
)

// The charges below size a record by its value, as contract storage prices a new record
// (StorePerByte on the value's length; the key is not priced). Every record pays at least its value
// bound, so a call that writes many records pays for each one.
//
// They are upper bounds worked out from the call's arguments, before the kapp runs. They do not
// repeat the kapp's checks: a call the kapp will refuse is priced like one it accepts, and with less
// gas than the charge it fails with ErrNotEnoughGas before anything is written. A failed builtin uses
// all its gas either way.

// maxVarintFieldSize bounds a protobuf field that holds a varint or a length prefix: a one-byte tag
// and the varint. It sizes the records a builtin writes before their values are known.
const maxVarintFieldSize uint64 = 1 + binary.MaxVarintLen64

// whitelistStoredSize returns the bytes an ITO whitelist adds to the ITO kapp: one record per
// address, holding the address's limit. Every entry pays, including one already on the list, since
// the builtin does not load the stored records.
func whitelistStoredSize(whitelist map[string]*transaction.WhitelistInfo) uint64 {
	return math.MulUint64(uint64(len(whitelist)), maxVarintFieldSize)
}

// sftMetaHashSize is the length of the hash a new SFT nonce's metadata holds: the digest of the
// node's hasher, which every hasher the node config can select (sha256, keccak, blake2b) gives.
const sftMetaHashSize = 32

// sftNewNonceSize bounds what a mint of a new SFT nonce stores: the nonce's MetaV2 record in the
// system account (Circulation, MaxSupply, and the Metadata message framing a Hash field) and the
// recipient's holder record.
const sftNewNonceSize = 2*maxVarintFieldSize + // Circulation, MaxSupply
	2*maxVarintFieldSize + sftMetaHashSize + // Metadata and Hash framing, the hash
	maxVarintFieldSize // holder record

// mintStoredSize returns the bytes the KDA kapp stores beyond what the flat cost covers when it
// mints amount tokens of assetID. The asset is the part of assetID before the first separator and
// the nonce the part after it, as the KDA kapp's Mint parses them.
//
//   - NonFungible: one holder record per token, holding its balance.
//   - SemiFungible without a nonce: a new nonce, with its metadata record and one holder record,
//     whatever the amount (the kapp creates the nonce for an amount of 0 too).
//
// It returns 0 for a mint that adds no record: a negative amount, an asset that does not load, a
// fungible mint (it updates one balance), and an SFT mint of an existing nonce (one holder record).
func mintStoredSize(kappController kapp.KAppController, assetID []byte, amount int64) uint64 {
	if amount < 0 {
		return 0
	}

	var nonce []byte
	if i := bytes.Index(assetID, []byte(kapps.Sp)); i >= 0 {
		assetID, nonce = assetID[:i], assetID[i+1:]
	}

	_, kda, err := kappController.GetKDAKApp().GetKDA(assetID)
	if err != nil || kda == nil {
		return 0
	}

	switch {
	case kda.AssetType == kapps.KDAData_NonFungible:
		return math.MulUint64(uint64(amount), maxVarintFieldSize) // #nosec G115 -- the amount is not negative
	case kda.AssetType == kapps.KDAData_SemiFungible && len(nonce) == 0:
		return sftNewNonceSize
	default:
		return 0
	}
}
