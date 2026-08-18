package contexts

import (
	"fmt"
	"math"
	"strings"

	"github.com/klever-io/klever-go/kvm/executor"
	"github.com/klever-io/klever-go/kvm/vmhost"
	"github.com/klever-io/klever-go/kvm/wasmbytes"
	"github.com/klever-io/klever-go/vmcommon"
)

const allowedCharsInFunctionName = "abcdefghijklmnopqrstuvwxyz0123456789_"

// wasmValidator is a validator for WASM SmartContracts
type wasmValidator struct {
	reserved *reservedFunctions
}

// newWASMValidator creates a new WASMValidator
func newWASMValidator(scAPINames vmcommon.FunctionNames, builtInFuncContainer vmcommon.BuiltInFunctionContainer) *wasmValidator {
	return &wasmValidator{
		reserved: NewReservedFunctions(scAPINames, builtInFuncContainer),
	}
}

// VerifyNoStartSection rejects contract code declaring a WASM start section, whose function
// runs at instantiation, outside the gas meter and before any module-level validation.
func VerifyNoStartSection(contract []byte) error {
	hasStartSection, err := wasmbytes.HasStartSection(contract)
	if err != nil {
		return vmhost.ErrContractCodeNotDecodable
	}

	if hasStartSection {
		return vmhost.ErrContractHasStartSection
	}

	return nil
}

func (validator *wasmValidator) verifyMemoryDeclaration(instance executor.Instance) error {
	if !instance.HasMemory() {
		return vmhost.ErrMemoryDeclarationMissing
	}

	return nil
}

// verifyTableDeclaration rejects contracts that declare a WASM table with no
// maximum (reported by Wasmer as u32::MAX), or with a maximum exceeding
// maxDeclaredTableSize (KLC-2526 / KLR-19). This is a redundant safety net:
// the primary enforcement runs earlier, during instantiation itself (see
// validate_tables in klever-vm-executor-rs), since a check that only runs
// after instantiation is too late to prevent the allocation it exists to
// stop. Checks the largest single table's maximum, not the sum across
// tables, but Wasmer itself caps a module to 100 tables, bounding the
// aggregate regardless.
//
// "No declared maximum" is checked explicitly, ahead of and independent of
// the maxDeclaredTableSize comparison: a declared maximum and an
// operator-configured cap occupy the same uint32 space, so a cap of exactly
// math.MaxUint32 (a plausible way to write "effectively unlimited") would
// otherwise make instance.MaxDeclaredTableSize() > maxDeclaredTableSize a
// silent no-op for genuinely unbounded tables, defeating the point of the
// check for the one case it most needs to catch.
func (validator *wasmValidator) verifyTableDeclaration(instance executor.Instance, maxDeclaredTableSize uint32) error {
	declaredMax := instance.MaxDeclaredTableSize()
	if declaredMax == math.MaxUint32 || declaredMax > maxDeclaredTableSize {
		return vmhost.ErrDeclaredTableSizeExceedsMaximum
	}

	return nil
}

func (validator *wasmValidator) verifyFunctions(instance executor.Instance) error {
	for _, functionName := range instance.GetFunctionNames() {
		err := validator.verifyValidFunctionName(functionName)
		if err != nil {
			return err
		}
	}

	return instance.ValidateFunctionArities()
}

var protectedFunctions = map[string]bool{
	"internalVMErrors":  true,
	"transferValueOnly": true,
	"writeLog":          true,
	"signalError":       true,
	"completedTxEvent":  true,
	"totalConsumedGas":  true,
	"returnData":        true,
	"gasRemaining":      true,
	"vmOutput":          true,
}

func (validator *wasmValidator) verifyProtectedFunctions(instance executor.Instance) error {
	for _, functionName := range instance.GetFunctionNames() {
		_, found := protectedFunctions[functionName]
		if found {
			return vmhost.ErrContractInvalid
		}

	}

	return nil
}

func (validator *wasmValidator) verifyValidFunctionName(functionName string) error {
	const maxLengthOfFunctionName = 256

	errInvalidName := fmt.Errorf("%w: %s", vmhost.ErrInvalidFunctionName, functionName)

	if len(functionName) == 0 {
		return errInvalidName
	}
	if len(functionName) >= maxLengthOfFunctionName {
		return errInvalidName
	}
	if isFirstCharacterNumeric(functionName) {
		return errInvalidName
	}
	if !validCharactersOnly(functionName) {
		return errInvalidName
	}
	if validator.reserved.IsReserved(functionName) {
		return errInvalidName
	}

	return nil
}

func validCharactersOnly(input string) bool {
	input = strings.ToLower(input)
	for i := 0; i < len(input); i++ {
		c := string(input[i])
		if !strings.Contains(allowedCharsInFunctionName, c) {
			return false
		}
	}

	return true
}

func isFirstCharacterNumeric(name string) bool {
	return name[0] >= '0' && name[0] <= '9'
}
