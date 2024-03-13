package batch_test

import (
	"fmt"
	"math"
	"testing"

	"github.com/klever-io/klever-go/data/batch"
	"github.com/klever-io/klever-go/tools/marshal/factory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func addRandom(b [][]byte, len int) [][]byte {
	for i := 0; i < len; i++ {
		data := []byte(fmt.Sprintf("%04d-abcdefghijklmnopqrstuvwxyz%f ", i, float64(i)*math.Pi))
		b = append(b, data)
	}
	return b
}

func TestGZIP(t *testing.T) {
	internalMarshalizer, err := factory.NewMarshalizer(factory.ProtoMarshalizer)
	require.Nil(t, err)

	buffer := make([][]byte, 0)
	buffer = addRandom(buffer, 3000)
	ba := batch.New(buffer...)
	ba.Algo = batch.CType_GZip

	data, _ := internalMarshalizer.Marshal(ba)
	err = ba.Compress(internalMarshalizer)
	assert.Nil(t, err)

	newData, _ := internalMarshalizer.Marshal(ba)

	dataCompressed := batch.Batch{Algo: batch.CType_GZip}
	err = internalMarshalizer.Unmarshal(&dataCompressed, newData)
	assert.Nil(t, err)
	assert.True(t, dataCompressed.IsCompressed)
	err = dataCompressed.Decompress(internalMarshalizer)
	assert.Nil(t, err)
	assert.False(t, dataCompressed.IsCompressed)

	assert.Equal(t, buffer, dataCompressed.Data)

	fmt.Printf("Size: %d, New Size %d\n", len(data), len(newData))
}

func BenchmarkCompress(b *testing.B) {
	algos := []batch.CType{batch.CType_GZip, batch.CType_LZ4}
	sizes := []int{100, 1000, 3000}
	b.ReportAllocs()
	internalMarshalizer, _ := factory.NewMarshalizer(factory.ProtoMarshalizer)

	compressionResult := make(map[string]float64)

	for _, s := range sizes {
		for _, a := range algos {
			b.Run(fmt.Sprintf("%s-%d", a, s), func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					b.StopTimer()
					buffer := addRandom(make([][]byte, 0), s)
					ba := batch.New(buffer...)
					ba.Algo = a
					size := ba.DataSize
					b.StartTimer()
					err := ba.Compress(internalMarshalizer)
					if err != nil {
						b.Errorf("Compress error: %v", err)
					}
					b.StopTimer()

					compressionResult[b.Name()] = (1 - float64(ba.DataSize)/float64(size)) * 100
				}
			})
		}
	}
	b.Errorf("%v", compressionResult)
}

func BenchmarkDecompress(b *testing.B) {
	algos := []batch.CType{batch.CType_GZip, batch.CType_LZ4}
	sizes := []int{100, 1000, 3000}
	b.ReportAllocs()
	internalMarshalizer, _ := factory.NewMarshalizer(factory.ProtoMarshalizer)

	for _, s := range sizes {
		for _, a := range algos {
			b.Run(fmt.Sprintf("%s-%d", a, s), func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					b.StopTimer()
					buffer := addRandom(make([][]byte, 0), s)
					ba := batch.New(buffer...)
					ba.Algo = a
					err := ba.Compress(internalMarshalizer)
					if err != nil {
						b.Errorf("Compress error: %v", err)
					}
					b.StartTimer()
					err = ba.Decompress(internalMarshalizer)
					if err != nil {
						b.Errorf("Compress error: %v", err)
					}
				}
			})
		}
	}
}
