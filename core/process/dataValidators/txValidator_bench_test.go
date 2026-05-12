package dataValidators_test

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/dataValidators"
	"github.com/klever-io/klever-go/crypto"
	cryptoMock "github.com/klever-io/klever-go/crypto/mock"
	"github.com/klever-io/klever-go/crypto/signing"
	"github.com/klever-io/klever-go/crypto/signing/ed25519"
	"github.com/klever-io/klever-go/data/state"
)

var errBenchInvalidPubKey = errors.New("bench: invalid pubkey")

// benchKeyGen returns a KeyGenMock whose PublicKeyFromByteArray simulates
// `cost` rounds of sha256 work per call. cost=0 returns immediately and
// measures pure scaffolding overhead; cost>0 performs exactly `cost`
// sha256 rounds to approximate real elliptic-curve point decoding.
func benchKeyGen(cost int) *cryptoMock.KeyGenMock {
	return &cryptoMock.KeyGenMock{
		PublicKeyFromByteArrayMock: func(b []byte) (crypto.PublicKey, error) {
			if cost == 0 {
				return &cryptoMock.PublicKeyMock{}, nil
			}
			h := sha256.Sum256(b)
			for i := 1; i < cost; i++ {
				h = sha256.Sum256(h[:])
			}
			_ = h
			return &cryptoMock.PublicKeyMock{}, nil
		},
	}
}

// buildBenchValidator creates a tx validator whose account has `numSigners`
// signers, all sharing the same address. Threshold=1, each weight=1, so
// any one valid Verify clears the threshold check.
func buildBenchValidator(tb testing.TB, numSigners int, keygenCost int) (process.TxValidator, []byte) {
	tb.Helper()
	addressMock := bytes.Repeat([]byte{0xAB}, 32)

	signers := make([]*state.Key, numSigners)
	for i := range signers {
		signers[i] = &state.Key{Address: addressMock, Weight: 1}
	}

	adb := &mock.AccountsStub{
		GetExistingAccountCalled: func(_ []byte) (state.AccountHandler, error) {
			acc, err := state.NewUserAccount(addressMock)
			if err != nil {
				return nil, err
			}
			acc.Permissions = []*state.Permission{
				{
					Type:      state.Permission_Owner,
					Threshold: 1,
					Signers:   signers,
				},
			}
			return acc, nil
		},
	}

	v, err := dataValidators.NewTxValidator(
		adb,
		storageTest,
		getTxPoolsHolder(),
		&mock.WhiteListHandlerStub{},
		mock.NewPubkeyConverterMock(32),
		&cryptoMock.SingleSignerStub{
			VerifyCalled: func(_ crypto.PublicKey, _, _ []byte) error { return nil },
		},
		benchKeyGen(keygenCost),
		getKAppController(),
		core.MaxTxNonceDeltaAllowed,
	)
	if err != nil {
		tb.Fatalf("NewTxValidator: %v", err)
	}
	return v, addressMock
}

// makeBenchInterceptedTx wraps the validator handler + intercepted-data stub
// into the anonymous struct CheckTxValidity expects.
func makeBenchInterceptedTx(addr []byte) interface {
	process.InterceptedData
	process.TxValidatorHandler
} {
	return struct {
		process.InterceptedData
		process.TxValidatorHandler
	}{
		InterceptedData:    &mock.InterceptedDataStub{},
		TxValidatorHandler: getTxValidatorHandler(addr, 0, 0),
	}
}

// BenchmarkCheckTxValidity_Signers measures end-to-end CheckTxValidity time
// across signer counts. The keygen cost is 0 (mock returns immediately) so
// the result isolates the signer-loop overhead from real EC point decode.
func BenchmarkCheckTxValidity_Signers(b *testing.B) {
	for _, n := range []int{1, 2, 3, 5, 10, 20} {
		b.Run(fmt.Sprintf("signers=%d", n), func(b *testing.B) {
			v, addr := buildBenchValidator(b, n, 0)
			tx := makeBenchInterceptedTx(addr)

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := v.CheckTxValidity(tx); err != nil {
					b.Fatalf("unexpected err: %v", err)
				}
			}
		})
	}
}

// BenchmarkCheckTxValidity_SignersWithCost measures the same path with a
// realistic per-signer cost (50 sha256 iterations ~= a few µs of CPU work
// per pubkey decode), which is the regime where parallelism could help.
func BenchmarkCheckTxValidity_SignersWithCost(b *testing.B) {
	for _, n := range []int{1, 3, 10} {
		b.Run(fmt.Sprintf("signers=%d", n), func(b *testing.B) {
			v, addr := buildBenchValidator(b, n, 50)
			tx := makeBenchInterceptedTx(addr)

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := v.CheckTxValidity(tx); err != nil {
					b.Fatalf("unexpected err: %v", err)
				}
			}
		})
	}
}

// loadSignerKeysSerial replicates the new production loop body in isolation,
// for direct head-to-head benching against the parallel variant.
func loadSignerKeysSerial(signers []*state.Key, kg crypto.KeyGenerator) (map[string]crypto.PublicKey, error) {
	out := make(map[string]crypto.PublicKey, len(signers))
	for _, s := range signers {
		pk, err := kg.PublicKeyFromByteArray(s.Address)
		if err != nil {
			return nil, err
		}
		out[string(s.Address)] = pk
	}
	return out, nil
}

// loadSignerKeysParallel replicates the OLD production loop body (goroutine
// per signer + WaitGroup + Mutex). Kept here so we can bench both
// implementations against real ed25519 keygen and pick a breakeven.
func loadSignerKeysParallel(signers []*state.Key, kg crypto.KeyGenerator) (map[string]crypto.PublicKey, error) {
	out := make(map[string]crypto.PublicKey)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var loadErr error
	var errMu sync.Mutex
	for _, s := range signers {
		wg.Add(1)
		go func(addrPub string) {
			defer wg.Done()
			pk, err := kg.PublicKeyFromByteArray([]byte(addrPub))
			if err != nil {
				errMu.Lock()
				if loadErr == nil {
					loadErr = err
				}
				errMu.Unlock()
				return
			}
			mu.Lock()
			out[addrPub] = pk
			mu.Unlock()
		}(string(s.Address))
	}
	wg.Wait()
	if loadErr != nil {
		return nil, loadErr
	}
	return out, nil
}

// realKeyGenAndAddrs returns a real ed25519 KeyGenerator plus n valid 32-byte
// public-key addresses (so PublicKeyFromByteArray actually does the EC point
// decode work — random bytes would either fail decoding or take a degenerate
// fast-path).
func realKeyGenAndAddrs(n int) (crypto.KeyGenerator, [][]byte) {
	kg := signing.NewKeyGenerator(ed25519.NewEd25519())
	addrs := make([][]byte, n)
	for i := range addrs {
		_, pub := kg.GeneratePair()
		raw, err := pub.ToByteArray()
		if err != nil {
			panic(err)
		}
		addrs[i] = raw
	}
	return kg, addrs
}

func makeSigners(addrs [][]byte) []*state.Key {
	signers := make([]*state.Key, len(addrs))
	for i, a := range addrs {
		signers[i] = &state.Key{Address: a, Weight: 1}
	}
	return signers
}

// BenchmarkLoadSignerKeys_RealEd25519 measures the signer-key-decode loop in
// isolation, with the real ed25519 keygen used in production. Reports both
// implementations side-by-side so we can pin the breakeven for a possible
// `len(signers) > N` conditional.
func BenchmarkLoadSignerKeys_RealEd25519(b *testing.B) {
	for _, n := range []int{1, 2, 3, 4, 5, 6, 8, 10, 16, 20} {
		kg, addrs := realKeyGenAndAddrs(n)
		signers := makeSigners(addrs)

		b.Run(fmt.Sprintf("serial/signers=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := loadSignerKeysSerial(signers, kg); err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run(fmt.Sprintf("parallel/signers=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := loadSignerKeysParallel(signers, kg); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkCheckTxValidity_RealEd25519 covers the end-to-end CheckTxValidity
// path with real ed25519 keygen, varying signer count. Confirms the
// loop-isolation result holds when the rest of CheckTxValidity is included.
func BenchmarkCheckTxValidity_RealEd25519(b *testing.B) {
	for _, n := range []int{1, 2, 3, 5, 10, 20} {
		b.Run(fmt.Sprintf("signers=%d", n), func(b *testing.B) {
			kg, addrs := realKeyGenAndAddrs(n)
			// All signers share the first address so the signature loop
			// finds a match immediately (we're measuring the keygen loop,
			// not signature verification cost).
			sharedAddr := addrs[0]
			signers := make([]*state.Key, n)
			for i := range signers {
				signers[i] = &state.Key{Address: sharedAddr, Weight: 1}
			}
			adb := &mock.AccountsStub{
				GetExistingAccountCalled: func(_ []byte) (state.AccountHandler, error) {
					acc, err := state.NewUserAccount(sharedAddr)
					if err != nil {
						return nil, err
					}
					acc.Permissions = []*state.Permission{
						{Type: state.Permission_Owner, Threshold: 1, Signers: signers},
					}
					return acc, nil
				},
			}
			v, err := dataValidators.NewTxValidator(
				adb, storageTest, getTxPoolsHolder(), &mock.WhiteListHandlerStub{},
				mock.NewPubkeyConverterMock(32),
				&cryptoMock.SingleSignerStub{
					VerifyCalled: func(_ crypto.PublicKey, _, _ []byte) error { return nil },
				},
				kg, getKAppController(), core.MaxTxNonceDeltaAllowed,
			)
			if err != nil {
				b.Fatalf("NewTxValidator: %v", err)
			}
			tx := makeBenchInterceptedTx(sharedAddr)

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := v.CheckTxValidity(tx); err != nil {
					b.Fatalf("unexpected err: %v", err)
				}
			}
		})
	}
}

// Smoke test: ensure the bench scaffolding actually exercises the multi-signer
// path. Catches breakage in the helper before the bench reports misleading
// numbers (e.g., short-circuit returning early because of mock misconfig).
func TestBenchScaffolding_MultiSignerPathExecutes(t *testing.T) {
	t.Parallel()
	for _, n := range []int{1, 3, 10} {
		v, addr := buildBenchValidator(t, n, 0)
		tx := makeBenchInterceptedTx(addr)
		if err := v.CheckTxValidity(tx); err != nil {
			t.Fatalf("signers=%d: %v", n, err)
		}
	}
	// also verify error path: invalid pubkey decode propagates
	addressMock := bytes.Repeat([]byte{0xAB}, 32)
	signers := []*state.Key{{Address: addressMock, Weight: 1}, {Address: addressMock, Weight: 1}}
	adb := &mock.AccountsStub{
		GetExistingAccountCalled: func(_ []byte) (state.AccountHandler, error) {
			acc, err := state.NewUserAccount(addressMock)
			if err != nil {
				return nil, err
			}
			acc.Permissions = []*state.Permission{{Type: state.Permission_Owner, Threshold: 1, Signers: signers}}
			return acc, nil
		},
	}
	v, err := dataValidators.NewTxValidator(
		adb,
		storageTest,
		getTxPoolsHolder(),
		&mock.WhiteListHandlerStub{},
		mock.NewPubkeyConverterMock(32),
		&cryptoMock.SingleSignerStub{},
		&cryptoMock.KeyGenMock{
			PublicKeyFromByteArrayMock: func(_ []byte) (crypto.PublicKey, error) {
				return nil, errBenchInvalidPubKey
			},
		},
		getKAppController(),
		core.MaxTxNonceDeltaAllowed,
	)
	if err != nil {
		t.Fatalf("NewTxValidator: %v", err)
	}
	tx := makeBenchInterceptedTx(addressMock)
	if err := v.CheckTxValidity(tx); !errors.Is(err, errBenchInvalidPubKey) {
		t.Fatalf("expected %v, got %v", errBenchInvalidPubKey, err)
	}
}
