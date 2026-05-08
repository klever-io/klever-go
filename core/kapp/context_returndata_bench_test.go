package kapp_test

import (
	"crypto/rand"
	"fmt"
	"testing"

	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/data/block"
)

// genReturnData returns n byte slices of size bytes each, populated with
// random data so the compiler can't constant-fold the copies away.
func genReturnData(n, size int) [][]byte {
	out := make([][]byte, n)
	for i := range out {
		out[i] = make([]byte, size)
		_, _ = rand.Read(out[i])
	}
	return out
}

func newCtxWithReturnData(n, size int) kapp.KappContext {
	ctx := kapp.NewKappContext(kapp.ArgsNewKAppContext{
		OriginalSender: []byte("sender"),
		ContractID:     0,
		Block:          &block.Block{},
	})
	ctx.SetReturnData(genReturnData(n, size))
	return ctx
}

// BenchmarkGetAndClearReturnData measures the per-call cost of pulling
// return data out of the context. Sweep across realistic SC return shapes:
//
//   - (1, 32)     — single small item (typical: assetID, proposalID, orderID)
//   - (5, 32)     — small array of small items (typical: minted-token IDs)
//   - (10, 64)    — moderate array of moderate items (e.g., transfer batch IDs)
//   - (50, 256)   — large array (uncommon but plausible for view fns)
//   - (1, 4096)   — single large item (e.g., serialized struct)
func BenchmarkGetAndClearReturnData(b *testing.B) {
	cases := []struct{ n, size int }{
		{1, 32},
		{5, 32},
		{10, 64},
		{50, 256},
		{1, 4096},
	}
	for _, c := range cases {
		b.Run(fmt.Sprintf("n=%d/size=%d", c.n, c.size), func(b *testing.B) {
			// Refill the context every call so each iteration has data
			// to drain. We measure GetAndClearReturnData *plus* the refill
			// (SetReturnData), then subtract the refill cost separately.
			data := genReturnData(c.n, c.size)
			ctx := kapp.NewKappContext(kapp.ArgsNewKAppContext{
				OriginalSender: []byte("sender"),
				ContractID:     0,
				Block:          &block.Block{},
			})

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				ctx.SetReturnData(data)
				_ = ctx.GetAndClearReturnData()
			}
		})
	}
}

// BenchmarkSetReturnData_Only isolates the refill cost so the
// GetAndClearReturnData number above can be interpreted (subtract this
// from the combined number to get the Get cost alone).
func BenchmarkSetReturnData_Only(b *testing.B) {
	cases := []struct{ n, size int }{
		{1, 32},
		{5, 32},
		{10, 64},
		{50, 256},
		{1, 4096},
	}
	for _, c := range cases {
		b.Run(fmt.Sprintf("n=%d/size=%d", c.n, c.size), func(b *testing.B) {
			data := genReturnData(c.n, c.size)
			ctx := kapp.NewKappContext(kapp.ArgsNewKAppContext{
				OriginalSender: []byte("sender"),
				ContractID:     0,
				Block:          &block.Block{},
			})

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				ctx.SetReturnData(data)
			}
		})
	}
}
