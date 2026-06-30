// Package bitmap holds small, dependency-free helpers for working with the
// public-key bitmaps used throughout consensus and multi-signature validation.
package bitmap

// HasPaddingBitsSet reports whether the bitmap has any set bit at a position that
// does not map to a configured public key. For consensus/validator counts that are
// not a multiple of 8 these trailing padding bits must be zero; otherwise a caller
// could inflate the apparent signer set without a corresponding aggregate signer
// (KLR-04). It also catches set bits in any byte beyond the one holding numValidBits,
// so it stays correct even when the bitmap is longer than strictly required.
func HasPaddingBitsSet(bitmap []byte, numValidBits int) bool {
	for bitIndex := numValidBits; bitIndex < len(bitmap)*8; bitIndex++ {
		if bitmap[bitIndex/8]&(1<<uint8(bitIndex%8)) != 0 { // #nosec G115 - bitIndex%8 is always < 8
			return true
		}
	}

	return false
}
