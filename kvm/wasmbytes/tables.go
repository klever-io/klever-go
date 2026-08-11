package wasmbytes

const SectionCode byte = 10

const (
	opUnreachable        byte = 0x00
	opNop                byte = 0x01
	opBlock              byte = 0x02
	opLoop               byte = 0x03
	opIf                 byte = 0x04
	opElse               byte = 0x05
	opEnd                byte = 0x0B
	opBr                 byte = 0x0C
	opBrIf               byte = 0x0D
	opBrTable            byte = 0x0E
	opReturn             byte = 0x0F
	opCall               byte = 0x10
	opCallIndirect       byte = 0x11
	opReturnCall         byte = 0x12
	opReturnCallIndirect byte = 0x13
	opDrop               byte = 0x1A
	opSelect             byte = 0x1B
	opSelectWithTypes    byte = 0x1C
	opFirstVariableOp    byte = 0x20
	opLastVariableOp     byte = 0x24
	opTableGet           byte = 0x25
	opTableSet           byte = 0x26
	opFirstMemoryAccess  byte = 0x28
	opLastMemoryAccess   byte = 0x3E
	opMemorySize         byte = 0x3F
	opMemoryGrow         byte = 0x40
	opI32Const           byte = 0x41
	opI64Const           byte = 0x42
	opF32Const           byte = 0x43
	opF64Const           byte = 0x44
	opFirstNumericOp     byte = 0x45
	opLastNumericOp      byte = 0xC4
	opRefNull            byte = 0xD0
	opRefIsNull          byte = 0xD1
	opRefFunc            byte = 0xD2
	opPrefixFC           byte = 0xFC
)

const (
	subLastSaturatingTruncation uint32 = 7
	subMemoryInit               uint32 = 8
	subDataDrop                 uint32 = 9
	subMemoryCopy               uint32 = 10
	subMemoryFill               uint32 = 11
	subTableInit                uint32 = 12
	subElemDrop                 uint32 = 13
	subTableCopy                uint32 = 14
	subTableGrow                uint32 = 15
	subTableSize                uint32 = 16
	subTableFill                uint32 = 17
)

const (
	maxLEB128Groups64 = 10
	valueTypeSize     = 1
	float32Size       = 4
	float64Size       = 8
)

func MutatesTables(code []byte) bool {
	mutates := false

	err := WalkSections(code, func(section Section) bool {
		if section.ID != SectionCode {
			return true
		}

		found, decodeErr := codeSectionMutatesTables(section.Payload)
		mutates = found || decodeErr != nil

		return false
	})
	if err != nil {
		return true
	}

	return mutates
}

func codeSectionMutatesTables(payload []byte) (bool, error) {
	count, offset, err := readVarUint32(payload)
	if err != nil {
		return false, err
	}

	for i := uint32(0); i < count; i++ {
		size, read, err := readVarUint32(payload[offset:])
		if err != nil {
			return false, err
		}
		offset += read

		end, err := skipBytes(payload, offset, int(size))
		if err != nil {
			return false, err
		}

		mutates, err := functionMutatesTables(payload[offset:end])
		if err != nil {
			return false, err
		}
		if mutates {
			return true, nil
		}

		offset = end
	}

	return false, nil
}

func functionMutatesTables(body []byte) (bool, error) {
	offset, err := skipLocalDeclarations(body)
	if err != nil {
		return false, err
	}

	for offset < len(body) {
		opcode := body[offset]
		offset++

		mutates, next, err := skipImmediates(opcode, body, offset)
		if err != nil {
			return false, err
		}
		if mutates {
			return true, nil
		}

		offset = next
	}

	return false, nil
}

func skipLocalDeclarations(body []byte) (int, error) {
	count, offset, err := readVarUint32(body)
	if err != nil {
		return 0, err
	}

	for i := uint32(0); i < count; i++ {
		offset, err = skipVarUint32(body, offset)
		if err != nil {
			return 0, err
		}

		offset, err = skipBytes(body, offset, valueTypeSize)
		if err != nil {
			return 0, err
		}
	}

	return offset, nil
}

func skipImmediates(opcode byte, body []byte, offset int) (bool, int, error) {
	switch opcode {
	case opTableSet:
		return true, offset, nil

	case opUnreachable, opNop, opElse, opEnd, opReturn, opDrop, opSelect, opRefIsNull:
		return false, offset, nil

	case opBlock, opLoop, opIf, opBr, opBrIf, opCall, opReturnCall, opRefFunc,
		opTableGet, opMemorySize, opMemoryGrow, opI32Const:
		next, err := skipVarUint32(body, offset)
		return false, next, err

	case opCallIndirect, opReturnCallIndirect:
		next, err := skipVarUint32Pair(body, offset)
		return false, next, err

	case opBrTable:
		next, err := skipBranchTable(body, offset)
		return false, next, err

	case opSelectWithTypes:
		next, err := skipValueTypeVector(body, offset)
		return false, next, err

	case opI64Const:
		next, err := skipLEB128(body, offset, maxLEB128Groups64)
		return false, next, err

	case opF32Const:
		next, err := skipBytes(body, offset, float32Size)
		return false, next, err

	case opF64Const:
		next, err := skipBytes(body, offset, float64Size)
		return false, next, err

	case opRefNull:
		next, err := skipBytes(body, offset, valueTypeSize)
		return false, next, err

	case opPrefixFC:
		return skipPrefixedImmediates(body, offset)
	}

	if opcode >= opFirstVariableOp && opcode <= opLastVariableOp {
		next, err := skipVarUint32(body, offset)
		return false, next, err
	}

	if opcode >= opFirstMemoryAccess && opcode <= opLastMemoryAccess {
		next, err := skipVarUint32Pair(body, offset)
		return false, next, err
	}

	if opcode >= opFirstNumericOp && opcode <= opLastNumericOp {
		return false, offset, nil
	}

	return false, 0, ErrInvalidWasmModule
}

func skipPrefixedImmediates(body []byte, offset int) (bool, int, error) {
	subOpcode, read, err := readVarUint32(body[offset:])
	if err != nil {
		return false, 0, err
	}
	offset += read

	switch subOpcode {
	case subTableInit, subTableCopy, subTableGrow, subTableFill:
		return true, offset, nil

	case subMemoryInit, subMemoryCopy:
		next, err := skipVarUint32Pair(body, offset)
		return false, next, err

	case subDataDrop, subMemoryFill, subElemDrop, subTableSize:
		next, err := skipVarUint32(body, offset)
		return false, next, err
	}

	if subOpcode <= subLastSaturatingTruncation {
		return false, offset, nil
	}

	return false, 0, ErrInvalidWasmModule
}

func skipBranchTable(body []byte, offset int) (int, error) {
	count, read, err := readVarUint32(body[offset:])
	if err != nil {
		return 0, err
	}
	offset += read

	for i := uint32(0); i <= count; i++ {
		offset, err = skipVarUint32(body, offset)
		if err != nil {
			return 0, err
		}
	}

	return offset, nil
}

func skipValueTypeVector(body []byte, offset int) (int, error) {
	count, read, err := readVarUint32(body[offset:])
	if err != nil {
		return 0, err
	}

	return skipBytes(body, offset+read, int(count)*valueTypeSize)
}

func skipVarUint32Pair(body []byte, offset int) (int, error) {
	next, err := skipVarUint32(body, offset)
	if err != nil {
		return 0, err
	}

	return skipVarUint32(body, next)
}

func skipVarUint32(body []byte, offset int) (int, error) {
	return skipLEB128(body, offset, maxLEB128Groups)
}

func skipLEB128(body []byte, offset int, maxGroups int) (int, error) {
	for group := 0; group < maxGroups; group++ {
		if offset >= len(body) {
			return 0, ErrInvalidWasmModule
		}

		b := body[offset]
		offset++

		if b&0x80 == 0 {
			return offset, nil
		}
	}

	return 0, ErrInvalidWasmModule
}

func skipBytes(body []byte, offset int, size int) (int, error) {
	end := offset + size
	if size < 0 || end < offset || end > len(body) {
		return 0, ErrInvalidWasmModule
	}

	return end, nil
}
