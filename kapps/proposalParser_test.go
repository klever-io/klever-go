package kapps

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseBool(t *testing.T) {
	tests := []struct {
		name    string
		input   []byte
		want    bool
		wantErr bool
	}{
		{"True", []byte("true"), true, false},
		{"False", []byte("false"), false, false},
		{"Invalid", []byte("notabool"), false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseBool(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got.Bool())
			}
		})
	}
}

func TestParseInt(t *testing.T) {
	tests := []struct {
		name    string
		input   []byte
		bitSize int
		want    int64
		wantErr bool
	}{
		{"Int8 Valid", []byte("127"), 8, 127, false},
		{"Int8 Overflow", []byte("128"), 8, 0, true},
		{"Int16 Valid", []byte("32767"), 16, 32767, false},
		{"Int32 Valid", []byte("-2147483648"), 32, -2147483648, false},
		{"Int64 Valid", []byte("9223372036854775807"), 64, 9223372036854775807, false},
		{"Invalid", []byte("notanint"), 64, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := parseInt(tt.bitSize)
			got, err := parser(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got.Int())
			}
		})
	}
}

func TestParseUint(t *testing.T) {
	tests := []struct {
		name    string
		input   []byte
		bitSize int
		want    uint64
		wantErr bool
	}{
		{"Uint8 Valid", []byte("255"), 8, 255, false},
		{"Uint8 Overflow", []byte("256"), 8, 0, true},
		{"Uint16 Valid", []byte("65535"), 16, 65535, false},
		{"Uint32 Valid", []byte("4294967295"), 32, 4294967295, false},
		{"Uint64 Valid", []byte("18446744073709551615"), 64, 18446744073709551615, false},
		{"Invalid", []byte("notauint"), 64, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := parseUint(tt.bitSize)
			got, err := parser(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got.Uint())
			}
		})
	}
}

func TestParseFloat(t *testing.T) {
	tests := []struct {
		name    string
		input   []byte
		bitSize int
		want    float64
		wantErr bool
	}{
		{"Float32 Valid", []byte("3.14159"), 32, 3.14159, false},
		{"Float64 Valid", []byte("-2.718281828"), 64, -2.718281828, false},
		{"Invalid", []byte("notafloat"), 64, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := parseFloat(tt.bitSize)
			got, err := parser(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.InDelta(t, tt.want, got.Float(), 1e-6)
			}
		})
	}
}

func TestParseComplex(t *testing.T) {
	tests := []struct {
		name    string
		input   []byte
		bitSize int
		want    complex128
		wantErr bool
	}{
		{"Complex64 Valid", []byte("1+2i"), 64, 1 + 2i, false},
		{"Complex128 Valid", []byte("-3.14+2.718i"), 128, -3.14 + 2.718i, false},
		{"Invalid", []byte("notacomplex"), 128, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := parseComplex(tt.bitSize)
			got, err := parser(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.InDelta(t, real(tt.want), real(got.Complex()), 1e-6)
				assert.InDelta(t, imag(tt.want), imag(got.Complex()), 1e-6)
			}
		})
	}
}

func TestParseString(t *testing.T) {
	input := []byte("Hello, World!")
	got, err := parseString(input)
	assert.NoError(t, err)
	assert.Equal(t, "Hello, World!", got.String())
}

func TestParseBytes(t *testing.T) {
	input := []byte{0x48, 0x65, 0x6C, 0x6C, 0x6F}
	got, err := parseBytes(input)
	assert.NoError(t, err)
	assert.Equal(t, input, got.Bytes())
}

func TestParsers(t *testing.T) {
	tests := []struct {
		name    string
		typ     EnumType
		input   []byte
		want    interface{}
		wantErr bool
	}{
		{"Bool", EnumType_Bool, []byte("true"), true, false},
		{"Int8", EnumType_Int8, []byte("127"), int64(127), false},
		{"Int16", EnumType_Int16, []byte("32767"), int64(32767), false},
		{"Int32", EnumType_Int32, []byte("-2147483648"), int64(-2147483648), false},
		{"Int64", EnumType_Int64, []byte("9223372036854775807"), int64(9223372036854775807), false},
		{"Uint8", EnumType_Uint8, []byte("255"), uint64(255), false},
		{"Uint16", EnumType_Uint16, []byte("65535"), uint64(65535), false},
		{"Uint32", EnumType_Uint32, []byte("4294967295"), uint64(4294967295), false},
		{"Uint64", EnumType_Uint64, []byte("18446744073709551615"), uint64(18446744073709551615), false},
		{"Float32", EnumType_Float32, []byte("3.14159"), float32(3.14159), false},
		{"Float64", EnumType_Float64, []byte("-2.718281828"), -2.718281828, false},
		{"Complex64", EnumType_Complex64, []byte("1+2i"), complex64(1 + 2i), false},
		{"Complex128", EnumType_Complex128, []byte("-3.14+2.718i"), complex128(-3.14 + 2.718i), false},
		{"String", EnumType_String, []byte("Hello, World!"), "Hello, World!", false},
		{"Bytes", EnumType_Bytes, []byte{0x48, 0x65, 0x6C, 0x6C, 0x6F}, []byte{0x48, 0x65, 0x6C, 0x6C, 0x6F}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser, ok := parsers[tt.typ]
			assert.True(t, ok, "Parser not found for type")
			got, err := parser(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				switch v := tt.want.(type) {
				case float32:
					assert.InDelta(t, v, float32(got.Float()), 1e-6)
				case float64:
					assert.InDelta(t, v, got.Float(), 1e-9)
				case complex64:
					gotComplex := complex64(got.Complex())
					assert.InDelta(t, real(v), real(gotComplex), 1e-6)
					assert.InDelta(t, imag(v), imag(gotComplex), 1e-6)
				case complex128:
					gotComplex := got.Complex()
					assert.InDelta(t, real(v), real(gotComplex), 1e-9)
					assert.InDelta(t, imag(v), imag(gotComplex), 1e-9)
				default:
					assert.Equal(t, tt.want, got.Interface())
				}
			}
		})
	}
}
