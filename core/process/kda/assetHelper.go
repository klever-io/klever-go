package kda

import (
	"encoding/binary"
	"fmt"
	"math/big"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/martinlindhe/base36"
)

// CheckBasicCreateArguments verifies if the basic arguments are correct for asset creation
func CheckBasicCreateArguments(asset *transaction.CreateAssetContract) error {
	if len(asset.Name) < core.MinLengthForAssetName ||
		len(asset.Name) > core.MaxLengthForAssetName {
		return common.ErrAssetNameInvalid
	}

	if len(asset.Ticker) < core.MinLengthForAssetTicker ||
		len(asset.Ticker) > core.MaxLengthForAssetTicker {
		return common.ErrAssetTickerLengthInvalid
	}

	if asset.Type != transaction.CreateAssetContract_NonFungible && asset.Royalties != nil && (asset.Royalties.MarketFixed > 0 || asset.Royalties.MarketPercentage > 0) {
		return fmt.Errorf("%w only NonFungible tokens have market royalties", process.ErrInvalidArgument)
	}

	return nil
}

// CheckPrecision verifies if the precision is within the allowed limits
func CheckPrecision(precision uint32) error {
	if precision < core.MinNumberOfDecimals ||
		precision > core.MaxNumberOfDecimals {
		return common.ErrAssetPrecision
	}

	return nil
}

func CheckValid100Params(values ...uint32) bool {
	for _, value := range values {
		if value > core.HundredPercent {
			return false
		}
	}
	return true
}

// CreateNewAssetIdentifier Create a random asset identifier for the asset
func CreateNewAssetIdentifier(hasher hashing.Hasher, randSeed []byte, caller []byte, nonce uint64, ticker []byte) []byte {
	nonceBuffer := make([]byte, 8)
	binary.BigEndian.PutUint64(nonceBuffer, nonce)

	newRandom := hasher.Compute(string(randSeed) + string(caller) + string(nonceBuffer) + string(ticker))
	newRandomAsBigInt := big.NewInt(0).SetBytes(newRandom)
	encoded := base36.Encode(newRandomAsBigInt.Uint64())[:kdautils.TickerRandomSequenceLength]

	tickerPrefix := append(ticker, []byte(kdautils.TickerSeparator)...)
	newIdentifier := append(tickerPrefix, encoded...)

	return newIdentifier
}
