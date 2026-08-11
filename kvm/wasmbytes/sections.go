package wasmbytes

import (
	"bytes"
	"errors"
)

const SectionStart byte = 8

const maxLEB128Groups = 5

var wasmPreamble = []byte{0x00, 0x61, 0x73, 0x6D, 0x01, 0x00, 0x00, 0x00}

var ErrInvalidWasmModule = errors.New("invalid wasm module")

type Section struct {
	ID      byte
	Payload []byte
}

func WalkSections(code []byte, visit func(section Section) bool) error {
	if len(code) < len(wasmPreamble) || !bytes.Equal(code[:len(wasmPreamble)], wasmPreamble) {
		return ErrInvalidWasmModule
	}

	offset := len(wasmPreamble)
	for offset < len(code) {
		id := code[offset]
		offset++

		payloadLen, read, err := readVarUint32(code[offset:])
		if err != nil {
			return err
		}
		offset += read

		end := offset + int(payloadLen)
		if end < offset || end > len(code) {
			return ErrInvalidWasmModule
		}

		if !visit(Section{ID: id, Payload: code[offset:end]}) {
			return nil
		}

		offset = end
	}

	return nil
}

func HasStartSection(code []byte) (bool, error) {
	found := false

	err := WalkSections(code, func(section Section) bool {
		if section.ID == SectionStart {
			found = true
			return false
		}

		return true
	})
	if err != nil {
		return false, err
	}

	return found, nil
}

func readVarUint32(data []byte) (uint32, int, error) {
	var result uint32
	var shift uint

	for i := 0; i < len(data) && i < maxLEB128Groups; i++ {
		b := data[i]

		if i == maxLEB128Groups-1 && b > 0x0F {
			return 0, 0, ErrInvalidWasmModule
		}

		result |= uint32(b&0x7F) << shift
		if b&0x80 == 0 {
			return result, i + 1, nil
		}

		shift += 7
	}

	return 0, 0, ErrInvalidWasmModule
}
