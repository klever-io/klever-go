package builtInFunctions

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"math/big"

	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/data/transaction"
)

var log = logger.GetOrCreate("core/builtInFunctions")

func DecodeITOPacks(data []byte) (map[string]*transaction.PackInfo, error) {
	buf := bytes.NewReader(data)

	res := make(map[string]*transaction.PackInfo)

	var length uint32
	err := binary.Read(buf, binary.BigEndian, &length)
	if err != nil {
		return nil, err
	}

	for i := uint32(0); i < length; i++ {
		var tokenLen uint32
		err := binary.Read(buf, binary.BigEndian, &tokenLen)
		if err != nil {
			return nil, err
		}

		token := make([]byte, tokenLen)
		err = binary.Read(buf, binary.BigEndian, token)
		if err != nil {
			return nil, err
		}

		items, err := decodeITOPackItem(buf)
		if err != nil {
			return nil, err
		}

		res[string(token)] = &transaction.PackInfo{
			Packs: items,
		}
	}

	if buf.Len() != 0 {
		return nil, errors.New("extra bytes found in buffer")
	}

	return res, nil
}

func decodeITOPackItem(buf *bytes.Reader) ([]*transaction.PackItem, error) {
	var res []*transaction.PackItem

	var length uint32
	err := binary.Read(buf, binary.BigEndian, &length)
	if err != nil {
		return nil, err
	}

	for i := uint32(0); i < length; i++ {
		item := &transaction.PackItem{
			Amount: readBigUint(buf).Int64(),
			Price:  readBigUint(buf).Int64(),
		}

		res = append(res, item)
	}

	if buf.Len() != 0 {
		return nil, errors.New("extra bytes found in buffer child")
	}

	return res, nil
}

func DecodeITOWhitelist(data []byte) (map[string]*transaction.WhitelistInfo, error) {
	buf := bytes.NewReader(data)

	res := make(map[string]*transaction.WhitelistInfo)

	var length uint32
	err := binary.Read(buf, binary.BigEndian, &length)
	if err != nil {
		return nil, err
	}

	for i := uint32(0); i < length; i++ {
		wlInfo := &transaction.WhitelistInfo{}

		address := make([]byte, 32)
		err := binary.Read(buf, binary.BigEndian, address)
		if err != nil {
			return nil, err
		}

		wlInfo.Limit = readBigUint(buf).Int64()

		res[hex.EncodeToString(address)] = wlInfo
	}

	if buf.Len() != 0 {
		return nil, errors.New("extra bytes found in buffer")
	}

	return res, nil
}

func DecodeURIs(data []byte) (map[string]string, error) {
	buf := bytes.NewReader(data)

	var length uint32
	err := binary.Read(buf, binary.BigEndian, &length)
	if err != nil {
		return nil, err
	}

	URIs := make(map[string]string)

	for i := uint32(0); i < length; i++ {
		var keyLen uint32
		err := binary.Read(buf, binary.BigEndian, &keyLen)
		if err != nil {
			return nil, err
		}

		key := make([]byte, keyLen)
		err = binary.Read(buf, binary.BigEndian, key)
		if err != nil {
			return nil, err
		}

		var valueLen uint32
		err = binary.Read(buf, binary.BigEndian, &valueLen)
		if err != nil {
			return nil, err
		}

		value := make([]byte, valueLen)
		err = binary.Read(buf, binary.BigEndian, value)
		if err != nil {
			return nil, err
		}

		URIs[string(key)] = string(value)
	}

	if buf.Len() != 0 {
		return nil, errors.New("extra bytes found in buffer")
	}

	return URIs, nil
}

func readBigUint(buf *bytes.Reader) *big.Int {
	var length uint32
	err := binary.Read(buf, binary.BigEndian, &length)
	if err != nil {
		return nil
	}
	valBytes := make([]byte, length)
	err = binary.Read(buf, binary.BigEndian, valBytes)
	if err != nil {
		return nil
	}
	return new(big.Int).SetBytes(valBytes)
}

func DecodeRoyaltiesData(data []byte) (*transaction.RoyaltiesInfo, error) {
	buf := bytes.NewReader(data)
	royalties := &transaction.RoyaltiesInfo{}

	// Reading Address
	royalties.Address = make([]byte, 32)
	err := binary.Read(buf, binary.BigEndian, royalties.Address)
	if err != nil {
		return nil, err
	}

	// Reading TransferPercentage
	var tpLength uint32
	err = binary.Read(buf, binary.BigEndian, &tpLength)
	if err != nil {
		return nil, err
	}

	for i := uint32(0); i < tpLength; i++ {
		tp := &transaction.RoyaltyInfo{
			Amount: readBigUint(buf).Int64(),
		}

		err := binary.Read(buf, binary.BigEndian, &tp.Percentage)
		if err != nil {
			return nil, err
		}
		royalties.TransferPercentage = append(royalties.TransferPercentage, tp)
	}

	// Reading TransferFixed
	royalties.TransferFixed = readBigUint(buf).Int64()

	// Reading MarketPercentage
	err = binary.Read(buf, binary.BigEndian, &royalties.MarketPercentage)
	if err != nil {
		return nil, err
	}

	// Reading MarketFixed
	royalties.MarketFixed = readBigUint(buf).Int64()

	// Reading SplitRoyalties
	var srLength uint32
	err = binary.Read(buf, binary.BigEndian, &srLength)
	if err != nil {
		return nil, err
	}

	royalties.SplitRoyalties = make(map[string]*transaction.RoyaltySplitInfo)

	for i := uint32(0); i < srLength; i++ {
		key := make([]byte, 32)
		err := binary.Read(buf, binary.BigEndian, key)
		if err != nil {
			return nil, err
		}

		encodedKey := hex.EncodeToString(key)

		royalties.SplitRoyalties[encodedKey] = &transaction.RoyaltySplitInfo{}

		err = binary.Read(buf, binary.BigEndian, &royalties.SplitRoyalties[encodedKey].PercentTransferPercentage)
		if err != nil {
			return nil, err
		}
		err = binary.Read(buf, binary.BigEndian, &royalties.SplitRoyalties[encodedKey].PercentTransferFixed)
		if err != nil {
			return nil, err
		}
		err = binary.Read(buf, binary.BigEndian, &royalties.SplitRoyalties[encodedKey].PercentMarketPercentage)
		if err != nil {
			return nil, err
		}
		err = binary.Read(buf, binary.BigEndian, &royalties.SplitRoyalties[encodedKey].PercentMarketFixed)
		if err != nil {
			return nil, err
		}
		err = binary.Read(buf, binary.BigEndian, &royalties.SplitRoyalties[encodedKey].PercentITOPercentage)
		if err != nil {
			return nil, err
		}
		err = binary.Read(buf, binary.BigEndian, &royalties.SplitRoyalties[encodedKey].PercentITOFixed)
		if err != nil {
			return nil, err
		}
	}

	// Reading ItoPercentage
	err = binary.Read(buf, binary.BigEndian, &royalties.ITOPercentage)
	if err != nil {
		return nil, err
	}

	// Reading ItoFixed
	royalties.ITOFixed = readBigUint(buf).Int64()

	if buf.Len() != 0 {
		return nil, errors.New("extra bytes found in buffer")
	}

	return royalties, nil
}
