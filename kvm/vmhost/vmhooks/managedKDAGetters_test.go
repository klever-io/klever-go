package vmhooks_test

import (
	"bytes"
	"fmt"
	"math/big"
	"testing"

	commonMock "github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/smartContract/hooks/counters"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/kvm/config"
	contextmock "github.com/klever-io/klever-go/kvm/mock/context"
	worldmock "github.com/klever-io/klever-go/kvm/mock/world"
	"github.com/klever-io/klever-go/kvm/vmhost"
	hostmock "github.com/klever-io/klever-go/kvm/vmhost/mock"
	"github.com/klever-io/klever-go/kvm/vmhost/vmhooks"
	"github.com/klever-io/klever-go/vmcommon"
	"github.com/klever-io/klever-go/vmcommon/parsers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// kdaGetterGasMap is perByteHookGasMap with distinct fixed costs for the KDA getters, so a getter
// charging another's cost fails
func kdaGetterGasMap() config.GasScheduleMap {
	gasMap := perByteHookGasMap()
	gasMap["BaseOpsAPICost"]["GetCallValue"] = 97
	gasMap["BaseOpsAPICost"]["GetUserKDA"] = 101
	gasMap["BaseOpsAPICost"]["GetKDATokenData"] = 103
	gasMap["BaseOpsAPICost"]["GetKDARoles"] = 107
	gasMap["BaseOpsAPICost"]["GetSFTMetadata"] = 109
	// not a multiple of DataCopyPerByte, so an oversize big int byte cannot pass for copied bytes
	gasMap["BigIntAPICost"]["CopyPerByteForTooBig"] = 1_009
	return gasMap
}

const (
	// kdaGetterOut is the first output handle: far above anything a builder creates, so the
	// outputs never land on a handle the hook allocated itself
	kdaGetterOut = int32(1 << 30)
	// kdaGetterBudget is the gas left for a call that is meant to succeed
	kdaGetterBudget = uint64(5_000_000_000)
)

var (
	kdaGetterAddress = bytes.Repeat([]byte{'a'}, 32)
	kdaGetterTicker  = []byte("TKN-ABCD")
)

// kdaGetterCase is one hook called on one shape of the data it reads
type kdaGetterCase struct {
	name string
	// install puts the shape where the hook reads it: the call input, the back transfers or the
	// blockchain stub
	install func(host *contextmock.VMHostMock)
	call    func(hooks *vmhooks.VMHooksImpl, addressHandle, tickerHandle int32)
	fixed   uint64
	// copyBytes is what the hook charges DataCopyPerByte for on both sides of the fork
	copyBytes uint64
	// tooBigBytes is the big int length it charges CopyPerByteForTooBig for on both sides
	tooBigBytes uint64
	// v6TooBigBytes is the big int length it charges CopyPerByteForTooBig for only from
	// FixAuditChangesV6: the transfer values above 32 bytes it copies into new handles
	v6TooBigBytes uint64
	// inputBytes is the address and ticker it copies, charged DataCopyPerByte from FixAuditChangesV6
	// only (KLC-2719), before the outputs
	inputBytes uint64
	// v6OutputBytes is the outputs it charges DataCopyPerByte for only from FixAuditChangesV6
	v6OutputBytes uint64
	// v6NameBytes is the token names of the transfers a call value getter copies out, charged
	// DataCopyPerByte only from FixAuditChangesV6, together with the encoded transfers
	v6NameBytes uint64
	// v6DecodeBytes is the encoded size of the records GetKDAToken decoded, charged DataCopyPerByte
	// only from FixAuditChangesV6, right after the read and before the outputs
	v6DecodeBytes uint64
	bufferOuts    []int32
	bigIntOuts    []int32
	// large marks a shape whose build allocates far more than flatCloneNoCloneBelow
	large bool
	// buildsBeforeCharge marks a hook whose read allocates in proportion to the shape before any
	// charge can be computed (GetBackTransfers clones the transfers), so only the handle probes
	// can tell that nothing was built
	buildsBeforeCharge bool
}

func (c kdaGetterCase) gas(gasMap config.GasScheduleMap, fixV6 bool) uint64 {
	perByte := gasMap["BaseOperationCost"]["DataCopyPerByte"]
	total := c.fixed + perByte*c.copyBytes + gasMap["BigIntAPICost"]["CopyPerByteForTooBig"]*c.tooBigBytes
	if fixV6 {
		total += perByte * (c.inputBytes + c.v6OutputBytes + c.v6DecodeBytes + c.v6NameBytes)
		total += gasMap["BigIntAPICost"]["CopyPerByteForTooBig"] * c.v6TooBigBytes
	}
	return total
}

func kdaGetterOuts(n int, offset int32) []int32 {
	outs := make([]int32, n)
	for i := range outs {
		outs[i] = kdaGetterOut + offset + int32(i) // #nosec G115
	}
	return outs
}

// kdaGetterTransfers is n transfers that mix every name the KLV filter looks at
func kdaGetterTransfers(n int) []*vmcommon.KDATransfer {
	transfers := make([]*vmcommon.KDATransfer, n)
	for i := range transfers {
		var name []byte
		switch i % 5 {
		case 1:
			name = []byte("KLV")
		case 2:
			name = []byte{}
		case 3:
			name = nil
		default:
			name = []byte(fmt.Sprintf("TKN-%06d", i))
		}
		transfers[i] = &vmcommon.KDATransfer{
			KDATokenName:  name,
			KDATokenNonce: uint64(i), // #nosec G115
			KDAValue:      big.NewInt(int64(i) * 1_000_003),
		}
	}
	return transfers
}

// kdaGetterValueTransfers is kdaGetterTransfers(len(sizes)) with the value of each transfer replaced by
// one of sizes[i] bytes: kept as it is for 0, nil for -1
func kdaGetterValueTransfers(sizes []int) []*vmcommon.KDATransfer {
	transfers := kdaGetterTransfers(len(sizes))
	for i, size := range sizes {
		switch {
		case size < 0:
			transfers[i].KDAValue = nil
		case size > 0:
			transfers[i].KDAValue = new(big.Int).SetBytes(kdaGetterBytes(size, 0xCD))
		}
	}
	return transfers
}

// kdaGetterTooBigValues is the length of the transfer values above 32 bytes, each on its own, of the
// transfers a getter copies: all of them, or for withoutKLV only those it keeps
func kdaGetterTooBigValues(transfers []*vmcommon.KDATransfer, withoutKLV bool) uint64 {
	total := uint64(0)
	for _, transfer := range transfers {
		kept := len(transfer.KDATokenName) > 0 && !bytes.Equal(transfer.KDATokenName, []byte("KLV"))
		if withoutKLV && !kept || transfer.KDAValue == nil {
			continue
		}
		if byteLen := (transfer.KDAValue.BitLen() + 7) / 8; byteLen > 32 {
			total += uint64(byteLen) // #nosec G115
		}
	}
	return total
}

// kdaGetterNameBytes is the length of the token names of the transfers a getter copies: all of them,
// or for withoutKLV only those it keeps
func kdaGetterNameBytes(transfers []*vmcommon.KDATransfer, withoutKLV bool) uint64 {
	total := uint64(0)
	for _, transfer := range transfers {
		kept := len(transfer.KDATokenName) > 0 && !bytes.Equal(transfer.KDATokenName, []byte("KLV"))
		if withoutKLV && !kept {
			continue
		}
		total += uint64(len(transfer.KDATokenName)) // #nosec G115
	}
	return total
}

func kdaGetterNonKLV(transfers []*vmcommon.KDATransfer) uint64 {
	kept := uint64(0)
	for _, transfer := range transfers {
		if len(transfer.KDATokenName) > 0 && !bytes.Equal(transfer.KDATokenName, []byte("KLV")) {
			kept++
		}
	}
	return kept
}

func kdaGetterSetInput(host *contextmock.VMHostMock, transfers []*vmcommon.KDATransfer) {
	runtime := host.RuntimeContext.(*contextmock.RuntimeContextMock)
	if runtime.VMInput == nil {
		runtime.VMInput = &vmcommon.ContractCallInput{}
	}
	runtime.VMInput.KDATransfers = transfers
}

func kdaGetterStubToken(host *contextmock.VMHostMock, kda *kapps.KDAData, user *kapps.UserKDA) {
	blockchain := host.BlockchainContext.(*hostmock.BlockchainContextStub)
	blockchain.GetKDATokenCalled = func(_ []byte, _ []byte, _ uint64) (*kapps.KDAData, *kapps.UserKDA, error) {
		return kda, user, nil
	}
}

func kdaGetterBytes(n int, fill byte) []byte {
	return bytes.Repeat([]byte{fill}, n)
}

func kdaGetterBuckets(n, keyLen, delegationLen int) (map[string]*kapps.UserBucket, uint64) {
	buckets := make(map[string]*kapps.UserBucket, n)
	charged := uint64(0)
	for i := 0; i < n; i++ {
		key := fmt.Sprintf("%0*d", keyLen, i)
		buckets[key] = &kapps.UserBucket{
			StakedAt:      int64(i),
			StakedEpoch:   uint32(i),     // #nosec G115
			UnstakedEpoch: uint32(i + 1), // #nosec G115
			Value:         int64(i) * 7,
			Delegation:    kdaGetterBytes(delegationLen, 'd'),
		}
		charged += uint64(vmhooks.UserBucketsLen + len(key) + delegationLen) // #nosec G115
	}
	return buckets, charged
}

func kdaGetterRoles(n, addressLen int) ([]*kapps.RolesData, uint64) {
	roles := make([]*kapps.RolesData, n)
	for i := range roles {
		roles[i] = &kapps.RolesData{
			Address:         kdaGetterBytes(addressLen, byte('A'+i%26)),
			HasRoleMint:     i%2 == 0,
			HasRoleTransfer: i%3 == 0,
		}
	}
	return roles, uint64(n * (vmhooks.RolesDataLen + addressLen)) // #nosec G115
}

func kdaGetterURIs(n, keyLen, valueLen int) (map[string]string, uint64) {
	uris := make(map[string]string, n)
	for i := 0; i < n; i++ {
		uris[fmt.Sprintf("%0*d", keyLen, i)] = string(kdaGetterBytes(valueLen, 'u'))
	}
	return uris, uint64(n * (vmhooks.URIsDataLen + keyLen + valueLen)) // #nosec G115
}

func kdaGetterRoyalties(addressLen, transferPercentages, splits, splitKeyLen int) (*kapps.RoyaltiesData, uint64) {
	royalties := &kapps.RoyaltiesData{
		Address:          kdaGetterBytes(addressLen, 'r'),
		TransferFixed:    11,
		MarketPercentage: 12,
		MarketFixed:      13,
		ITOPercentage:    14,
		ITOFixed:         15,
		SplitRoyalties:   make(map[string]*kapps.RoyaltySplitData, splits),
	}
	for i := 0; i < transferPercentages; i++ {
		royalties.TransferPercentage = append(royalties.TransferPercentage,
			&kapps.RoyaltyData{Amount: int64(i) * 1_000_000_007, Percentage: uint32(i)}) // #nosec G115
	}
	for i := 0; i < splits; i++ {
		royalties.SplitRoyalties[fmt.Sprintf("%0*d", splitKeyLen, i)] = &kapps.RoyaltySplitData{
			PercentTransferPercentage: uint32(i), // #nosec G115
			PercentITOFixed:           7,
		}
	}
	charged := vmhooks.RoyaltiesLen + addressLen + vmhooks.TransferPercentageLen*transferPercentages +
		splits*(vmhooks.SplitRoyaltiesLen+splitKeyLen)
	return royalties, uint64(charged) // #nosec G115
}

func kdaGetterCallValueCases(gasMap config.GasScheduleMap) []kdaGetterCase {
	fixed := gasMap["BaseOpsAPICost"]["GetCallValue"]
	out := kdaGetterOuts(1, 0)
	cases := make([]kdaGetterCase, 0)

	for _, n := range []int{0, 1, 3, 5, 257, 20_000} {
		transfers := kdaGetterTransfers(n)
		cases = append(cases,
			kdaGetterCase{
				name:    fmt.Sprintf("ManagedGetMultiKDACallValue/transfers=%d", n),
				install: func(host *contextmock.VMHostMock) { kdaGetterSetInput(host, transfers) },
				call: func(hooks *vmhooks.VMHooksImpl, _, _ int32) {
					hooks.ManagedGetMultiKDACallValue(out[0])
				},
				fixed:       fixed,
				copyBytes:   uint64(16 * n), // #nosec G115
				v6NameBytes: kdaGetterNameBytes(transfers, false),
				bufferOuts:  out,
				large:       n >= 20_000,
			},
			kdaGetterCase{
				name:    fmt.Sprintf("ManagedGetMultiKDAWithoutKLVCallValue/transfers=%d", n),
				install: func(host *contextmock.VMHostMock) { kdaGetterSetInput(host, transfers) },
				call: func(hooks *vmhooks.VMHooksImpl, _, _ int32) {
					hooks.ManagedGetMultiKDAWithoutKLVCallValue(out[0])
				},
				fixed:       fixed,
				copyBytes:   16 * kdaGetterNonKLV(transfers),
				v6NameBytes: kdaGetterNameBytes(transfers, true),
				bufferOuts:  out,
				large:       n >= 20_000,
			},
		)
	}

	maxNormal := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 32*8), big.NewInt(1))
	values := []struct {
		name        string
		value       *big.Int
		tooBigBytes uint64
	}{
		{"zero", big.NewInt(0), 0},
		{"small", big.NewInt(123_456), 0},
		{"32Bytes", maxNormal, 0},
		{"33Bytes", new(big.Int).Add(maxNormal, big.NewInt(1)), 33},
		{"4096Bytes", new(big.Int).SetBytes(kdaGetterBytes(4096, 0xAB)), 4096},
	}
	for _, n := range []int{0, 3, 20_000} {
		for _, v := range values {
			transfers := kdaGetterTransfers(n)
			value := v.value
			cases = append(cases, kdaGetterCase{
				name: fmt.Sprintf("ManagedGetBackTransfers/transfers=%d/value=%s", n, v.name),
				install: func(host *contextmock.VMHostMock) {
					host.ManagedTypes().AddBackTransfers(transfers)
					host.ManagedTypes().AddValueOnlyBackTransfer(value)
				},
				call: func(hooks *vmhooks.VMHooksImpl, _, _ int32) {
					hooks.ManagedGetBackTransfers(kdaGetterOut, kdaGetterOut+1)
				},
				fixed:              fixed,
				copyBytes:          uint64(16 * n), // #nosec G115
				tooBigBytes:        v.tooBigBytes,
				v6NameBytes:        kdaGetterNameBytes(transfers, false),
				bufferOuts:         []int32{kdaGetterOut},
				bigIntOuts:         []int32{kdaGetterOut + 1},
				large:              n >= 20_000,
				buildsBeforeCharge: true,
			})
		}
	}

	// the transfer values are copied into new big int handles: from FixAuditChangesV6 one above 32
	// bytes pays CopyPerByteForTooBig a byte, on its own, before anything is built. The names follow
	// kdaGetterTransfers: TKN, KLV, empty, nil, TKN, ...
	valueShapes := []struct {
		name  string
		sizes []int
		// firstNameLen, when set, replaces the first transfer's name, a kept one, with one this long:
		// longer than a record, so a name charge that overdrew ahead of the bounded record charge
		// would show
		firstNameLen int
	}{
		// a kept 33 byte value, oversize KLV, empty and nil named values, a free 32 byte one, a kept
		// 4096 byte one and a kept nil one
		{"oversize", []int{33, 40, 50, 60, 32, 4096, 0, 0, 0, -1}, 0},
		// only the transfers ManagedGetMultiKDAWithoutKLVCallValue leaves out are oversize
		{"oversizeLeftOut", []int{0, 40, 50, 60, 0}, 0},
		{"exactly32", []int{32, 32, 32, 32, 32}, 0},
		{"nilValues", []int{-1, -1, -1, -1, -1}, 0},
		{"longName", []int{0, 0, 0}, 4096},
	}
	for _, shape := range valueShapes {
		transfers := kdaGetterValueTransfers(shape.sizes)
		if shape.firstNameLen > 0 {
			transfers[0].KDATokenName = bytes.Repeat([]byte{'N'}, shape.firstNameLen)
		}
		n := len(transfers)
		cases = append(cases,
			kdaGetterCase{
				name:    "ManagedGetMultiKDACallValue/values=" + shape.name,
				install: func(host *contextmock.VMHostMock) { kdaGetterSetInput(host, transfers) },
				call: func(hooks *vmhooks.VMHooksImpl, _, _ int32) {
					hooks.ManagedGetMultiKDACallValue(out[0])
				},
				fixed:         fixed,
				copyBytes:     uint64(16 * n), // #nosec G115
				v6TooBigBytes: kdaGetterTooBigValues(transfers, false),
				v6NameBytes:   kdaGetterNameBytes(transfers, false),
				bufferOuts:    out,
			},
			kdaGetterCase{
				name:    "ManagedGetMultiKDAWithoutKLVCallValue/values=" + shape.name,
				install: func(host *contextmock.VMHostMock) { kdaGetterSetInput(host, transfers) },
				call: func(hooks *vmhooks.VMHooksImpl, _, _ int32) {
					hooks.ManagedGetMultiKDAWithoutKLVCallValue(out[0])
				},
				fixed:         fixed,
				copyBytes:     16 * kdaGetterNonKLV(transfers),
				v6TooBigBytes: kdaGetterTooBigValues(transfers, true),
				v6NameBytes:   kdaGetterNameBytes(transfers, true),
				bufferOuts:    out,
			},
		)
		for _, v := range values[2:4] {
			value := v.value
			cases = append(cases, kdaGetterCase{
				name: fmt.Sprintf("ManagedGetBackTransfers/values=%s/value=%s", shape.name, v.name),
				install: func(host *contextmock.VMHostMock) {
					host.ManagedTypes().AddBackTransfers(transfers)
					host.ManagedTypes().AddValueOnlyBackTransfer(value)
				},
				call: func(hooks *vmhooks.VMHooksImpl, _, _ int32) {
					hooks.ManagedGetBackTransfers(kdaGetterOut, kdaGetterOut+1)
				},
				fixed:              fixed,
				copyBytes:          uint64(16 * n), // #nosec G115
				tooBigBytes:        v.tooBigBytes,
				v6TooBigBytes:      kdaGetterTooBigValues(transfers, false),
				v6NameBytes:        kdaGetterNameBytes(transfers, false),
				bufferOuts:         []int32{kdaGetterOut},
				bigIntOuts:         []int32{kdaGetterOut + 1},
				buildsBeforeCharge: true,
			})
		}
	}

	return cases
}

func kdaGetterUserKDACases(gasMap config.GasScheduleMap) []kdaGetterCase {
	fixed := gasMap["BaseOpsAPICost"]["GetUserKDA"]
	bigIntOuts := kdaGetterOuts(2, 0)
	bufferOuts := kdaGetterOuts(4, 2)
	smallBuckets, smallBucketBytes := kdaGetterBuckets(3, 5, 7)
	largeBuckets, largeBucketBytes := kdaGetterBuckets(2_000, 64, 32)
	noDelegation, noDelegationBytes := kdaGetterBuckets(4, 1, 0)

	shapes := []struct {
		name    string
		user    *kapps.UserKDA
		charged uint64
		large   bool
	}{
		{"empty", &kapps.UserKDA{}, 0, false},
		{"balancesOnly", &kapps.UserKDA{Balance: 5, FrozenBalance: 6, LastClaim: &kapps.LastClaim{Timestamp: 7, Epoch: 8}}, 0, false},
		{"mimeAndMetadata", &kapps.UserKDA{MIME: []byte("image/png"), Metadata: kdaGetterBytes(300, 'm')}, 9 + 300, false},
		{"smallBuckets", &kapps.UserKDA{Balance: 1, Buckets: smallBuckets, MIME: []byte("x")}, smallBucketBytes + 1, false},
		{"bucketsWithoutDelegation", &kapps.UserKDA{Buckets: noDelegation}, noDelegationBytes, false},
		{"large", &kapps.UserKDA{
			Balance:  1 << 40,
			Buckets:  largeBuckets,
			MIME:     kdaGetterBytes(1024, 'i'),
			Metadata: kdaGetterBytes(256*1024, 'm'),
		}, largeBucketBytes + 1024 + 256*1024, true},
	}

	cases := make([]kdaGetterCase, 0, len(shapes))
	for _, shape := range shapes {
		user := shape.user
		cases = append(cases, kdaGetterCase{
			name: "ManagedGetUserKDA/" + shape.name,
			install: func(host *contextmock.VMHostMock) {
				kdaGetterStubToken(host, &kapps.KDAData{}, user)
			},
			call: func(hooks *vmhooks.VMHooksImpl, addressHandle, tickerHandle int32) {
				hooks.ManagedGetUserKDA(addressHandle, tickerHandle, 0,
					bigIntOuts[0], bigIntOuts[1], bufferOuts[0], bufferOuts[1], bufferOuts[2], bufferOuts[3])
			},
			fixed:         fixed,
			copyBytes:     shape.charged,
			inputBytes:    uint64(len(kdaGetterAddress) + len(kdaGetterTicker)),
			v6DecodeBytes: vmhooks.KDATokenDecodedLen(&kapps.KDAData{}, user),
			bufferOuts:    bufferOuts,
			bigIntOuts:    bigIntOuts,
			large:         shape.large,
		})
	}
	return cases
}

func kdaGetterTokenDataCases(gasMap config.GasScheduleMap) []kdaGetterCase {
	fixed := gasMap["BaseOpsAPICost"]["GetKDATokenData"]
	// precision, id, name, creator, admin, logo, uris, initialSupply, circulatingSupply, maxSupply,
	// minted, burned, royalties, properties, attributes, roles, issueDate
	h := kdaGetterOuts(17, 0)
	bufferOuts := []int32{h[1], h[2], h[3], h[4], h[5], h[6], h[12], h[15]}
	bigIntOuts := []int32{h[0], h[7], h[8], h[9], h[10], h[11], h[13], h[14], h[16]}

	smallRoles, smallRoleBytes := kdaGetterRoles(2, 1)
	largeRoles, largeRoleBytes := kdaGetterRoles(2_000, 32)
	smallURIs, smallURIBytes := kdaGetterURIs(3, 1, 1)
	largeURIs, largeURIBytes := kdaGetterURIs(2_000, 16, 64)
	emptyRoyalties, emptyRoyaltyBytes := kdaGetterRoyalties(0, 0, 0, 0)
	smallRoyalties, smallRoyaltyBytes := kdaGetterRoyalties(1, 1, 1, 1)
	largeRoyalties, largeRoyaltyBytes := kdaGetterRoyalties(32, 500, 500, 32)

	identity := func(kda *kapps.KDAData) uint64 {
		return uint64(len(kda.ID) + len(kda.Name) + len(kda.OwnerAddress) + len(kda.AdminAddress))
	}
	base := func() *kapps.KDAData {
		return &kapps.KDAData{Properties: &kapps.PropertiesData{CanMint: true}, Attributes: &kapps.AttributesData{IsPaused: true}}
	}

	shapes := []struct {
		name    string
		kda     *kapps.KDAData
		charged uint64
		large   bool
	}{
		{"minimal", base(), 0, false},
		{"identityOnly", func() *kapps.KDAData {
			kda := base()
			kda.ID = []byte("TKN-ABCD")
			kda.Name = []byte("Token Name")
			kda.OwnerAddress = kdaGetterBytes(32, 'o')
			kda.AdminAddress = kdaGetterBytes(32, 'p')
			return kda
		}(), 0, false},
		{"emptyRoyalties", func() *kapps.KDAData {
			kda := base()
			kda.Royalties = emptyRoyalties
			return kda
		}(), emptyRoyaltyBytes, false},
		{"small", func() *kapps.KDAData {
			kda := base()
			kda.ID = []byte("T")
			kda.Logo = "logo"
			kda.Royalties = smallRoyalties
			kda.Roles = smallRoles
			kda.URIs = smallURIs
			kda.Precision = 6
			kda.MaxSupply = 1 << 50
			return kda
		}(), 4 + smallRoyaltyBytes + smallRoleBytes + smallURIBytes, false},
		{"large", func() *kapps.KDAData {
			kda := base()
			kda.ID = kdaGetterBytes(64, 'i')
			kda.Name = kdaGetterBytes(128, 'n')
			kda.OwnerAddress = kdaGetterBytes(32, 'o')
			kda.AdminAddress = kdaGetterBytes(32, 'p')
			kda.Logo = string(kdaGetterBytes(64*1024, 'l'))
			kda.Royalties = largeRoyalties
			kda.Roles = largeRoles
			kda.URIs = largeURIs
			return kda
		}(), 64*1024 + largeRoyaltyBytes + largeRoleBytes + largeURIBytes, true},
	}

	cases := make([]kdaGetterCase, 0, len(shapes))
	for _, shape := range shapes {
		kda := shape.kda
		cases = append(cases, kdaGetterCase{
			name: "ManagedGetKDATokenData/" + shape.name,
			install: func(host *contextmock.VMHostMock) {
				kdaGetterStubToken(host, kda, &kapps.UserKDA{})
			},
			call: func(hooks *vmhooks.VMHooksImpl, addressHandle, tickerHandle int32) {
				hooks.ManagedGetKDATokenData(addressHandle, tickerHandle, 0,
					h[0], h[1], h[2], h[3], h[4], h[5], h[6], h[7], h[8], h[9], h[10], h[11], h[12], h[13], h[14], h[15], h[16])
			},
			fixed:     fixed,
			copyBytes: shape.charged,
			// from the fork id, name, creator and admin are paid for too (KLC-2755)
			inputBytes:    uint64(len(kdaGetterAddress) + len(kdaGetterTicker)),
			v6OutputBytes: identity(kda),
			v6DecodeBytes: vmhooks.KDATokenDecodedLen(kda, &kapps.UserKDA{}),
			bufferOuts:    bufferOuts,
			bigIntOuts:    bigIntOuts,
			large:         shape.large,
		})
	}
	return cases
}

func kdaGetterRolesCases(gasMap config.GasScheduleMap) []kdaGetterCase {
	fixed := gasMap["BaseOpsAPICost"]["GetKDARoles"]
	out := kdaGetterOuts(1, 0)
	cases := make([]kdaGetterCase, 0)
	for _, shape := range []struct{ n, addressLen int }{{0, 0}, {1, 1}, {2, 5}, {3, 0}, {5_000, 32}} {
		roles, charged := kdaGetterRoles(shape.n, shape.addressLen)
		cases = append(cases, kdaGetterCase{
			name: fmt.Sprintf("ManagedGetKDARoles/roles=%d/address=%d", shape.n, shape.addressLen),
			install: func(host *contextmock.VMHostMock) {
				kdaGetterStubToken(host, &kapps.KDAData{Roles: roles}, nil)
			},
			call: func(hooks *vmhooks.VMHooksImpl, _, tickerHandle int32) {
				hooks.ManagedGetKDARoles(tickerHandle, out[0])
			},
			fixed:         fixed,
			copyBytes:     charged,
			inputBytes:    uint64(len(kdaGetterTicker)),
			v6DecodeBytes: vmhooks.KDATokenDecodedLen(&kapps.KDAData{Roles: roles}, nil),
			bufferOuts:    out,
			large:         shape.n >= 5_000,
		})
	}
	return cases
}

func kdaGetterSftCases(gasMap config.GasScheduleMap) []kdaGetterCase {
	fixed := gasMap["BaseOpsAPICost"]["GetSFTMetadata"]
	out := kdaGetterOuts(1, 0)
	cases := make([]kdaGetterCase, 0)
	for _, shape := range []struct{ name, hash, attributes int }{
		{0, 0, 0}, {20, 30, 100}, {0, 32, 0}, {1, 0, 1}, {1024, 64, 512 * 1024},
	} {
		meta := &kapps.MetaV2{
			Circulation: 10,
			MaxSupply:   1 << 40,
			Metadata: &kapps.MetaV2Data{
				Name:       kdaGetterBytes(shape.name, 'n'),
				Hash:       kdaGetterBytes(shape.hash, 'h'),
				Attributes: kdaGetterBytes(shape.attributes, 'a'),
			},
		}
		cases = append(cases, kdaGetterCase{
			name: fmt.Sprintf("ManagedGetSftMetadata/name=%d/hash=%d/attributes=%d", shape.name, shape.hash, shape.attributes),
			install: func(host *contextmock.VMHostMock) {
				blockchain := host.BlockchainContext.(*hostmock.BlockchainContextStub)
				blockchain.GetSFTMetaCalled = func(_ []byte, _ uint64) (*kapps.MetaV2, error) {
					return meta, nil
				}
			},
			call: func(hooks *vmhooks.VMHooksImpl, _, tickerHandle int32) {
				hooks.ManagedGetSftMetadata(tickerHandle, 3, out[0])
			},
			fixed:      fixed,
			copyBytes:  uint64(shape.name + shape.hash + shape.attributes), // #nosec G115
			inputBytes: uint64(len(kdaGetterTicker)),
			bufferOuts: out,
			large:      shape.attributes >= 512*1024,
		})
	}
	return cases
}

func kdaGetterAllCases(gasMap config.GasScheduleMap) []kdaGetterCase {
	cases := kdaGetterCallValueCases(gasMap)
	cases = append(cases, kdaGetterUserKDACases(gasMap)...)
	cases = append(cases, kdaGetterTokenDataCases(gasMap)...)
	cases = append(cases, kdaGetterRolesCases(gasMap)...)
	cases = append(cases, kdaGetterSftCases(gasMap)...)
	return cases
}

// kdaGetterRun is what one call left behind
type kdaGetterRun struct {
	gasUsed   uint64
	allocated uint64
	failErr   error
	// buffers and bigInts are the outputs, nil for one the call never wrote
	buffers [][]byte
	bigInts []*big.Int
	// newBuffers and newBigInts count the handles the call created, outputs included
	newBuffers int32
	newBigInts int32
}

func runKDAGetter(t *testing.T, gasMap config.GasScheduleMap, fixV6 bool, c kdaGetterCase, budget uint64) kdaGetterRun {
	t.Helper()
	host, _ := newMockVMHostWithRealMetering(t, fixV6, gasMap, flatCloneGasProvided)
	runtime := host.RuntimeContext.(*contextmock.RuntimeContextMock)
	managedType := host.ManagedTypes()
	c.install(host)

	addressHandle := managedType.NewManagedBufferFromBytes(kdaGetterAddress)
	tickerHandle := managedType.NewManagedBufferFromBytes(kdaGetterTicker)
	bufferProbe := managedType.NewManagedBuffer()
	bigIntProbe := managedType.NewBigIntFromInt64(0)

	run := kdaGetterRun{}
	run.gasUsed, run.allocated = runFlatCloneHook(host, budget, func() {
		c.call(vmhooks.NewVMHooksImpl(host), addressHandle, tickerHandle)
	})
	run.failErr = runtime.FailExecutionErr

	run.newBuffers = managedType.NewManagedBuffer() - bufferProbe - 1
	run.newBigInts = managedType.NewBigIntFromInt64(0) - bigIntProbe - 1
	for _, handle := range c.bufferOuts {
		data, err := managedType.GetBytes(handle)
		if err != nil {
			data = nil
		}
		run.buffers = append(run.buffers, data)
	}
	for _, handle := range c.bigIntOuts {
		value, err := managedType.GetBigInt(handle)
		if err != nil {
			value = nil
		}
		run.bigInts = append(run.bigInts, value)
	}
	return run
}

// The KDA getters built their outputs and only then charged for them, unbounded, or not at all for
// id, name, creator and admin. From FixAuditChangesV6 each output is paid for, bounded, before it
// is built. The amount is what it was before the fork, plus the identity fields of
// ManagedGetKDATokenData, and on top of KLC-2719's address and ticker copies; the outputs are the
// same on both sides
func TestVMHooksImpl_KDAGetters_GasAndOutputsBothSidesOfV6(t *testing.T) {
	t.Parallel()

	gasMap := kdaGetterGasMap()
	for _, c := range kdaGetterAllCases(gasMap) {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			pre := runKDAGetter(t, gasMap, false, c, kdaGetterBudget)
			post := runKDAGetter(t, gasMap, true, c, kdaGetterBudget)

			require.NoError(t, pre.failErr)
			require.NoError(t, post.failErr)
			assert.Equal(t, c.gas(gasMap, false), pre.gasUsed, "pre-fork gas")
			assert.Equal(t, c.gas(gasMap, true), post.gasUsed, "post-fork gas")

			assert.Equal(t, pre.buffers, post.buffers)
			assert.Equal(t, pre.bigInts, post.bigInts)
			assert.Equal(t, pre.newBuffers, post.newBuffers)
			assert.Equal(t, pre.newBigInts, post.newBigInts)
			for i, data := range post.buffers {
				assert.NotNil(t, data, "buffer output %d", i)
			}
			for i, value := range post.bigInts {
				assert.NotNil(t, value, "big int output %d", i)
			}
		})
	}
}

// ManagedGetKDATokenData charged nothing for id, name, creator and admin before the fork. From it they
// are DataCopyPerByte a byte like the rest of its outputs: this pins that delta on its own
func TestVMHooksImpl_ManagedGetKDATokenData_ChargesIdentityFromV6(t *testing.T) {
	t.Parallel()

	gasMap := kdaGetterGasMap()
	perByte := gasMap["BaseOperationCost"]["DataCopyPerByte"]
	for _, c := range kdaGetterTokenDataCases(gasMap) {
		if c.name != "ManagedGetKDATokenData/identityOnly" {
			continue
		}
		pre := runKDAGetter(t, gasMap, false, c, kdaGetterBudget)
		post := runKDAGetter(t, gasMap, true, c, kdaGetterBudget)
		copies := perByte * uint64(len(kdaGetterAddress)+len(kdaGetterTicker))
		identity := perByte * uint64(len("TKN-ABCD")+len("Token Name")+32+32)
		assert.Equal(t, gasMap["BaseOpsAPICost"]["GetKDATokenData"], pre.gasUsed)
		// and, from the same fork, the records the read decoded
		decode := perByte * c.v6DecodeBytes
		assert.Equal(t, pre.gasUsed+copies+identity+decode, post.gasUsed)
		return
	}
	t.Fatal("identityOnly shape missing")
}

// A getter that cannot pay for its fixed cost or for its outputs fails from FixAuditChangesV6 with
// ErrNotEnoughGas, before it builds anything: no handle created, no output written, and for the
// large shapes nothing near their size allocated. Before the fork the same budget overdraws and the
// outputs are built. Not parallel: it counts allocations
func TestVMHooksImpl_KDAGetters_UnderfundedBuildsNothingFromV6(t *testing.T) {
	gasMap := kdaGetterGasMap()
	perByte := gasMap["BaseOperationCost"]["DataCopyPerByte"]
	tooBigPerByte := gasMap["BigIntAPICost"]["CopyPerByteForTooBig"]

	for _, c := range kdaGetterAllCases(gasMap) {
		postGas := c.gas(gasMap, true)
		budgets := []struct {
			name   string
			budget uint64
		}{
			{"fixedShortfall", c.fixed - 1},
		}
		if c.copyBytes > 0 || c.tooBigBytes > 0 || c.v6OutputBytes > 0 || c.v6NameBytes > 0 || c.v6TooBigBytes > 0 {
			budgets = append(budgets, struct {
				name   string
				budget uint64
			}{"outputShortfall", postGas - 1})
		}
		if c.tooBigBytes > 0 && c.copyBytes > 0 {
			// one short of the transfer bytes, before the big int charge is reached
			budgets = append(budgets, struct {
				name   string
				budget uint64
			}{"transferBytesShortfall", c.fixed + perByte*c.copyBytes - 1})
		}
		if c.v6NameBytes > 0 {
			// enough for the encoded transfers but not their names, which from FixAuditChangesV6 are
			// paid for in the same charge
			budgets = append(budgets, struct {
				name   string
				budget uint64
			}{"transferNamesShortfall", c.fixed + perByte*c.copyBytes})
		}
		if c.v6TooBigBytes > 0 {
			// one short of the transfer values, charged after the transfer bytes and names and, for
			// ManagedGetBackTransfers, before its call value
			budgets = append(budgets, struct {
				name   string
				budget uint64
			}{"transferValuesShortfall", c.fixed + perByte*(c.copyBytes+c.v6NameBytes) + tooBigPerByte*c.v6TooBigBytes - 1})
		}

		for _, b := range budgets {
			t.Run(fmt.Sprintf("%s/%s", c.name, b.name), func(t *testing.T) {
				post := runKDAGetter(t, gasMap, true, c, b.budget)
				require.ErrorIs(t, post.failErr, vmhost.ErrNotEnoughGas)
				// a fault burns the gas left
				assert.Equal(t, b.budget, post.gasUsed)
				assert.Zero(t, post.newBuffers, "buffers created")
				assert.Zero(t, post.newBigInts, "big ints created")
				for i, data := range post.buffers {
					assert.Nil(t, data, "buffer output %d", i)
				}
				for i, value := range post.bigInts {
					assert.Nil(t, value, "big int output %d", i)
				}
				if c.large && !c.buildsBeforeCharge {
					assert.Less(t, post.allocated, uint64(flatCloneNoCloneBelow), "allocated")
				}

				// pre-fork the charges overdraw and the outputs are built anyway
				pre := runKDAGetter(t, gasMap, false, c, b.budget)
				require.NoError(t, pre.failErr)
				assert.Equal(t, c.gas(gasMap, false), pre.gasUsed)
				for i, data := range pre.buffers {
					assert.NotNil(t, data, "pre-fork buffer output %d", i)
				}
				if c.large && !c.buildsBeforeCharge {
					assert.Greater(t, pre.allocated, uint64(flatCloneNoCloneBelow), "pre-fork allocated")
				}
			})
		}
	}
}

func kdaGetterLimitTrieReads(t *testing.T, host *contextmock.VMHostMock) {
	t.Helper()

	parser, err := parsers.NewKDATransferParser(worldmock.WorldMarshalizer)
	require.NoError(t, err)
	counter, err := counters.NewUsageCounter(parser, host.ForkControllerContext)
	require.NoError(t, err)
	counter.SetMaximumValues(map[string]uint64{
		"MaxBuiltInCallsPerTx":      1,
		"MaxNumberOfTransfersPerTx": 1,
		"MaxNumberOfTrieReadsPerTx": 1,
	})

	blockchain := host.BlockchainContext.(*hostmock.BlockchainContextStub)
	getKDAToken, getSFTMeta := blockchain.GetKDATokenCalled, blockchain.GetSFTMetaCalled
	blockchain.GetKDATokenCalled = func(address []byte, tokenID []byte, nonce uint64) (*kapps.KDAData, *kapps.UserKDA, error) {
		err := counter.ProcessCrtNumberOfTrieReadsCounter()
		if err != nil {
			return nil, nil, err
		}
		return getKDAToken(address, tokenID, nonce)
	}
	blockchain.GetSFTMetaCalled = func(tokenID []byte, nonce uint64) (*kapps.MetaV2, error) {
		err := counter.ProcessCrtNumberOfTrieReadsCounter()
		if err != nil {
			return nil, err
		}
		return getSFTMeta(tokenID, nonce)
	}
}

func newKDAGetterHostWithFixV7(t *testing.T, gasMap config.GasScheduleMap, fixV7 bool) *contextmock.VMHostMock {
	t.Helper()

	host, _ := newMockVMHostWithRealMetering(t, true, gasMap, kdaGetterBudget)
	host.ForkControllerContext = &commonMock.ForkControllerStub{
		FixAuditChangesV4Value: true,
		FixAuditChangesV6Value: true,
		FixAuditChangesV7Value: fixV7,
	}
	return host
}

func TestVMHooksImpl_KDAGetters_ReadPastTheTrieReadsLimitFailsExecution(t *testing.T) {
	t.Parallel()

	gasMap := kdaGetterGasMap()
	balanceOut := kdaGetterOuts(1, 0)
	cases := []kdaGetterCase{
		{
			name: "ManagedGetKDABalance",
			install: func(host *contextmock.VMHostMock) {
				kdaGetterStubToken(host, &kapps.KDAData{}, &kapps.UserKDA{Balance: 5})
			},
			call: func(hooks *vmhooks.VMHooksImpl, addressHandle, tickerHandle int32) {
				hooks.ManagedGetKDABalance(addressHandle, tickerHandle, 0, balanceOut[0])
			},
		},
		kdaGetterUserKDACases(gasMap)[0],
		kdaGetterTokenDataCases(gasMap)[0],
		kdaGetterRolesCases(gasMap)[0],
		kdaGetterSftCases(gasMap)[0],
	}

	for _, c := range cases {
		for _, fixV7 := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/fixAuditChangesV7=%t", c.name, fixV7), func(t *testing.T) {
				t.Parallel()

				host := newKDAGetterHostWithFixV7(t, gasMap, fixV7)
				runtime := host.RuntimeContext.(*contextmock.RuntimeContextMock)
				managedType := host.ManagedTypes()
				c.install(host)
				kdaGetterLimitTrieReads(t, host)

				addressHandle := managedType.NewManagedBufferFromBytes(kdaGetterAddress)
				tickerHandle := managedType.NewManagedBufferFromBytes(kdaGetterTicker)
				hooks := vmhooks.NewVMHooksImpl(host)

				c.call(hooks, addressHandle, tickerHandle)
				require.NoError(t, runtime.FailExecutionErr)

				c.call(hooks, addressHandle, tickerHandle)
				if fixV7 {
					require.ErrorIs(t, runtime.FailExecutionErr, process.ErrMaxCallsReached)
					return
				}
				// the real hook counts only from FixAuditChangesV7, so on chain this error never
				// reaches faultKDAArg before the fork: this case pins faultKDAArg's own V7 gate
				require.ErrorIs(t, runtime.FailExecutionErr, vmhost.ErrArgOutOfRange)
			})
		}
	}
}
