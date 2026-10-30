package mock

import (
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/tools/marshal"
)

// NFTHolderRecord is an NFT holder record with metadata set: held with balance 1 as Mint stores
// it, or spent with balance 0 as the fungible balance path leaves it (KLC-2835).
func NFTHolderRecord(balance int64) []byte {
	data, err := marshal.NewProtoMarshalizer().Marshal(&kapps.UserKDA{Balance: balance, Metadata: []byte("nft-metadata")})
	if err != nil {
		panic(err)
	}

	return data
}
