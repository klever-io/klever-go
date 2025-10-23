package hostCore_test

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/klever-io/klever-go/data/vm"
	"github.com/klever-io/klever-go/kvm/testcommon"
	"github.com/klever-io/klever-go/vmcommon"
	"github.com/stretchr/testify/require"
)

// Path to the heavy operations contract WASM file (relative to project root)
const heavyOpsContractPath = "../../test/contracts/timeout/output/timeout.wasm"

// TestHeavyOperations_Run tests the run function which performs many hook calls
// and measures its execution time to verify optimization effectiveness
func TestHeavyOperations_Run(t *testing.T) {
	// Load the contract WASM code
	contractCode, err := os.ReadFile(heavyOpsContractPath)
	require.NoError(t, err, "Failed to load timeout contract from %s", heavyOpsContractPath)
	require.NotEmpty(t, contractCode, "Contract code is empty")

	// Setup VM Host and Mock World
	vmHost, mockWorld := createVmHostAndMockWorld(t)
	defer vmHost.Reset()

	// Create the smart contract account directly in the mock world
	scAddress := testcommon.MakeTestSCAddress("timeout")
	t.Logf("Creating smart contract at address: %x", scAddress)

	scAccount := mockWorld.CreateSmartContractAccount(testOwnerAddress, scAddress, contractCode, mockWorld)
	mockWorld.PutAccount(scAccount)

	// Step 2: Call the run function with no arguments
	t.Log("\nExecuting run function...")

	callInput := &vmcommon.ContractCallInput{
		VMInput: vmcommon.VMInput{
			CallerAddr:   testOwnerAddress,
			GasProvided:  100_000_000, // High gas limit for execution
			CallType:     vm.DirectCall,
			Arguments:    [][]byte{}, // No arguments for run function
			KDATransfers: []*vmcommon.KDATransfer{},
		},
		RecipientAddr:     scAddress,
		Function:          "run",
		AllowInitFunction: false,
	}

	// Execute and measure time
	executionStartTime := time.Now()
	vmOutput, err := vmHost.RunSmartContractCall(callInput)
	executionDuration := time.Since(executionStartTime)

	// Validate execution results
	require.NoError(t, err, "Failed to execute run function")
	require.NotNil(t, vmOutput, "VMOutput is nil after execution")

	// Print detailed results
	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("HEAVY OPERATIONS CONTRACT EXECUTION RESULTS")
	fmt.Println(strings.Repeat("=", 80))

	fmt.Printf("\n📋 CONTRACT INFORMATION:\n")
	fmt.Printf("   Contract Path: %s\n", heavyOpsContractPath)
	fmt.Printf("   Contract Size: %d bytes\n", len(contractCode))
	fmt.Printf("   Function: run()\n")

	fmt.Printf("\n📍 CONTRACT ADDRESS:\n")
	fmt.Printf("   Address: %x\n", scAddress)

	fmt.Printf("\n⏱️  EXECUTION:\n")
	fmt.Printf("   Duration: %v\n", executionDuration)
	fmt.Printf("   Duration (nanoseconds): %d ns\n", executionDuration.Nanoseconds())
	fmt.Printf("   Duration (microseconds): %d µs\n", executionDuration.Microseconds())
	fmt.Printf("   Duration (milliseconds): %d ms\n", executionDuration.Milliseconds())

	fmt.Printf("\n⛽ GAS USAGE:\n")
	fmt.Printf("   Gas Provided: %d\n", callInput.GasProvided)
	fmt.Printf("   Gas Remaining: %d\n", vmOutput.GasRemaining)
	fmt.Printf("   Gas Used: %d\n", callInput.GasProvided-vmOutput.GasRemaining)

	fmt.Printf("\n📤 EXECUTION RESULT:\n")
	fmt.Printf("   Return Code: %s (%d)\n", vmOutput.ReturnCode.String(), vmOutput.ReturnCode)
	fmt.Printf("   Return Message: %s\n", vmOutput.ReturnMessage)

	if len(vmOutput.ReturnData) > 0 {
		fmt.Printf("\n📊 RETURN DATA:\n")
		for i, data := range vmOutput.ReturnData {
			fmt.Printf("   [%d] %d bytes: %x\n", i, len(data), data)
			// Try to interpret as string if printable
			if isPrintable(data) {
				fmt.Printf("       (as string): %s\n", string(data))
			}
		}
	}

	if len(vmOutput.Logs) > 0 {
		fmt.Printf("\n📝 LOGS (%d entries):\n", len(vmOutput.Logs))
		for i, log := range vmOutput.Logs {
			fmt.Printf("   [%d] Address: %x\n", i, log.Address)
			fmt.Printf("       Identifier: %s\n", string(log.Identifier))
			if len(log.Topics) > 0 {
				fmt.Printf("       Topics: %v\n", log.Topics)
			}
			if len(log.Data) > 0 {
				fmt.Printf("       Data: %x\n", log.Data)
			}
		}
	}

	if len(vmOutput.OutputAccounts) > 0 {
		fmt.Printf("\n💾 OUTPUT ACCOUNTS (%d):\n", len(vmOutput.OutputAccounts))
		for addr, outputAccount := range vmOutput.OutputAccounts {
			fmt.Printf("   Address: %x\n", []byte(addr))
			fmt.Printf("   Gas Used: %d\n", outputAccount.GasUsed)
			if len(outputAccount.OutputTransfers) > 0 {
				fmt.Printf("   Transfers: %d\n", len(outputAccount.OutputTransfers))
			}
			if len(outputAccount.StorageUpdates) > 0 {
				fmt.Printf("   Storage Updates: %d\n", len(outputAccount.StorageUpdates))
			}
		}
	}

	fmt.Println("\n" + strings.Repeat("=", 80))

	// Assert successful execution
	require.Equal(t, vmcommon.Ok, vmOutput.ReturnCode,
		"Execution failed with return code %s: %s",
		vmOutput.ReturnCode.String(),
		vmOutput.ReturnMessage)

	t.Logf("✓ run function executed successfully")
	t.Logf("✓ Total execution time: %v", executionDuration)
	t.Logf("✓ Gas consumed: %d", callInput.GasProvided-vmOutput.GasRemaining)
}

// TestHeavyOperations_RunMultipleIterations runs the run function multiple times
// and reports average, min, and max execution times to measure hook call overhead
func TestHeavyOperations_RunMultipleIterations(t *testing.T) {
	iterations := 10

	// Load the contract WASM code
	contractCode, err := os.ReadFile(heavyOpsContractPath)
	require.NoError(t, err, "Failed to load timeout contract from %s", heavyOpsContractPath)

	var durations []time.Duration
	var gasUsages []uint64

	t.Logf("Running %d iterations of run function...\n", iterations)

	for i := 0; i < iterations; i++ {
		// Setup fresh VM Host for each iteration
		vmHost, mockWorld := createVmHostAndMockWorld(t)

		// Create the smart contract account directly
		scAddress := testcommon.MakeTestSCAddress("timeout")
		scAccount := mockWorld.CreateSmartContractAccount(testOwnerAddress, scAddress, contractCode, mockWorld)
		mockWorld.PutAccount(scAccount)

		// Execute run and measure time
		callInput := &vmcommon.ContractCallInput{
			VMInput: vmcommon.VMInput{
				CallerAddr:   testOwnerAddress,
				GasProvided:  100_000_000,
				CallType:     vm.DirectCall,
				Arguments:    [][]byte{}, // No arguments for run function
				KDATransfers: []*vmcommon.KDATransfer{},
			},
			RecipientAddr:     scAddress,
			Function:          "run",
			AllowInitFunction: false,
		}

		startTime := time.Now()
		vmOutput, err := vmHost.RunSmartContractCall(callInput)
		duration := time.Since(startTime)

		require.NoError(t, err)
		require.Equal(t, vmcommon.Ok, vmOutput.ReturnCode)

		durations = append(durations, duration)
		gasUsages = append(gasUsages, callInput.GasProvided-vmOutput.GasRemaining)

		vmHost.Reset()

		t.Logf("  Iteration %2d: %v (gas used: %d)", i+1, duration, gasUsages[i])
	}

	// Calculate statistics
	var totalDuration time.Duration
	var totalGas uint64
	minDuration := durations[0]
	maxDuration := durations[0]

	for i, d := range durations {
		totalDuration += d
		totalGas += gasUsages[i]
		if d < minDuration {
			minDuration = d
		}
		if d > maxDuration {
			maxDuration = d
		}
	}

	avgDuration := totalDuration / time.Duration(iterations)
	avgGas := totalGas / uint64(iterations)

	// Print summary
	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("PERFORMANCE SUMMARY")
	fmt.Println(strings.Repeat("=", 80))
	fmt.Printf("\n📊 EXECUTION TIME STATISTICS (%d iterations):\n", iterations)
	fmt.Printf("   Average: %v (%d µs)\n", avgDuration, avgDuration.Microseconds())
	fmt.Printf("   Minimum: %v (%d µs)\n", minDuration, minDuration.Microseconds())
	fmt.Printf("   Maximum: %v (%d µs)\n", maxDuration, maxDuration.Microseconds())
	fmt.Printf("   Total:   %v\n", totalDuration)

	fmt.Printf("\n⛽ GAS USAGE STATISTICS:\n")
	fmt.Printf("   Average: %d\n", avgGas)
	fmt.Printf("   Total:   %d\n", totalGas)

	fmt.Println("\n" + strings.Repeat("=", 80))
}

// isPrintable checks if a byte slice contains only printable ASCII characters
func isPrintable(data []byte) bool {
	for _, b := range data {
		if b < 32 || b > 126 {
			return false
		}
	}
	return true
}
