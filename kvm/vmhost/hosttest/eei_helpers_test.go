package hostCoretest

import (
	"testing"

	"github.com/klever-io/klever-go/vmcommon"
	"github.com/stretchr/testify/assert"
)

func TestBaseOpsAPI_validateToken(t *testing.T) {
	var result bool
	result = vmcommon.ValidateToken([]byte("KLVRIDEFL-08d8eff"))
	assert.False(t, result)
	result = vmcommon.ValidateToken([]byte("KLVRIDEFL-08d8e"))
	assert.False(t, result)
	result = vmcommon.ValidateToken([]byte("KLVRIDEFL08d8ef"))
	assert.False(t, result)
	result = vmcommon.ValidateToken([]byte("KLVRIDEFl-08d8ef"))
	assert.False(t, result)
	result = vmcommon.ValidateToken([]byte("KLVRIDEF*-08d8ef"))
	assert.False(t, result)
	result = vmcommon.ValidateToken([]byte("KLVRIDEFL-08d8eF"))
	assert.False(t, result)
	result = vmcommon.ValidateToken([]byte("KLVRIDEFL-08d*ef"))
	assert.False(t, result)

	result = vmcommon.ValidateToken([]byte("ALC6258d2"))
	assert.False(t, result)
	result = vmcommon.ValidateToken([]byte("AL-C6258d2"))
	assert.False(t, result)
	result = vmcommon.ValidateToken([]byte("alc-6258d2"))
	assert.False(t, result)
	result = vmcommon.ValidateToken([]byte("ALC-6258D2"))
	assert.False(t, result)
	result = vmcommon.ValidateToken([]byte("ALC-6258d2ff"))
	assert.False(t, result)
	result = vmcommon.ValidateToken([]byte("AL-6258d2"))
	assert.False(t, result)
	result = vmcommon.ValidateToken([]byte("ALCCCCCCCCC-6258d2"))
	assert.False(t, result)

	result = vmcommon.ValidateToken([]byte("KLVDEF2-08EF"))
	assert.True(t, result)
	result = vmcommon.ValidateToken([]byte("KLVDEFL-08EF"))
	assert.True(t, result)
	result = vmcommon.ValidateToken([]byte("ALC-58D2"))
	assert.True(t, result)
	result = vmcommon.ValidateToken([]byte("ALC12-58D2"))
	assert.True(t, result)
	result = vmcommon.ValidateToken([]byte("12345-58D2"))
	assert.True(t, result)
}
