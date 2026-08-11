package mock

import "github.com/klever-io/klever-go/kvm/vmhost"

// MockContractCode wraps an identifier into a minimal well-formed WASM module, so mock
// contracts carry bytecode that survives module-level validation. The identifier is kept
// in a custom section, so the produced code stays unique per contract.
func MockContractCode(identifier []byte) []byte {
	const customSectionID = 0x00

	sectionName := []byte("mock")
	payload := make([]byte, 0, 1+len(sectionName)+len(identifier))
	payload = append(payload, byte(len(sectionName)))
	payload = append(payload, sectionName...)
	payload = append(payload, identifier...)

	code := []byte{0x00, 0x61, 0x73, 0x6D, 0x01, 0x00, 0x00, 0x00}
	code = append(code, customSectionID)
	code = append(code, vmhost.U64ToLEB128(uint64(len(payload)))...)

	return append(code, payload...)
}

// StartSectionMarker is the byte the start function of WasmCodeWithStartSection writes at
// offset 0 of the exported memory, so tests can observe whether the start function ran.
const StartSectionMarker byte = 42

var (
	wasmModulePreamble = []byte{0x00, 0x61, 0x73, 0x6D, 0x01, 0x00, 0x00, 0x00}
	wasmTypeSection    = []byte{0x01, 0x04, 0x01, 0x60, 0x00, 0x00}
	wasmFuncSection    = []byte{0x03, 0x03, 0x02, 0x00, 0x00}
	wasmMemorySection  = []byte{0x05, 0x03, 0x01, 0x00, 0x01}
	wasmExportSection  = []byte{
		0x07, 0x11, 0x02,
		0x04, 'i', 'n', 'i', 't', 0x00, 0x00,
		0x06, 'm', 'e', 'm', 'o', 'r', 'y', 0x02, 0x00,
	}
	wasmStartSection = []byte{0x08, 0x01, 0x01}
	wasmCodeSection  = []byte{
		0x0A, 0x0E, 0x02,
		0x02, 0x00, 0x0B,
		0x09, 0x00, 0x41, 0x00, 0x41, StartSectionMarker, 0x36, 0x02, 0x00, 0x0B,
	}
)

// WasmCodeWithStartSection returns a WASM module that wasmer accepts and instantiates, declaring
// a start section whose function writes StartSectionMarker at offset 0 of the exported memory.
func WasmCodeWithStartSection() []byte {
	return buildWasmCode(true)
}

// WasmCodeWithoutStartSection returns the same module as WasmCodeWithStartSection, minus the start
// section, so tests can tell a rejection caused by the start section from a malformed fixture.
func WasmCodeWithoutStartSection() []byte {
	return buildWasmCode(false)
}

func buildWasmCode(withStartSection bool) []byte {
	code := make([]byte, 0, 64)
	code = append(code, wasmModulePreamble...)
	code = append(code, wasmTypeSection...)
	code = append(code, wasmFuncSection...)
	code = append(code, wasmMemorySection...)
	code = append(code, wasmExportSection...)
	if withStartSection {
		code = append(code, wasmStartSection...)
	}
	return append(code, wasmCodeSection...)
}
