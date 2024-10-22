package kda

import (
	"encoding/binary"
	"fmt"
	"math/big"
	"unicode/utf8"

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

func ValidateCreateAsset(tc *transaction.CreateAssetContract, forkController core.ForkController, pubkeyConv core.PubkeyConverter) (transaction.Transaction_TXResultCode, error) {
	var isValid bool

	if forkController.EnableSmartContracts() {
		isValid = kdautils.IsAssetNameHumanReadable(tc.Name)
	} else {
		isValid = kdautils.IsAssetNameHumanReadableOld(tc.Name)
	}

	if !isValid {
		return transaction.Transaction_ContractInvalid, process.ErrTokenNameNotHumanReadable
	}

	if !kdautils.IsTickerValid(tc.Ticker) {
		return transaction.Transaction_ContractInvalid, process.ErrTickerNameNotValid
	}

	if len(tc.GetOwnerAddress()) != pubkeyConv.Len() {
		return transaction.Transaction_AccountError, process.ErrInvalidOwnerAddr
	}

	if forkController.EnableSmartContracts() &&
		len(tc.GetAdminAddress()) > 0 &&
		len(tc.GetAdminAddress()) != pubkeyConv.Len() {
		return transaction.Transaction_AccountError, process.ErrInvalidAdminAddr
	}

	if tc.GetMaxSupply() < 0 {
		return transaction.Transaction_ParameterInvalid, process.ErrSupplyNotValid
	}

	return transaction.Transaction_Ok, nil
}

func ValidateLogoAndUri(tc *transaction.CreateAssetContract) (transaction.Transaction_TXResultCode, error) {
	if !utf8.ValidString(tc.GetLogo()) || len(tc.GetLogo()) > core.MaxLogoURISize {
		return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
	}

	if len(tc.GetURIs()) > core.MaxURIMapSize {
		return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
	}

	for key, uri := range tc.GetURIs() {
		if !utf8.ValidString(key) ||
			!utf8.ValidString(uri) ||
			len(key) > core.MaxURIKeySize ||
			len(uri) > core.MaxURIValueSize {
			return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
		}
	}
	return transaction.Transaction_Ok, nil
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
func CreateNewAssetIdentifier(hasher hashing.Hasher, randSeed []byte, caller []byte, nonce uint64, ticker []byte, contractID int) []byte {
	nonceBuffer := make([]byte, 8)
	binary.BigEndian.PutUint64(nonceBuffer, nonce)

	seed := string(randSeed) + string(caller) + string(nonceBuffer) + string(ticker)
	if contractID > 0 {
		idBuffer := make([]byte, 8)
		binary.BigEndian.PutUint64(idBuffer, uint64(contractID))
		seed += string(idBuffer)
	}

	newRandom := hasher.Compute(seed)
	newRandomAsBigInt := big.NewInt(0).SetBytes(newRandom)
	encoded := base36.Encode(newRandomAsBigInt.Uint64())[:kdautils.TickerRandomSequenceLength]

	tickerPrefix := append(ticker, []byte(kdautils.TickerSeparator)...)
	newIdentifier := append(tickerPrefix, encoded...)

	return newIdentifier
}
