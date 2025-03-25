package preprocess_test

import (
	"testing"

	"github.com/klever-io/klever-go/core/process/block/preprocess"
	"github.com/stretchr/testify/assert"
)

func TestProcessResults(t *testing.T) {
	t.Parallel()

	t.Run("Hashes", func(t *testing.T) {
		// Create test data
		txHashes := [][]byte{
			[]byte("hash1"),
			[]byte("hash2"),
			[]byte("hash3"),
		}
		size := int64(100)

		// Create processResults instance
		pr := preprocess.NewProcessResults(txHashes, size)

		// Test Hashes() method
		result := pr.Hashes()
		assert.Equal(t, txHashes, result)
		assert.Len(t, result, 3)
	})

	t.Run("Length", func(t *testing.T) {
		// Create test data
		txHashes := [][]byte{
			[]byte("hash1"),
			[]byte("hash2"),
			[]byte("hash3"),
		}
		size := int64(100)

		// Create processResults instance
		pr := preprocess.NewProcessResults(txHashes, size)

		// Test Length() method
		result := pr.Length()
		assert.Equal(t, 3, result)

		// Test with empty hashes
		prEmpty := preprocess.NewProcessResults([][]byte{}, 0)
		assert.Equal(t, 0, prEmpty.Length())
	})

	t.Run("Size", func(t *testing.T) {
		// Create test data
		txHashes := [][]byte{
			[]byte("hash1"),
			[]byte("hash2"),
		}
		size := int64(100)

		// Create processResults instance
		pr := preprocess.NewProcessResults(txHashes, size)

		// Test Size() method
		result := pr.Size()
		assert.Equal(t, size, result)
	})

	t.Run("IsInterfaceNil", func(t *testing.T) {
		// Create a non-nil instance
		pr := preprocess.NewProcessResults([][]byte{[]byte("hash1")}, 10)

		// Test IsInterfaceNil() method with non-nil instance
		result := pr.IsInterfaceNil()
		assert.False(t, result)

		// Test with nil instance
		pr = nil
		assert.True(t, pr.IsInterfaceNil())
	})
}
