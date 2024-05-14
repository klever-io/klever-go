package kdautils

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"strconv"

	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/kapps"
	"github.com/multiformats/go-base36"
)

const TickerSeparatorByte = '-'
const TickerSeparator = string(TickerSeparatorByte)
const TickerRandomSequenceLength = 4

var (
	KLVIdentifier         = []byte("KLV")
	KFIIdentifier         = []byte("KFI")
	KLVKey                = ToKDAKey(KLVIdentifier, nil)
	KFIKey                = ToKDAKey(KFIIdentifier, nil)
	ProposalControllerKey = ToProposalKey(0)
	MarketKeyLength       = 8
)

// IsTickerValid verifies if ticker satisfies all rules
func IsTickerValid(tickerName []byte) bool {
	if len(tickerName) < core.MinLengthForAssetTicker ||
		len(tickerName) > core.MaxLengthForAssetTicker ||
		bytes.Equal(tickerName, KLVIdentifier) ||
		bytes.Equal(tickerName, KFIIdentifier) {
		return false
	}

	for _, ch := range tickerName {
		isBigCharacter := ch >= 'A' && ch <= 'Z'
		isNumber := ch >= '0' && ch <= '9'
		isReadable := isBigCharacter || isNumber
		if !isReadable {
			return false
		}
	}

	return true
}

// IsAssetNameHumanReadable verifies if the asset name is human readable
func IsAssetNameHumanReadable(assetName []byte) bool {
	if len(assetName) < core.MinLengthForAssetName ||
		len(assetName) > core.MaxLengthForAssetName {
		return false
	}

	for _, ch := range assetName {
		isSmallCharacter := ch >= 'a' && ch <= 'z'
		isBigCharacter := ch >= 'A' && ch <= 'Z'
		isNumber := ch >= '0' && ch <= '9'
		isSpace := ch == ' '
		isReadable := isSmallCharacter || isBigCharacter || isNumber || isSpace
		if !isReadable {
			return false
		}
	}
	return true
}

// IsSupplyValid verifies if the supply satisfies all rules
func IsSupplyValid(mintable bool, initSupply, circSupply, maxSupply int64) bool {
	//MaxSupply == 0 alows infinite max supply
	if initSupply < 0 || circSupply < 0 || maxSupply < 0 {
		return false
	}

	if !mintable {
		if initSupply == circSupply && initSupply == maxSupply {
			return true
		}
		return false
	}

	if initSupply == circSupply && (maxSupply == 0 || initSupply <= maxSupply) {
		return true
	}

	return false
}

// IsNFTContractValid verifies if the NFT precision and initial supply is set to 0
func IsNFTContractValid(initSupply int64, precision uint32) bool {
	if initSupply != 0 {
		return false
	}

	if precision != 0 {
		return false
	}

	return true
}

func ToBucketID(hasher hashing.Hasher, randSeed, sender, assetID []byte, nonce uint64, contractIndex int, amount int64) []byte {
	nonceBuffer := make([]byte, 8)
	contractIndexBuffer := make([]byte, 8)
	amountBuffer := make([]byte, 8)
	binary.BigEndian.PutUint64(nonceBuffer, nonce)
	binary.BigEndian.PutUint64(contractIndexBuffer, uint64(contractIndex))
	binary.BigEndian.PutUint64(amountBuffer, uint64(amount))

	return hasher.Compute(
		string(randSeed) +
			string(sender) +
			string(assetID) +
			fmt.Sprintf("%x", nonceBuffer) +
			fmt.Sprintf("%x", contractIndexBuffer) +
			fmt.Sprintf("%x", amountBuffer))
}

func ToMarketID(hasher hashing.Hasher, randSeed, sender []byte, nonce uint64, contractIndex int, marketKeyLength int) []byte {
	nonceBuffer := make([]byte, 8)
	contractIndexBuffer := make([]byte, 8)
	binary.BigEndian.PutUint64(nonceBuffer, nonce)
	binary.BigEndian.PutUint64(contractIndexBuffer, uint64(contractIndex))

	return hasher.Compute(string(randSeed) + string(sender) + fmt.Sprintf("%x", nonceBuffer) + fmt.Sprintf("%x", contractIndexBuffer))[:marketKeyLength]
}

// ToKDAKey parse a key for the given KDA
func ToKDAKey(kdaID []byte, nonce []byte) []byte {
	key := kapps.KDAPrefix + kapps.Sp + string(kdaID)
	if len(nonce) > 0 && string(nonce) != "0" {
		// access internal tokenID for NFT
		key += kapps.Sp + string(nonce)
	}
	return []byte(key)
}

// ToKDAKeyWithouPrefix parse a key for the given KDA
func ToKDAKeyWithouPrefix(kdaID []byte, nonce []byte) []byte {
	key := string(kdaID)
	if len(nonce) > 0 && string(nonce) != "0" {
		// access internal tokenID for NFT
		key += kapps.Sp + string(nonce)
	}
	return []byte(key)
}

// ToProposalKey parse a key for the given proposal
func ToProposalKey(proposalID uint64) []byte {
	return []byte(kapps.ProposalPrefix + kapps.Sp + strconv.FormatUint(proposalID, 10))
}

// ToITOKey parse a key for the given ITO
func ToITOKey(kdaID []byte) []byte {
	return []byte(kapps.ITOPrefix + kapps.Sp + string(kdaID))
}

// ToITOWhitelistKey parse a key for the given ITO whitelist
func ToITOWhitelistKey(kdaID []byte, hexAddress string) []byte {
	return []byte(kapps.ITOWhitelistPrefix + kapps.Sp + string(kdaID) + kapps.Sp + hexAddress)
}

// ToMarketOrderKey parse a key for the given market order
func ToMarketOrderKey(orderID []byte) []byte {
	return []byte(kapps.MarketOrderPrefix + kapps.Sp + string(orderID))
}

// ToMarketplaceKey parse a key for the given marketplace
func ToMarketplaceKey(marketplaceID []byte) []byte {
	return []byte(kapps.MarketplacePrefix + kapps.Sp + string(marketplaceID))
}

// ExtractTokenIDAndNonce extract tokenID and nonce from key
// Return KLV tokenID if nil or empty
// Check Asset type and if not valid ticker return error
func ExtractAssetIDAndNonce(key []byte) ([]byte, uint64, core.KDAType, error) {
	if len(key) == 0 {
		return KLVIdentifier, 0, 0, nil
	}

	// check if is KLV or KFI identifier
	if bytes.Equal(key, KLVIdentifier) || bytes.Equal(key, KFIIdentifier) {
		return key, 0, 0, nil
	}

	tokenType := core.Fungible
	nonce := uint64(0)

	// check if any nonce is provided by checking separator
	keySplit := bytes.Split(key, []byte(kapps.Sp))
	if len(keySplit) > 1 {
		tokenType = core.NonFungible
		var err error
		nonce, err = strconv.ParseUint(string(keySplit[1]), 10, 64)
		if err != nil {
			return nil, 0, 0, err
		}
	}

	// check if valid ticker split by dash separator
	tickerSplit := bytes.Split(keySplit[0], []byte(TickerSeparator))
	if len(tickerSplit) != 2 {
		return nil, 0, 0, fmt.Errorf("invalid ticker name")
	}

	// check ticker length
	if len(tickerSplit[0]) < core.MinLengthForAssetTicker || len(tickerSplit[0]) > core.MaxLengthForAssetTicker {
		return nil, 0, 0, fmt.Errorf("invalid ticker name length")
	}

	// check random sequence length
	if len(tickerSplit[1]) != TickerRandomSequenceLength {
		return nil, 0, 0, fmt.Errorf("invalid ticker random sequence length: %d", len(tickerSplit[1]))
	}

	// check if random sequence is a valid base36 encoding
	_, err := base36.DecodeString(string(tickerSplit[1]))
	if err != nil {
		return nil, 0, 0, fmt.Errorf("invalid ticker random sequence encoding: %s", string(tickerSplit[1]))
	}

	return keySplit[0], uint64(nonce), tokenType, nil
}
