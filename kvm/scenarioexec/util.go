package scenarioexec

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"

	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	"github.com/klever-io/klever-go/data/state"
	worldmock "github.com/klever-io/klever-go/kvm/mock/world"
	scenexpressionreconstructor "github.com/klever-io/klever-go/kvm/scenarioexec/expression/reconstructor"
	"github.com/klever-io/klever-go/kvm/scenarioexec/kdaconvert"
	scenjsonmodel "github.com/klever-io/klever-go/kvm/scenarioexec/model"
	"github.com/klever-io/klever-go/vmcommon"
)

func createTestAssetsFromSetKDAData(kdaData []*scenjsonmodel.KDAData, world *worldmock.MockWorld) {
	for _, scenKDAData := range kdaData {
		world.MockTestAsset(scenKDAData)
	}
}

func convertAccount(testAcct *scenjsonmodel.Account, world *worldmock.MockWorld) (state.UserAccountHandler, error) {
	storage := make(map[string][]byte)
	for _, stkvp := range testAcct.Storage {
		key := string(stkvp.Key.Value)
		storage[key] = stkvp.Value.Value
	}
	createTestAssetsFromSetKDAData(testAcct.KDAData, world)

	if len(testAcct.Address.Value) != 32 {
		return nil, errors.New("bad test: account address should be 32 bytes long")
	}

	acct, err := state.NewUserAccount(testAcct.Address.Value)
	if err != nil {
		return nil, err
	}
	acct.Nonce = testAcct.Nonce.Value
	acct.Balance = testAcct.Balance.Value.Int64()
	acct.Name = testAcct.Username.Value
	acct.SetCode(testAcct.Code.Value)
	if testAcct.CodeMetadata.Unspecified {
		acct.SetCodeMetadata((&vmcommon.CodeMetadata{
			Payable:     true,
			Upgradeable: true,
			Readable:    true,
		}).ToBytes())
	} else {
		acct.SetCodeMetadata(testAcct.CodeMetadata.Value)
	}
	acct.OwnerAddress = testAcct.Owner.Value

	for _, stkvp := range testAcct.Storage {
		err = acct.DataTrieTracker().SaveKeyValue(stkvp.Key.Value, stkvp.Value.Value)
		if err != nil {
			return nil, err
		}
	}

	err = kdaconvert.WriteScenariosKDAToStorage(testAcct.KDAData, acct)
	if err != nil {
		return nil, err
	}

	err = kdaconvert.SetMultiKDARoles(acct, testAcct.KDAData, world)
	if err != nil {
		return nil, err
	}

	return acct, nil
}

func convertKAppAccount(testAcct *scenjsonmodel.Account, world *worldmock.MockWorld) (state.KAppAccountHandler, error) {
	storage := make(map[string][]byte)
	for _, stkvp := range testAcct.Storage {
		key := string(stkvp.Key.Value)
		storage[key] = stkvp.Value.Value
	}
	if len(testAcct.Address.Value) != 32 {
		return nil, errors.New("bad test: kapp address should be 32 bytes long")
	}

	// get account from cacher
	acct, err := world.AccountsCacher.GetExistingKapp(testAcct.Address.Value)
	if err != nil {
		return nil, err
	}

	for _, stkvp := range testAcct.Storage {
		err = acct.DataTrieTracker().SaveKeyValue(stkvp.Key.Value, stkvp.Value.Value)
		if err != nil {
			return nil, err
		}
	}

	return acct, nil
}

func validate(addr []byte, code []byte) error {
	if len(addr) != 32 {
		return fmt.Errorf(
			"account address should be 32 bytes long: 0x%s",
			hex.EncodeToString(addr))
	}

	scAddress := core.IsSmartContractAddress(addr)
	if len(code) > 0 {
		if !scAddress {
			return fmt.Errorf(
				"account has code but not a smart contract address: %s",
				hex.EncodeToString(addr))
		}
	} else {
		if scAddress {
			return fmt.Errorf(
				"account has a smart contract address, but has no code: 0x%s",
				hex.EncodeToString(addr))
		}
	}

	return nil
}

func validateSetStateAccount(scenAccount *scenjsonmodel.Account, converted state.UserAccountHandler) error {
	err := validate(converted.AddressBytes(), scenAccount.Code.Value)
	if err != nil {
		return fmt.Errorf(
			`"setState" step validation failed for account "%s": %w`,
			scenAccount.Address.Original,
			err)
	}
	return nil
}

func validateNewAddressMocks(testNAMs []*scenjsonmodel.NewAddressMock) error {
	for _, testNAM := range testNAMs {
		if !worldmock.IsSmartContractAddress(testNAM.NewAddress.Value) {
			return fmt.Errorf(
				`address in "setState" "newAddresses" field should have SC format: %s`,
				testNAM.NewAddress.Original)
		}
	}
	return nil
}

func convertNewAddressMocks(testNAMs []*scenjsonmodel.NewAddressMock) []*worldmock.NewAddressMock {
	var result []*worldmock.NewAddressMock
	for _, testNAM := range testNAMs {
		result = append(result, &worldmock.NewAddressMock{
			CreatorAddress: testNAM.CreatorAddress.Value,
			CreatorNonce:   testNAM.CreatorNonce.Value,
			NewAddress:     testNAM.NewAddress.Value,
		})
	}
	return result
}

func convertBlockInfo(testBlockInfo *scenjsonmodel.BlockInfo, currentInfo *worldmock.BlockInfo) *worldmock.BlockInfo {
	if testBlockInfo == nil {
		return currentInfo
	}

	if currentInfo == nil {
		currentInfo = &worldmock.BlockInfo{
			BlockTimestamp: 0,
			BlockNonce:     0,
			BlockSlot:      0,
			BlockEpoch:     0,
			RandomSeed:     nil,
		}
	}

	if !testBlockInfo.BlockTimestamp.OriginalEmpty() {
		currentInfo.BlockTimestamp = int64(testBlockInfo.BlockTimestamp.Value) // #nosec G115 - block timestamp max int64

	}

	if !testBlockInfo.BlockNonce.OriginalEmpty() {
		currentInfo.BlockNonce = testBlockInfo.BlockNonce.Value
	}

	if !testBlockInfo.BlockRound.OriginalEmpty() {
		currentInfo.BlockSlot = testBlockInfo.BlockRound.Value
	}

	if !testBlockInfo.BlockEpoch.OriginalEmpty() {
		// #nosec G115 - scenario data is trusted
		currentInfo.BlockEpoch = uint32(testBlockInfo.BlockEpoch.Value)
	}

	if testBlockInfo.BlockRandomSeed != nil && !testBlockInfo.BlockRandomSeed.OriginalEmpty() {
		var randomsSeed [48]byte
		copy(randomsSeed[:], testBlockInfo.BlockRandomSeed.Value)
		currentInfo.RandomSeed = &randomsSeed

	}

	return currentInfo
}

// this is a small hack, so we can reuse JSON printing in error messages
func (ae *VMTestExecutor) convertLogToTestFormat(outputLog *vmcommon.LogEntry) *scenjsonmodel.LogEntry {
	topics := scenjsonmodel.JSONCheckValueList{
		Values: make([]scenjsonmodel.JSONCheckBytes, len(outputLog.Topics)),
	}
	for i, topic := range outputLog.Topics {
		topics.Values[i] = scenjsonmodel.JSONCheckBytesReconstructed(
			topic,
			ae.exprReconstructor.Reconstruct(topic,
				scenexpressionreconstructor.NoHint))
	}
	testLog := scenjsonmodel.LogEntry{
		Address: scenjsonmodel.JSONCheckBytesReconstructed(
			outputLog.Address,
			ae.exprReconstructor.Reconstruct(outputLog.Address,
				scenexpressionreconstructor.AddressHint)),
		Endpoint: scenjsonmodel.JSONCheckBytesReconstructed(
			outputLog.Identifier,
			ae.exprReconstructor.Reconstruct(outputLog.Identifier,
				scenexpressionreconstructor.StrHint)),
		Topics: topics,
	}

	return &testLog
}

func generateTxHash(txIndex string) []byte {
	txIndexBytes := []byte(txIndex)
	if len(txIndexBytes) > 32 {
		return txIndexBytes[:32]
	}
	for i := len(txIndexBytes); i < 32; i++ {
		txIndexBytes = append(txIndexBytes, '.')
	}
	return txIndexBytes
}

func addKDAToVMInput(klvValue scenjsonmodel.JSONBigInt, kdaData []*scenjsonmodel.KDATxData, vmInput *vmcommon.VMInput) {
	kdaDataLen := len(kdaData)

	// KLVValue argument is just a feature for semantic scenario writing, has the same use as KDATxData
	hasKLVValue := klvValue.Value != nil && klvValue.Value.Cmp(big.NewInt(0)) > 0

	if kdaDataLen > 0 || hasKLVValue {
		vmInput.KDATransfers = make([]*vmcommon.KDATransfer, kdaDataLen)
		for i := 0; i < kdaDataLen; i++ {
			tokenIdentifier := kdaData[i].TokenIdentifier.Value
			vmInput.KDATransfers[i] = &vmcommon.KDATransfer{}
			vmInput.KDATransfers[i].KDATokenName = tokenIdentifier
			vmInput.KDATransfers[i].KDAValue = kdaData[i].Value.Value
			vmInput.KDATransfers[i].KDATokenNonce = kdaData[i].Nonce.Value
			if vmInput.KDATransfers[i].KDATokenNonce != 0 {
				vmInput.KDATransfers[i].KDATokenType = uint32(core.NonFungible)
			} else {
				vmInput.KDATransfers[i].KDATokenType = uint32(core.Fungible)
			}

			// if KDAData contains KLV token, it has priority over KLVValue
			if bytes.Equal(tokenIdentifier, kdautils.KLVIdentifier) {
				hasKLVValue = false
			}
		}
		if hasKLVValue {
			vmInput.KDATransfers = append(vmInput.KDATransfers, &vmcommon.KDATransfer{
				KDATokenName:  kdautils.KLVIdentifier,
				KDAValue:      klvValue.Value,
				KDATokenNonce: 0,
				KDATokenType:  uint32(core.Fungible),
			})
		}
	}
}

func logGasTrace(ae *VMTestExecutor) {
	if ae.PeekTraceGas() {
		metering := ae.getVMHost().Metering()
		scGasTrace := metering.GetGasTrace()
		totalGasUsedByAPIs := 0
		for scAddress, gasTrace := range scGasTrace {
			fmt.Println("Gas Trace for: ", "SC Address", scAddress)
			for functionName, value := range gasTrace {
				totalGasUsed := uint64(0)
				for _, usedGas := range value {
					totalGasUsed += usedGas
				}
				fmt.Println("GasTrace: functionName:", functionName, ",  totalGasUsed:", totalGasUsed, ", numberOfCalls:", len(value))
				totalGasUsedByAPIs += int(totalGasUsed) // #nosec G115
			}
			fmt.Println("TotalGasUsedByAPIs: ", totalGasUsedByAPIs)
		}
	}
}

func setGasTraceInMetering(ae *VMTestExecutor, enable bool) {
	metering := ae.getVMHost().Metering()
	if enable && ae.PeekTraceGas() {
		metering.SetGasTracing(true)
	} else {
		metering.SetGasTracing(false)
	}
}

func setExternalStepGasTracing(ae *VMTestExecutor, step *scenjsonmodel.ExternalStepsStep) {
	switch step.TraceGas.ToInt() {
	case scenjsonmodel.Undefined.ToInt():
		ae.scenarioTraceGas = append(ae.scenarioTraceGas, ae.PeekTraceGas())
	case scenjsonmodel.TrueValue.ToInt():
		ae.scenarioTraceGas = append(ae.scenarioTraceGas, true)
	case scenjsonmodel.FalseValue.ToInt():
		ae.scenarioTraceGas = append(ae.scenarioTraceGas, false)
	}
}

func resetGasTracesIfNewTest(ae *VMTestExecutor, scenario *scenjsonmodel.Scenario) {
	if ae.vm == nil || scenario.IsNewTest {
		ae.scenarioTraceGas = make([]bool, 0)
		ae.scenarioTraceGas = append(ae.scenarioTraceGas, scenario.TraceGas)
		scenario.IsNewTest = false
	}
}
