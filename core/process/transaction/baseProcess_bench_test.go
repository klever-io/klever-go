package transaction_test

import (
	"fmt"
	"sync"
	"testing"

	"github.com/klever-io/klever-go/core/process/transaction"
	"github.com/klever-io/klever-go/crypto"
	"github.com/klever-io/klever-go/crypto/signing"
	"github.com/klever-io/klever-go/crypto/signing/ed25519"
	"github.com/klever-io/klever-go/data/state"
)

// loadSignerPublicKeysGoroutine is the old goroutine-based implementation for comparison.
func loadSignerPublicKeysGoroutine(keyGen crypto.KeyGenerator, signers []*state.Key) (map[string]crypto.PublicKey, error) {
	signersPub := make(map[string]crypto.PublicKey)
	var wg sync.WaitGroup
	var mu sync.Mutex
	errChan := make(chan error, len(signers))

	for _, signer := range signers {
		wg.Add(1)
		go func(addrPub []byte) {
			defer wg.Done()
			senderPubKey, err := keyGen.PublicKeyFromByteArray(addrPub)
			if err != nil {
				errChan <- fmt.Errorf("invalid signer address: %w", err)
				return
			}
			mu.Lock()
			signersPub[string(addrPub)] = senderPubKey
			mu.Unlock()
		}(signer.Address)
	}

	wg.Wait()
	close(errChan)

	if err := <-errChan; err != nil {
		return nil, err
	}

	return signersPub, nil
}

func generateSigners(b *testing.B, n int) []*state.Key {
	b.Helper()
	keyGen := signing.NewKeyGenerator(ed25519.NewEd25519())
	signers := make([]*state.Key, n)
	for i := range n {
		seed := make([]byte, 32)
		seed[0] = byte(i)
		privKey, err := keyGen.PrivateKeyFromByteArray(seed)
		if err != nil {
			b.Fatal(err)
		}
		pubBytes, err := privKey.GeneratePublic().ToByteArray()
		if err != nil {
			b.Fatal(err)
		}
		signers[i] = &state.Key{Address: pubBytes, Weight: 1}
	}
	return signers
}

func BenchmarkLoadSignerPublicKeys(b *testing.B) {
	for _, count := range []int{1, 2, 5, 10} {
		signers := generateSigners(b, count)
		txProc := transaction.NewTxProcessorExportTest()
		keyGen := txProc.KeyGen()

		b.Run(fmt.Sprintf("sequential/%d_signers", count), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				_, err := txProc.LoadSignerPublicKeys(signers)
				if err != nil {
					b.Fatal(err)
				}
			}
		})

		b.Run(fmt.Sprintf("goroutine/%d_signers", count), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				_, err := loadSignerPublicKeysGoroutine(keyGen, signers)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
