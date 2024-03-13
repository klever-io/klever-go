package smartContract

import (
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/vmcommon"
)

func findVMByScAddress(container process.VirtualMachinesContainer, scAddress []byte) (vmcommon.VMExecutionHandler, error) {
	vmType, err := vmcommon.ParseVMTypeFromContractAddress(scAddress)
	if err != nil {
		return nil, err
	}

	vm, err := container.Get(vmType)
	if err != nil {
		return nil, err
	}

	return vm, nil
}
