package featuresintegrationtest

import (
	"fmt"
	"math/big"
	"os"
	"strings"
	"testing"

	"github.com/klever-io/klever-go/core/kapp/builtInFunctions"
	"github.com/klever-io/klever-go/crypto/hashing/blake2b"
	imock "github.com/klever-io/klever-go/integrationTest/mock"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/kvm/config"
	worldhook "github.com/klever-io/klever-go/kvm/mock/world"
	"github.com/klever-io/klever-go/kvm/vmhost"
	"github.com/klever-io/klever-go/kvm/vmhost/hostCore"
	"github.com/klever-io/klever-go/kvm/vmhost/mock"
	vmi "github.com/klever-io/klever-go/vmcommon"
	"github.com/klever-io/klever-go/vmcommon/parsers"
	"github.com/stretchr/testify/require"
)

var defaultHasher = &blake2b.Blake2b{}

type pureFunctionIO struct {
	functionName    string
	arguments       [][]byte
	expectedStatus  vmi.ReturnCode
	expectedMessage string
	expectedResults [][]byte
}

var testVMType = []byte{0, 0}

type resultInterpreter func([]byte) *big.Int
type logProgress func(testCaseIndex, testCaseCount int)

type pureFunctionExecutor struct {
	world           *worldhook.MockWorld
	vm              vmi.VMExecutionHandler
	contractAddress []byte
	userAddress     []byte
}

func newPureFunctionExecutor() (*pureFunctionExecutor, error) {
	world := worldhook.NewMockWorld()

	gasSchedule := config.MakeGasMapForTests()

	protectedKeys := [][]byte{
		[]byte(kapps.ProtectedKleverKeyPrefix),
		[]byte(kapps.ProtectedKLVKeyPrefix),
		[]byte(kapps.ProtectedKFIKeyPrefix),
		[]byte(kapps.KDAPrefix),
	}

	kdaTransferParser, _ := parsers.NewKDATransferParser(worldhook.WorldMarshalizer)
	vm, err := hostCore.NewVMHost(
		world,
		&vmhost.VMHostParameters{
			VMType:                   testVMType,
			OverrideVMExecutor:       nil,
			GasSchedule:              gasSchedule,
			BuiltInFuncContainer:     builtInFunctions.NewBuiltInFunctionContainer(),
			ProtectedKeyPrefix:       protectedKeys,
			KDATransferParser:        kdaTransferParser,
			EpochNotifier:            &mock.EpochNotifierStub{},
			ForkController:           &imock.ForkControllerStub{},
			WasmerSIGSEGVPassthrough: false,
			Hasher:                   defaultHasher,

			TimeOutForSCExecutionInMilliseconds: 2000,
		})
	if err != nil {
		return nil, err
	}
	return &pureFunctionExecutor{
		world: world,
		vm:    vm,
	}, nil
}

func (pfe *pureFunctionExecutor) initAccounts(contractPath string) {
	pfe.contractAddress = []byte("contract_addr_________________s1")
	pfe.userAddress = []byte("user_addr_____________________s1")

	scCode, err := os.ReadFile(contractPath)
	if err != nil {
		panic(err)
	}

	contract, err := pfe.world.AccountsCacher.LoadUser(pfe.contractAddress)
	if err != nil {
		return
	}
	contract.SetCode(scCode)

	user, err := pfe.world.AccountsCacher.LoadUser(pfe.userAddress)
	if err != nil {
		return
	}
	err = user.SetUserKDA(nil, nil, &kapps.UserKDA{Balance: 0x100000000})
	if err != nil {
		return
	}

	err = pfe.world.AccountsCacher.SaveUser(contract)
	if err != nil {
		return
	}
	err = pfe.world.AccountsCacher.SaveUser(user)
	if err != nil {
		return
	}
}

func (pfe *pureFunctionExecutor) scCall(testCase *pureFunctionIO) (*vmi.VMOutput, error) {
	input := &vmi.ContractCallInput{
		RecipientAddr: pfe.contractAddress,
		Function:      testCase.functionName,
		VMInput: vmi.VMInput{
			CallerAddr:  pfe.userAddress,
			Arguments:   testCase.arguments,
			GasProvided: 100000000,
		},
	}

	return pfe.vm.RunSmartContractCall(input)
}

func (pfe *pureFunctionExecutor) checkTxResults(
	testCase *pureFunctionIO,
	output *vmi.VMOutput,
	resultInterpreter resultInterpreter) error {

	if output.ReturnCode != testCase.expectedStatus {
		return fmt.Errorf("result code mismatch. Want: %d. Have: %d (%s). Message: %s",
			int(testCase.expectedStatus), int(output.ReturnCode), output.ReturnCode.String(), output.ReturnMessage)
	}

	if output.ReturnMessage != testCase.expectedMessage {
		return fmt.Errorf("result message mismatch. Want: %s. Have: %s",
			testCase.expectedMessage, output.ReturnMessage)
	}

	// check result
	if len(output.ReturnData) != len(testCase.expectedResults) {
		// rec := er.ExprReconstructor{}
		// return fmt.Errorf("result length mismatch. Want: %s. Have: %s",
		// 	rec.ReconstructList(testCase.expectedResults, er.NoHint),
		// 	rec.ReconstructList(output.ReturnData, er.NoHint))
	}
	for i, expected := range testCase.expectedResults {
		wantNum := resultInterpreter(expected)
		haveNum := resultInterpreter(output.ReturnData[i])
		if wantNum.Cmp(haveNum) != 0 {
			var argStr []string
			for _, arg := range testCase.arguments {
				argNum := resultInterpreter(arg)
				argStr = append(argStr, fmt.Sprintf("%d", argNum))
			}
			return fmt.Errorf("result mismatch. Want: %d. Have: %d. Call: %s(%s)",
				wantNum, haveNum, testCase.functionName, strings.Join(argStr, ", "))
		}
	}

	return nil
}

func (pfe *pureFunctionExecutor) executePureFunctionTests(t *testing.T,
	testCases []*pureFunctionIO,
	resultInterpreter resultInterpreter,
	logProgress logProgress) {

	defer func() {
		vmHost := pfe.vm.(vmhost.VMHost)
		vmHost.Reset()
	}()

	// RUN!
	for testCaseIndex, testCase := range testCases {
		if logProgress != nil {
			logProgress(testCaseIndex, len(testCases))
		}

		output, err := pfe.scCall(testCase)
		require.Nil(t, err)

		err = pfe.checkTxResults(testCase, output, resultInterpreter)
		require.Nil(t, err)
	}
}
