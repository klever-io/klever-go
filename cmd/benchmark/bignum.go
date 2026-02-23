package main

// RunBigNumBenchmark measures the arithmetic throughput that a Kleverchain
// validator relies on for consensus and cryptographic operations:
//
//   - Modular exponentiation (big.Int.Exp): the core of RSA-style operations
//     and the most expensive primitive in many ZK-proof verifiers.  Tested
//     with a 2048-bit modulus to match production key sizes.
//
//   - Modular multiplication (big.Int.Mul + Mod): repeated squarings that
//     appear in ECC scalar multiplication and Pedersen commitments.
//
//   - Float64 transcendentals (math.Pow / math.Sqrt / math.Log): used in
//     economic models, reward-curve calculations, and staking formulas.
//     Measures the raw FPU throughput of the CPU.
//
//   - Integer division throughput (uint64): cheap baseline that reveals
//     general ALU pipeline health; slowdowns here indicate throttling.
//
// All timed loops run for bigNumDuration seconds so results are stable across
// machines with very different clock speeds.

import (
	"fmt"
	"math"
	"math/big"
	"os"
	"strings"
	"time"
)

const bigNumDuration = 3 * time.Second

// BigNumResult holds all big-number / floating-point benchmark metrics.
type BigNumResult struct {
	ModExpOpsPerSec  float64 // 2048-bit modular exponentiations per second
	ModMulOpsPerSec  float64 // 2048-bit modular multiplications per second
	Float64OpsPerSec float64 // math.Pow + math.Sqrt + math.Log ops per second
	IntDivOpsPerSec  float64 // uint64 integer divisions per second
}

// RunBigNumBenchmark executes all four arithmetic sub-benchmarks.
func RunBigNumBenchmark() (*BigNumResult, error) {
	result := &BigNumResult{}

	// 1. Modular exponentiation (2048-bit) ------------------------------------
	clearLine("  BigNum: modular exponentiation (2048-bit, %s)...", bigNumDuration)
	ops, err := bigModExp()
	if err != nil {
		return nil, fmt.Errorf("mod exp: %w", err)
	}
	result.ModExpOpsPerSec = ops

	// 2. Modular multiplication (2048-bit) ------------------------------------
	clearLine("  BigNum: modular multiplication (2048-bit, %s)...", bigNumDuration)
	ops, err = bigModMul()
	if err != nil {
		return nil, fmt.Errorf("mod mul: %w", err)
	}
	result.ModMulOpsPerSec = ops

	// 3. Float64 transcendentals ----------------------------------------------
	clearLine("  BigNum: float64 transcendentals (pow/sqrt/log, %s)...", bigNumDuration)
	ops, err = floatTranscendentals()
	if err != nil {
		return nil, fmt.Errorf("float transcendentals: %w", err)
	}
	result.Float64OpsPerSec = ops

	// 4. Integer division -----------------------------------------------------
	clearLine("  BigNum: integer division (uint64, %s)...", bigNumDuration)
	ops, err = intDivThroughput()
	if err != nil {
		return nil, fmt.Errorf("int div: %w", err)
	}
	result.IntDivOpsPerSec = ops

	fmt.Fprintf(os.Stderr, "  %s\r", strings.Repeat(" ", 60))
	return result, nil
}

// ---------------------------------------------------------------------------
// 2048-bit modular exponentiation
// ---------------------------------------------------------------------------

// bigModExp measures big.Int.Exp(base, exp, mod) throughput using fixed
// 2048-bit operands that are representative of production cryptographic use.
func bigModExp() (float64, error) {
	// Fixed 2048-bit prime modulus (MODP group 14 from RFC 3526).
	modHex := "FFFFFFFFFFFFFFFFC90FDAA22168C234C4C6628B80DC1CD1" +
		"29024E088A67CC74020BBEA63B139B22514A08798E3404DD" +
		"EF9519B3CD3A431B302B0A6DF25F14374FE1356D6D51C245" +
		"E485B576625E7EC6F44C42E9A637ED6B0BFF5CB6F406B7ED" +
		"EE386BFB5A899FA5AE9F24117C4B1FE649286651ECE45B3D" +
		"C2007CB8A163BF0598DA48361C55D39A69163FA8FD24CF5F" +
		"83655D23DCA3AD961C62F356208552BB9ED529077096966D" +
		"670C354E4ABC9804F1746C08CA18217C32905E462E36CE3B" +
		"E39E772C180E86039B2783A2EC07A28FB5C55DF06F4C52C9" +
		"DE2BCBF6955817183995497CEA956AE515D2261898FA0510" +
		"15728E5A8AACAA68FFFFFFFFFFFFFFFF"
	mod := new(big.Int)
	if _, ok := mod.SetString(modHex, 16); !ok {
		return 0, fmt.Errorf("bigModExp: failed to parse modulus hex constant")
	}

	base := new(big.Int).SetUint64(65537)
	exp := new(big.Int).Sub(mod, big.NewInt(1)) // worst-case exponent = mod-1
	result := new(big.Int)

	inc := big.NewInt(1)
	deadline := time.Now().Add(bigNumDuration)
	var count int64
	for time.Now().Before(deadline) {
		result.Exp(base, exp, mod)
		base.Add(base, inc) // vary base to avoid trivial shortcuts
		count++
	}
	_ = result

	return float64(count) / bigNumDuration.Seconds(), nil
}

// ---------------------------------------------------------------------------
// 2048-bit modular multiplication
// ---------------------------------------------------------------------------

// bigModMul measures the throughput of repeated big.Int Mul+Mod, which is the
// inner loop of Montgomery multiplication and ECC point doubling.
func bigModMul() (float64, error) {
	modHex := "FFFFFFFFFFFFFFFFC90FDAA22168C234C4C6628B80DC1CD1" +
		"29024E088A67CC74020BBEA63B139B22514A08798E3404DD" +
		"EF9519B3CD3A431B302B0A6DF25F14374FE1356D6D51C245" +
		"E485B576625E7EC6F44C42E9A637ED6B0BFF5CB6F406B7ED" +
		"EE386BFB5A899FA5AE9F24117C4B1FE649286651ECE45B3D" +
		"C2007CB8A163BF0598DA48361C55D39A69163FA8FD24CF5F" +
		"83655D23DCA3AD961C62F356208552BB9ED529077096966D" +
		"670C354E4ABC9804F1746C08CA18217C32905E462E36CE3B" +
		"E39E772C180E86039B2783A2EC07A28FB5C55DF06F4C52C9" +
		"DE2BCBF6955817183995497CEA956AE515D2261898FA0510" +
		"15728E5A8AACAA68FFFFFFFFFFFFFFFF"
	mod := new(big.Int)
	if _, ok := mod.SetString(modHex, 16); !ok {
		return 0, fmt.Errorf("bigModMul: failed to parse modulus hex constant")
	}

	a := new(big.Int).SetUint64(0xDEADBEEFCAFEBABE)
	b := new(big.Int).SetUint64(0xFEEDFACEDEADC0DE)
	tmp := new(big.Int)

	deadline := time.Now().Add(bigNumDuration)
	var count int64
	for time.Now().Before(deadline) {
		tmp.Mul(a, b)
		tmp.Mod(tmp, mod)
		a.Set(tmp)
		count++
	}
	_ = tmp

	return float64(count) / bigNumDuration.Seconds(), nil
}

// ---------------------------------------------------------------------------
// Float64 transcendentals
// ---------------------------------------------------------------------------

// floatTranscendentals measures how many (math.Pow + math.Sqrt + math.Log)
// triplets per second the FPU can execute.  A triplet counts as one "op".
func floatTranscendentals() (float64, error) {
	var sink float64
	x := 2.0
	deadline := time.Now().Add(bigNumDuration)
	var count int64
	for time.Now().Before(deadline) {
		sink += math.Pow(x, 1.0001)
		sink += math.Sqrt(x + float64(count))
		sink += math.Log(x + 1)
		x = sink - math.Trunc(sink) + 1.0 // keep x in [1,2) to avoid NaN
		count++
	}
	if sink == 0 {
		x = 1
		_ = x
	}

	return float64(count) / bigNumDuration.Seconds(), nil
}

// ---------------------------------------------------------------------------
// Integer division
// ---------------------------------------------------------------------------

// intDivThroughput measures raw uint64 division throughput.
// Division is chosen over multiplication because it is non-pipelined on most
// microarchitectures and thus a more sensitive indicator of ALU health.
func intDivThroughput() (float64, error) {
	var a uint64 = 0xFEDCBA9876543210
	var b uint64 = 0x0123456789ABCDEF
	var sink uint64

	deadline := time.Now().Add(bigNumDuration)
	var count int64
	for time.Now().Before(deadline) {
		sink += a / b
		a = a*6364136223846793005 + 1442695040888963407 // LCG to vary operands
		b = b*2862933555777941757 + 3037000499
		if b == 0 {
			b = 1
		}
		count++
	}
	if sink == 0 {
		a = 1
		_ = a
	}

	return float64(count) / bigNumDuration.Seconds(), nil
}
