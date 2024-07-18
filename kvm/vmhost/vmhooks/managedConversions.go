package vmhooks

import (
	"encoding/binary"
	"errors"
	"math/big"

	"github.com/klever-io/klever-go/common/types"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/kvm/math"
	"github.com/klever-io/klever-go/kvm/vmhost"
	"github.com/klever-io/klever-go/vmcommon"
)

const kdaTransferLen = 16 // no royalties
const TransferPercentageLen = 8
const SplitRoyaltiesLen = 28
const RoyaltiesLen = 32
const RolesDataLen = 8
const UserBucketsLen = 24
const SFTMetaLen = 12
const SFTMetadataLen = 12
const URIsDataLen = 8

// Deserializes a *transaction.TXContract object.
func readKDATransfer(
	managedType vmhost.ManagedTypesContext,
	data []byte,
) (*vmcommon.KDATransfer, error) {
	if len(data) != kdaTransferLen {
		return nil, errors.New("invalid KDA transfer object encoding")
	}

	tokenIdentifierHandle := int32(binary.BigEndian.Uint32(data[0:4]))
	tokenIdentifier, err := managedType.GetBytes(tokenIdentifierHandle)
	if err != nil {
		return nil, err
	}
	managedType.ConsumeGasForBytes(tokenIdentifier)

	nonce := binary.BigEndian.Uint64(data[4:12])

	valueHandle := int32(binary.BigEndian.Uint32(data[12:16]))
	value, err := managedType.GetBigInt(valueHandle)
	if err != nil {
		return nil, err
	}

	managedType.ConsumeGasForBigIntCopy(value)

	tokenType := core.Fungible
	if nonce > 0 {
		tokenType = core.NonFungible
	}

	return &vmcommon.KDATransfer{
		KDATokenName:  tokenIdentifier,
		KDATokenType:  uint32(tokenType),
		KDATokenNonce: nonce,
		KDAValue:      value,
	}, nil
}

// Converts a managed buffer of serialized data to a slice of KDATransfer.
// The format is:
// - token identifier handle - 4 bytes
// - nonce - 8 bytes
// - value handle - 4 bytes
// Total: 16 bytes.
func readKDATransfers(
	managedType vmhost.ManagedTypesContext,
	managedVecHandle int32,
) ([]*vmcommon.KDATransfer, error) {
	managedVecBytes, err := managedType.GetBytes(managedVecHandle)
	if err != nil {
		return nil, err
	}
	managedType.ConsumeGasForBytes(managedVecBytes)

	if len(managedVecBytes)%kdaTransferLen != 0 {
		return nil, errors.New("invalid managed vector of KDA transfers")
	}

	numTransfers := len(managedVecBytes) / kdaTransferLen
	result := make([]*vmcommon.KDATransfer, 0, numTransfers)
	for i := 0; i < len(managedVecBytes); i += kdaTransferLen {
		kdaTransfer, err := readKDATransfer(managedType, managedVecBytes[i:i+kdaTransferLen])
		if err != nil {
			return nil, err
		}
		result = append(result, kdaTransfer)
	}

	return result, nil
}

// Serializes a vmcommon.KDATransfer object.
func writeKDATransfer(
	managedType vmhost.ManagedTypesContext,
	kdaTransfer *vmcommon.KDATransfer,
	destinationBytes []byte,
) {
	tokenIdentifierHandle := managedType.NewManagedBufferFromBytes(kdaTransfer.KDATokenName)
	valueHandle := managedType.NewBigInt(kdaTransfer.KDAValue)

	binary.BigEndian.PutUint32(destinationBytes[0:4], uint32(tokenIdentifierHandle))
	binary.BigEndian.PutUint64(destinationBytes[4:12], kdaTransfer.KDATokenNonce)
	binary.BigEndian.PutUint32(destinationBytes[12:16], uint32(valueHandle))
}

// Serializes a kapps.RoyaltySplitData object.
func writeSplitRoyalties(
	managedType vmhost.ManagedTypesContext,
	key string,
	sr *kapps.RoyaltySplitData,
	destinationBytes []byte,
) {
	keyHandle := managedType.NewManagedBufferFromBytes([]byte(key))
	binary.BigEndian.PutUint32(destinationBytes[0:4], uint32(keyHandle))
	binary.BigEndian.PutUint32(destinationBytes[4:8], sr.PercentTransferFixed)
	binary.BigEndian.PutUint32(destinationBytes[8:12], sr.PercentTransferPercentage)
	binary.BigEndian.PutUint32(destinationBytes[12:16], sr.PercentMarketFixed)
	binary.BigEndian.PutUint32(destinationBytes[16:20], sr.PercentMarketPercentage)
	binary.BigEndian.PutUint32(destinationBytes[20:24], sr.PercentITOFixed)
	binary.BigEndian.PutUint32(destinationBytes[24:28], sr.PercentITOPercentage)
}

// Serializes a list of SplitRoyalties one after the other into a byte slice.
// The format is (for each SplitRoyalties):
// - map key   				   - 4 bytes
// - PercentTransferPercentage - 4 bytes
// - PercentTransferFixed      - 4 bytes
// - PercentMarketPercentage   - 4 bytes
// - PercentMarketFixed        - 4 bytes
// - PercentITOPercentage      - 4 bytes
// - PercentITOFixed           - 4 bytes
// Total: 28 bytes.
func writeSplitRoyaltiesToBytes(
	managedType vmhost.ManagedTypesContext,
	splitRoyalties map[string]*kapps.RoyaltySplitData,
) []byte {
	destinationBytes := make([]byte, SplitRoyaltiesLen*len(splitRoyalties))
	dataIndex := 0

	dMap := types.NewDeterministicMap(splitRoyalties)

	dMap.Each(func(key string, value *kapps.RoyaltySplitData) error {
		writeSplitRoyalties(managedType, key, value, destinationBytes[dataIndex:dataIndex+SplitRoyaltiesLen])
		dataIndex += SplitRoyaltiesLen
		return nil
	})

	return destinationBytes
}

// Serializes a kapps.TransferPercentage object.
func writeTransferPercentages(
	managedType vmhost.ManagedTypesContext,
	tp *kapps.RoyaltyData,
	destinationBytes []byte,
) {
	amount := big.NewInt(tp.Amount)
	amountHandle := managedType.NewBigInt(amount)
	binary.BigEndian.PutUint32(destinationBytes[0:4], uint32(amountHandle))
	binary.BigEndian.PutUint32(destinationBytes[4:8], tp.Percentage)
}

// Serializes a list of TransferPercentage one after the other into a byte slice.
// The format is (for each TransferPercentage):
// - amount     - 4 bytes
// - percentage - 4 bytes
// Total: 8 bytes.
func writeTransferPercentagesToBytes(
	managedType vmhost.ManagedTypesContext,
	tps []*kapps.RoyaltyData,
) []byte {
	destinationBytes := make([]byte, TransferPercentageLen*len(tps))
	dataIndex := 0
	for _, tp := range tps {
		writeTransferPercentages(managedType, tp, destinationBytes[dataIndex:dataIndex+TransferPercentageLen])
		dataIndex += TransferPercentageLen
	}

	return destinationBytes
}

// Serializes LastClaim a byte slice.
// - Timestamp            - 4 bytes
// - Epoch                - 4 bytes
// Total: 8 bytes.
func writeLastClaim(
	managedType vmhost.ManagedTypesContext,
	lastClaim *kapps.LastClaim,
) []byte {
	destinationBytes := make([]byte, RoyaltiesLen)

	if lastClaim == nil {
		lastClaim = &kapps.LastClaim{
			Timestamp: 0,
			Epoch:     0,
		}
	}

	timestampHandle := managedType.NewBigIntFromInt64(lastClaim.Timestamp)

	binary.BigEndian.PutUint32(destinationBytes[0:4], uint32(timestampHandle))
	binary.BigEndian.PutUint32(destinationBytes[4:8], lastClaim.Epoch)

	return destinationBytes
}

func writeUserBuckets(
	managedType vmhost.ManagedTypesContext,
	buckets map[string]*kapps.UserBucket) []byte {
	destinationBytes := make([]byte, UserBucketsLen*len(buckets))
	dataIndex := 0

	dMap := types.NewDeterministicMap(buckets)

	dMap.Each(func(key string, value *kapps.UserBucket) error {
		writeUserBucket(managedType, key, value, destinationBytes[dataIndex:dataIndex+UserBucketsLen])
		dataIndex += UserBucketsLen
		return nil
	})

	return destinationBytes
}

// Serializes Royalties a byte slice.
// - Key           - 4 bytes
// - StakedAt      - 4 bytes
// - StakedEpoch   - 4 bytes
// - UnstakedEpoch - 4 bytes
// - Value         - 4 bytes
// - Delegation    - 4 bytes
// Total: 24 bytes.
func writeUserBucket(managedType vmhost.ManagedTypesContext, key string, bucket *kapps.UserBucket, destinationBytes []byte) {
	keyHandle := managedType.NewManagedBufferFromBytes([]byte(key))
	stakedAtHandle := managedType.NewBigIntFromInt64(bucket.StakedAt)
	valueHandle := managedType.NewBigIntFromInt64(bucket.Value)
	delegationHandle := managedType.NewManagedBufferFromBytes(bucket.Delegation)

	binary.BigEndian.PutUint32(destinationBytes[0:4], uint32(keyHandle))
	binary.BigEndian.PutUint32(destinationBytes[4:8], uint32(stakedAtHandle))
	binary.BigEndian.PutUint32(destinationBytes[8:12], bucket.StakedEpoch)
	binary.BigEndian.PutUint32(destinationBytes[12:16], bucket.UnstakedEpoch)
	binary.BigEndian.PutUint32(destinationBytes[16:20], uint32(valueHandle))
	binary.BigEndian.PutUint32(destinationBytes[20:24], uint32(delegationHandle))
}

// Serializes Royalties a byte slice.
// - Address            - 4 bytes
// - TransferPercentage - 4 bytes
// - TransferFixed      - 4 bytes
// - MarketPercentage   - 4 bytes
// - MarketFixed        - 4 bytes
// - SplitRoyalties     - 4 bytes
// - ITOFixed           - 4 bytes
// - ITOPercentage      - 4 bytes
// Total: 32 bytes.
func writeRoyaltiesToBytes(
	managedType vmhost.ManagedTypesContext,
	royalties *kapps.RoyaltiesData,
) []byte {
	if royalties == nil {
		return make([]byte, 0)
	}

	destinationBytes := make([]byte, RoyaltiesLen)
	addressHandle := managedType.NewManagedBufferFromBytes(royalties.Address)
	transferPercentageHandle := managedType.NewManagedBufferFromBytes(writeTransferPercentagesToBytes(managedType, royalties.TransferPercentage))
	transferFixedHandle := managedType.NewBigIntFromInt64(royalties.TransferFixed)
	marketFixedHandle := managedType.NewBigIntFromInt64(royalties.MarketFixed)
	splitRoyaltiesHandle := managedType.NewManagedBufferFromBytes(writeSplitRoyaltiesToBytes(managedType, royalties.SplitRoyalties))
	itoFixedHandle := managedType.NewBigIntFromInt64(royalties.ITOFixed)

	binary.BigEndian.PutUint32(destinationBytes[0:4], uint32(addressHandle))
	binary.BigEndian.PutUint32(destinationBytes[4:8], uint32(transferPercentageHandle))
	binary.BigEndian.PutUint32(destinationBytes[8:12], uint32(transferFixedHandle))
	binary.BigEndian.PutUint32(destinationBytes[12:16], royalties.MarketPercentage)
	binary.BigEndian.PutUint32(destinationBytes[16:20], uint32(marketFixedHandle))
	binary.BigEndian.PutUint32(destinationBytes[20:24], uint32(splitRoyaltiesHandle))
	binary.BigEndian.PutUint32(destinationBytes[24:28], uint32(itoFixedHandle))
	binary.BigEndian.PutUint32(destinationBytes[28:32], royalties.ITOPercentage)

	return destinationBytes
}

func encodeBool(value bool, index int) uint32 {
	if value {
		return uint32(1) << index
	}
	return 0
}

// Serializes Properties
func getPropertiesValue(properties *kapps.PropertiesData, tokenType int32) uint32 {
	value := uint32(0)

	value += encodeBool(properties.CanFreeze, 0)
	value += encodeBool(properties.CanWipe, 1)
	value += encodeBool(properties.CanPause, 2)
	value += encodeBool(properties.CanMint, 3)
	value += encodeBool(properties.CanBurn, 4)
	value += encodeBool(properties.CanChangeOwner, 5)
	value += encodeBool(properties.CanAddRoles, 6)
	value += encodeBool(properties.LimitTransfer, 7)

	// convert tokenType from int32 into 2bits and add to value bits 30 and 31 masked
	value += uint32(tokenType) << 30

	return value
}

// Serializes Attributes
func getAttributesValue(attributes *kapps.AttributesData) uint32 {
	value := uint32(0)

	value += encodeBool(attributes.IsPaused, 0)
	value += encodeBool(attributes.IsNFTMintStopped, 1)
	value += encodeBool(attributes.IsRoyaltiesChangeStopped, 2)
	value += encodeBool(attributes.IsNFTMetadataChangeStopped, 3)

	return value
}

// Serializes Roles to a byte slice.
// Address              - 4 bytes
// booleans	            - 4 bytes
// Total: 8 bytes.
func writeRolesToBytes(managedType vmhost.ManagedTypesContext, roles []*kapps.RolesData) []byte {
	destinationBytes := make([]byte, RolesDataLen*len(roles))
	dataIndex := 0
	for _, at := range roles {
		writeRoles(managedType, at, destinationBytes[dataIndex:dataIndex+RolesDataLen])
		dataIndex += RolesDataLen
	}

	return destinationBytes
}

// Serializes Uris to a byte slice.
// Key             - 4 bytes
// Value           - 4 bytes
// Total: 8 bytes.
func writeURIsToBytes(managedType vmhost.ManagedTypesContext, uris map[string]string) []byte {
	destinationBytes := make([]byte, URIsDataLen*len(uris))
	dataIndex := 0

	dMap := types.NewDeterministicMap(uris)

	dMap.Each(func(key string, value string) error {
		writeURIs(managedType, key, value, destinationBytes[dataIndex:dataIndex+URIsDataLen])
		dataIndex += URIsDataLen
		return nil
	})

	return destinationBytes
}

// Serializes Metadatav2 to a byte slice.
// Circulation      - 4 bytes
// Supply           - 4 bytes
// Metadata         - 4 bytes
// Total: 12 bytes.
func writeSFTMeta(managedType vmhost.ManagedTypesContext, meta *kapps.MetaV2) []byte {
	destinationBytes := make([]byte, SFTMetaLen)

	maxSupplyHandle := managedType.NewBigIntFromInt64(meta.MaxSupply)
	binary.BigEndian.PutUint32(destinationBytes[0:4], uint32(maxSupplyHandle))

	circulationSupplyHandle := managedType.NewBigIntFromInt64(meta.Circulation)
	binary.BigEndian.PutUint32(destinationBytes[4:8], uint32(circulationSupplyHandle))

	metadata := writeSFTMetadata(managedType, meta.Metadata)
	metaHandle := managedType.NewManagedBufferFromBytes(metadata)
	binary.BigEndian.PutUint32(destinationBytes[8:12], uint32(metaHandle))

	return destinationBytes
}

// Serializes Metadatav2 to a byte slice.
// Name             - 4 bytes
// Hash           	- 4 bytes
// Attributes       - 4 bytes
// Total: 12 bytes.
func writeSFTMetadata(managedType vmhost.ManagedTypesContext, metadata *kapps.MetaV2Data) []byte {
	destinationBytes := make([]byte, SFTMetadataLen)

	nameHandle := managedType.NewManagedBufferFromBytes(metadata.Name)
	binary.BigEndian.PutUint32(destinationBytes[0:4], uint32(nameHandle))

	hashHandle := managedType.NewManagedBufferFromBytes(metadata.Hash)
	binary.BigEndian.PutUint32(destinationBytes[4:8], uint32(hashHandle))

	attriHandle := managedType.NewManagedBufferFromBytes(metadata.Attributes)
	binary.BigEndian.PutUint32(destinationBytes[8:12], uint32(attriHandle))

	return destinationBytes
}

func writeURIs(managedType vmhost.ManagedTypesContext, key, value string, destinationBytes []byte) {
	keyHandle := managedType.NewManagedBufferFromBytes([]byte(key))
	binary.BigEndian.PutUint32(destinationBytes[0:4], uint32(keyHandle))

	valueHandle := managedType.NewManagedBufferFromBytes([]byte(value))
	binary.BigEndian.PutUint32(destinationBytes[4:8], uint32(valueHandle))
}

func writeRoles(managedType vmhost.ManagedTypesContext, role *kapps.RolesData, destinationBytes []byte) {
	addressHandle := managedType.NewManagedBufferFromBytes(role.Address)
	binary.BigEndian.PutUint32(destinationBytes[0:4], uint32(addressHandle))

	value := uint32(0)

	value += encodeBool(role.HasRoleMint, 0)
	value += encodeBool(role.HasRoleSetITOPrices, 1)
	value += encodeBool(role.HasRoleDeposit, 2)
	value += encodeBool(role.HasRoleTransfer, 3)

	binary.BigEndian.PutUint32(destinationBytes[4:8], value)
}

// Serializes a list of KDATransfer one after the other into a byte slice.
// The format is (for each KDATransfer):
// - token identifier handle - 4 bytes
// - nonce - 8 bytes
// - value handle - 4 bytes
// - royalties handle - 4 bytes
// Total: 20 bytes.
func writeKDATransfersToBytes(
	managedType vmhost.ManagedTypesContext,
	kdaTransfers []*vmcommon.KDATransfer,
) []byte {
	destinationBytes := make([]byte, kdaTransferLen*len(kdaTransfers))
	dataIndex := 0
	for _, kdaTransfer := range kdaTransfers {
		writeKDATransfer(managedType, kdaTransfer, destinationBytes[dataIndex:dataIndex+kdaTransferLen])
		dataIndex += kdaTransferLen
	}

	return destinationBytes
}

type vmInputData struct {
	destination []byte
	function    string
	value       *big.Int
	arguments   [][]byte
}

func readDestinationValueFunctionArguments(
	host vmhost.VMHost,
	destHandle int32,
	valueHandle int32,
	functionHandle int32,
	argumentsHandle int32,
) (*vmInputData, error) {
	managedType := host.ManagedTypes()

	vmInput, err := readDestinationValueArguments(host, destHandle, valueHandle, argumentsHandle)
	if err != nil {
		return nil, err
	}

	function, err := managedType.GetBytes(functionHandle)
	if err != nil {
		return nil, err
	}
	vmInput.function = string(function)

	return vmInput, err
}

func readDestinationValueArguments(
	host vmhost.VMHost,
	destHandle int32,
	valueHandle int32,
	argumentsHandle int32,
) (*vmInputData, error) {
	managedType := host.ManagedTypes()

	vmInput, err := readDestinationArguments(host, destHandle, argumentsHandle)
	if err != nil {
		return nil, err
	}

	vmInput.value, err = managedType.GetBigInt(valueHandle)
	if err != nil {
		return nil, err
	}

	return vmInput, err
}

func readDestinationFunctionArguments(
	host vmhost.VMHost,
	destHandle int32,
	functionHandle int32,
	argumentsHandle int32,
) (*vmInputData, error) {
	managedType := host.ManagedTypes()

	vmInput, err := readDestinationArguments(host, destHandle, argumentsHandle)
	if err != nil {
		return nil, err
	}

	function, err := managedType.GetBytes(functionHandle)
	if err != nil {
		return nil, err
	}
	vmInput.function = string(function)

	return vmInput, err
}

func readDestinationArguments(
	host vmhost.VMHost,
	destHandle int32,
	argumentsHandle int32,
) (*vmInputData, error) {
	managedType := host.ManagedTypes()
	metering := host.Metering()

	var err error
	vmInput := &vmInputData{}

	vmInput.destination, err = managedType.GetBytes(destHandle)
	if err != nil {
		return nil, err
	}

	vmInput.value = big.NewInt(0)
	data, actualLen, err := managedType.ReadManagedVecOfManagedBuffers(argumentsHandle)
	if err != nil {
		return nil, err
	}
	vmInput.arguments = data

	gasToUse := math.MulUint64(metering.GasSchedule().BaseOperationCost.DataCopyPerByte, actualLen)
	metering.UseAndTraceGas(gasToUse)

	return vmInput, err
}
