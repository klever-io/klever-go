package builtInFunctions

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"

	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/data/transaction"
)

var log = logger.GetOrCreate("core/builtInFunctions")

func DecodeITOPacks(data []byte) (map[string]*transaction.PackInfo, error) {
	buf := bytes.NewReader(data)
	res := make(map[string]*transaction.PackInfo)

	length, err := readUint32(buf)
	if err != nil {
		return nil, err
	}

	// prevent large length
	if length > core.MaxPacks {
		return nil, common.ErrMaxBytesExceeded
	}

	for i := uint32(0); i < length; i++ {
		token, err := readBytesMaxSize(buf, 2*core.MaxLengthForAssetTicker)
		if err != nil {
			return nil, err
		}

		items, err := decodeITOPackItem(buf)
		if err != nil {
			return nil, err
		}

		res[string(token)] = &transaction.PackInfo{Packs: items}
	}

	if buf.Len() != 0 {
		return nil, errors.New("extra bytes found in buffer")
	}

	return res, nil
}

func decodeITOPackItem(buf *bytes.Reader) ([]*transaction.PackItem, error) {
	length, err := readUint32(buf)
	if err != nil {
		return nil, err
	}

	// prevent large length
	if length > core.MaxPackItems {
		return nil, common.ErrMaxBytesExceeded
	}

	res := make([]*transaction.PackItem, 0, length)

	for i := uint32(0); i < length; i++ {
		amount, err := readBigUint(buf)
		if err != nil {
			return nil, err
		}

		price, err := readBigUint(buf)
		if err != nil {
			return nil, err
		}

		item := &transaction.PackItem{
			Amount: amount.Int64(),
			Price:  price.Int64(),
		}

		res = append(res, item)
	}

	return res, nil
}

func DecodeITOWhitelist(data []byte) (map[string]*transaction.WhitelistInfo, error) {
	buf := bytes.NewReader(data)
	res := make(map[string]*transaction.WhitelistInfo)

	length, err := readUint32(buf)
	if err != nil {
		return nil, err
	}

	// prevent large length
	if length > core.MaxWhitelistSize {
		return nil, common.ErrMaxBytesExceeded
	}

	for i := uint32(0); i < length; i++ {
		address, err := decodeAddress(buf)
		if err != nil {
			return nil, err
		}

		limit, err := readBigUint(buf)
		if err != nil {
			return nil, err
		}

		res[hex.EncodeToString(address)] = &transaction.WhitelistInfo{
			Limit: limit.Int64(),
		}
	}

	if buf.Len() != 0 {
		return nil, errors.New("extra bytes found in buffer")
	}

	return res, nil
}

func DecodeURIs(data []byte) (map[string]string, error) {
	buf := bytes.NewReader(data)

	length, err := readUint32(buf)
	if err != nil {
		return nil, err
	}

	URIs := make(map[string]string)

	// prevent large length
	if length > core.MaxURIMapSize {
		return nil, common.ErrMaxBytesExceeded
	}

	for i := uint32(0); i < length; i++ {
		key, value, err := decodeURI(buf)
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

func decodeURI(buf *bytes.Reader) ([]byte, []byte, error) {

	key, err := readBytesMaxSize(buf, core.MaxURIKeySize)
	if err != nil {
		return nil, nil, err
	}

	value, err := readBytesMaxSize(buf, core.MaxURIValueSize)
	if err != nil {
		return nil, nil, err
	}

	return key, value, nil
}

func DecodeRoyaltiesData(data []byte) (*transaction.RoyaltiesInfo, error) {
	buf := bytes.NewReader(data)
	royalties := &transaction.RoyaltiesInfo{}

	address, err := decodeAddress(buf)
	if err != nil {
		return nil, err
	}
	royalties.Address = address

	if err := decodeTransferPercentage(buf, royalties); err != nil {
		return nil, err
	}

	if err := decodeFixedAndPercentages(buf, royalties); err != nil {
		return nil, err
	}

	if err := decodeSplitRoyalties(buf, royalties); err != nil {
		return nil, err
	}

	if err := decodeITOData(buf, royalties); err != nil {
		return nil, err
	}

	if buf.Len() != 0 {
		return nil, errors.New("extra bytes found in buffer")
	}

	return royalties, nil
}

func decodeAddress(buf *bytes.Reader) ([]byte, error) {
	address := make([]byte, 32)
	err := binary.Read(buf, binary.BigEndian, address)
	return address, err
}

func decodeTransferPercentage(buf *bytes.Reader, royalties *transaction.RoyaltiesInfo) error {
	var tpLength uint32
	if err := binary.Read(buf, binary.BigEndian, &tpLength); err != nil {
		return err
	}

	// prevent large length
	if tpLength > core.MaxTransferRoyalties {
		return common.ErrInvalidValue
	}

	for i := uint32(0); i < tpLength; i++ {
		tp := &transaction.RoyaltyInfo{}
		amount, err := readBigUint(buf)
		if err != nil {
			return err
		}
		tp.Amount = amount.Int64()

		if err := binary.Read(buf, binary.BigEndian, &tp.Percentage); err != nil {
			return err
		}
		royalties.TransferPercentage = append(royalties.TransferPercentage, tp)
	}
	return nil
}

func decodeFixedAndPercentages(buf *bytes.Reader, royalties *transaction.RoyaltiesInfo) error {
	transferFixed, err := readBigUint(buf)
	if err != nil {
		return err
	}
	royalties.TransferFixed = transferFixed.Int64()

	if err := binary.Read(buf, binary.BigEndian, &royalties.MarketPercentage); err != nil {
		return err
	}

	marketFixed, err := readBigUint(buf)
	if err != nil {
		return err
	}
	royalties.MarketFixed = marketFixed.Int64()

	return nil
}

func decodeSplitRoyalties(buf *bytes.Reader, royalties *transaction.RoyaltiesInfo) error {
	var srLength uint32
	if err := binary.Read(buf, binary.BigEndian, &srLength); err != nil {
		return err
	}

	// prevent large slipt royalties length
	if srLength > core.MaxTransferRoyalties {
		return common.ErrInvalidValue
	}

	royalties.SplitRoyalties = make(map[string]*transaction.RoyaltySplitInfo)

	for i := uint32(0); i < srLength; i++ {
		key, err := decodeAddress(buf)
		if err != nil {
			return err
		}

		encodedKey := hex.EncodeToString(key)
		splitInfo := &transaction.RoyaltySplitInfo{}

		if err := decodeSplitInfo(buf, splitInfo); err != nil {
			return err
		}

		royalties.SplitRoyalties[encodedKey] = splitInfo
	}

	return nil
}

func decodeSplitInfo(buf *bytes.Reader, splitInfo *transaction.RoyaltySplitInfo) error {
	fields := []*uint32{
		&splitInfo.PercentTransferPercentage,
		&splitInfo.PercentTransferFixed,
		&splitInfo.PercentMarketPercentage,
		&splitInfo.PercentMarketFixed,
		&splitInfo.PercentITOPercentage,
		&splitInfo.PercentITOFixed,
	}

	for _, field := range fields {
		if err := binary.Read(buf, binary.BigEndian, field); err != nil {
			return err
		}
	}

	return nil
}

func decodeITOData(buf *bytes.Reader, royalties *transaction.RoyaltiesInfo) error {
	if err := binary.Read(buf, binary.BigEndian, &royalties.ITOPercentage); err != nil {
		return err
	}

	itoFixed, err := readBigUint(buf)
	if err != nil {
		return err
	}
	royalties.ITOFixed = itoFixed.Int64()

	return nil
}

// EncodeOperationDataCheck encodes operations into a string.
// validate the length of operations
func EncodeOperationDataCheck(operations []byte) ([]byte, error) {
	if len(operations) > core.MaxOperationsSize {
		return nil, common.ErrInvalidPermission
	}

	return hex.DecodeString(encodeOperationData(operations))
}

func encodeOperationData(operations []byte) string {

	// Convert to hexadecimal string
	hexOperations := hex.EncodeToString(operations)

	// Convert each hex character to its ASCII equivalent
	var builder strings.Builder
	for _, ch := range hexOperations {
		builder.WriteString(fmt.Sprintf("%02x", ch))
	}

	// Calculate the length of hexOperations and convert to big-endian bytes
	lengthBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(lengthBytes, uint32(len(hexOperations))) // #nosec G115

	// Prefix the ASCII operations with the hex-encoded length
	return hex.EncodeToString(lengthBytes) + builder.String()
}

// EncodeAccountPermissionData encodes account permissions into a byte slice.
func EncodeAccountPermissionData(permissions []*transaction.AccPermission) ([]byte, error) {
	buf := new(bytes.Buffer)

	// Write the length of the permissions array
	length := uint32(len(permissions)) // #nosec G115
	if length > core.MaxAccountPermission {
		return nil, common.ErrInvalidPermission
	}

	err := writeUint32(buf, length)
	if err != nil {
		return nil, err
	}

	for _, permission := range permissions {
		if err := encodePermission(buf, permission); err != nil {
			return nil, err
		}
	}

	return buf.Bytes(), nil
}

// encodePermission encodes a single permission into the buffer.
func encodePermission(buf *bytes.Buffer, permission *transaction.AccPermission) error {
	// Write the permission type
	err := writeUint8(buf, uint8(permission.Type)) // #nosec G115
	if err != nil {
		return err
	}

	// Write the permission name
	if err := writeString(buf, permission.PermissionName); err != nil {
		return err
	}

	// Write the threshold
	err = binary.Write(buf, binary.BigEndian, permission.Threshold)
	if err != nil {
		return err
	}

	// Write the operations
	operationsHex := hex.EncodeToString(permission.Operations)
	if err := writeString(buf, operationsHex); err != nil {
		return err
	}

	// Write the signers
	if err := encodeSigners(buf, permission.Signers); err != nil {
		return err
	}

	return nil
}

// encodeSigners encodes the signers of a permission into the buffer.
func encodeSigners(buf *bytes.Buffer, signers []*transaction.AccKey) error {
	signersLen := uint32(len(signers)) // #nosec G115
	err := writeUint32(buf, signersLen)
	if err != nil {
		return err
	}

	for _, signer := range signers {
		if err := encodeSigner(buf, signer); err != nil {
			return err
		}
	}

	return nil
}

// encodeSigner encodes a single signer into the buffer.
func encodeSigner(buf *bytes.Buffer, signer *transaction.AccKey) error {
	if len(signer.Address) != 32 {
		return errors.New("invalid address length")
	}

	_, err := buf.Write(signer.Address)
	if err != nil {
		return err
	}

	err = binary.Write(buf, binary.BigEndian, signer.Weight)
	if err != nil {
		return err
	}

	return nil
}

// DecodeAccountPermissionData decodes account permissions from a byte slice.
func DecodeAccountPermissionData(data []byte) ([]*transaction.AccPermission, error) {
	buf := bytes.NewReader(data)

	length, err := readUint32(buf)
	if err != nil {
		return nil, err
	}

	// prevent large length
	if length > core.MaxAccountPermission {
		return nil, common.ErrInvalidPermission
	}

	permissions := make([]*transaction.AccPermission, 0, length)
	for i := uint32(0); i < length; i++ {
		permission, err := decodePermission(buf)
		if err != nil {
			return nil, err
		}
		permissions = append(permissions, permission)
	}

	return permissions, nil
}

// decodePermission decodes a single permission from the buffer.
func decodePermission(buf *bytes.Reader) (*transaction.AccPermission, error) {
	permission := &transaction.AccPermission{}

	permType, err := readUint8(buf)
	if err != nil {
		return nil, err
	}
	permission.Type = transaction.AccPermission_AccPermissionType(permType)

	name, err := readString(buf)
	if err != nil {
		return nil, err
	}
	permission.PermissionName = name

	err = binary.Read(buf, binary.BigEndian, &permission.Threshold)
	if err != nil {
		return nil, err
	}

	operationsHex, err := readString(buf)
	if err != nil {
		return nil, err
	}
	permission.Operations, err = hex.DecodeString(operationsHex)
	if err != nil {
		return nil, errors.New("error decoding operations")
	}

	if len(permission.Operations) > core.MaxOperationsSize {
		return nil, common.ErrInvalidPermission
	}

	signers, err := decodeSigners(buf)
	if err != nil {
		return nil, err
	}
	permission.Signers = signers

	return permission, nil
}

// decodeSigners decodes the signers of a permission from the buffer.
func decodeSigners(buf *bytes.Reader) ([]*transaction.AccKey, error) {
	signersLen, err := readUint32(buf)
	if err != nil {
		return nil, err
	}

	signers := make([]*transaction.AccKey, 0, signersLen)
	for i := uint32(0); i < signersLen; i++ {
		signer, err := decodeSigner(buf)
		if err != nil {
			return nil, err
		}
		signers = append(signers, signer)
	}

	return signers, nil
}

// decodeSigner decodes a single signer from the buffer.
func decodeSigner(buf *bytes.Reader) (*transaction.AccKey, error) {
	signer := &transaction.AccKey{}

	address, err := decodeAddress(buf)
	if err != nil {
		return nil, err
	}
	signer.Address = address

	err = binary.Read(buf, binary.BigEndian, &signer.Weight)
	if err != nil {
		return nil, err
	}

	return signer, nil
}

// writeUint32 writes a uint32 value to the buffer.
func writeUint32(buf *bytes.Buffer, value uint32) error {
	return binary.Write(buf, binary.BigEndian, value)
}

// writeUint8 writes a uint8 value to the buffer.
func writeUint8(buf *bytes.Buffer, value uint8) error {
	return binary.Write(buf, binary.BigEndian, value)
}

func writeInt64AsBiInt(buf *bytes.Buffer, valueInt int64) error {
	value := big.NewInt(valueInt)
	return writeBigInt(buf, value)
}

func writeBigInt(buf *bytes.Buffer, value *big.Int) error {
	valueBytes := value.Bytes()
	valueLen := uint32(len(valueBytes)) // #nosec G115

	if err := writeUint32(buf, valueLen); err != nil {
		return err
	}

	buf.Write(valueBytes)

	return nil
}

// writeString writes the length of the string followed by the string bytes.
func writeString(buf *bytes.Buffer, value string) error {
	valueBytes := []byte(value)
	valueLen := uint32(len(valueBytes)) // #nosec G115

	if err := writeUint32(buf, valueLen); err != nil {
		return err
	}

	_, err := buf.Write(valueBytes)
	return err
}

// readUint32 reads a uint32 value from the buffer.
func readUint32(buf *bytes.Reader) (uint32, error) {
	var value uint32
	err := binary.Read(buf, binary.BigEndian, &value)
	return value, err
}

// readUint8 reads a uint8 value from the buffer.
func readUint8(buf *bytes.Reader) (uint8, error) {
	var value uint8
	err := binary.Read(buf, binary.BigEndian, &value)
	return value, err
}

func readBytes(buf *bytes.Reader) ([]byte, error) {
	return readBytesMaxSize(buf, core.MaxBytesToDecode)
}

func readBytesMaxSize(buf *bytes.Reader, max uint32) ([]byte, error) {
	length, err := readUint32(buf)
	if err != nil {
		return nil, err
	}

	// prevent large allocations
	if length > max {
		return nil, common.ErrMaxBytesExceeded
	}

	value := make([]byte, length)
	err = binary.Read(buf, binary.BigEndian, value)
	if err != nil {
		return nil, err
	}

	return value, nil
}

func readBigUint(buf *bytes.Reader) (*big.Int, error) {
	var length uint32
	err := binary.Read(buf, binary.BigEndian, &length)
	if err != nil {
		return nil, err
	}

	// prevent large big int allocations
	if length > core.MaxBytesBigInt {
		return nil, common.ErrMaxBytesExceeded
	}

	valBytes := make([]byte, length)
	err = binary.Read(buf, binary.BigEndian, valBytes)
	if err != nil {
		return nil, err
	}
	return new(big.Int).SetBytes(valBytes), nil
}

// readString reads a string from the buffer by first reading its length.
func readString(buf *bytes.Reader) (string, error) {
	value, err := readBytes(buf)
	return string(value), err
}
