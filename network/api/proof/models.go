package proof

// ProofResponse is a Merkle inclusion proof for one account under a state root.
// The full client-side verification steps are documented on state.MerkleProof.
type ProofResponse struct {
	// Hex-encoded trie nodes from the root down to the account leaf. Each node is a
	// protobuf body plus a trailing type byte (0 extension CollapsedEn{Key, EncodedChild},
	// 1 leaf CollapsedLn{Key, Value}, 2 branch CollapsedBn{EncodedChildren}, 17 slots).
	// The first node hashes to rootHash with unkeyed blake2b-256 over the full bytes; each
	// next node hashes to the child selected by the key. The key is the address bytes
	// reversed, each byte as low then high nibble, plus terminator nibble 16. A branch
	// consumes one nibble, an extension its Key; the leaf Key must equal the rest.
	Proof []string `json:"proof"`
	// Hex protobuf-encoded account leaf. /proof/verify does not check this field;
	// compare it with the leaf in the verified proof nodes.
	Value string `json:"value"`
	// Hex state root the proof is under. Trust it only when it matches the accounts
	// TrieRoot of a finalized block header obtained independently of this node.
	RootHash string `json:"rootHash"`
}

// VerifyRequest is the body of POST /proof/verify.
// RootHash and each proof node are hex. Address is a bech32 account address.
type VerifyRequest struct {
	// Hex state root, taken from a finalized block header obtained independently.
	RootHash string `json:"rootHash"`
	// Bech32 account address.
	Address string `json:"address"`
	// Hex-encoded trie nodes, as returned in ProofResponse.proof.
	Proof []string `json:"proof"`
}

// VerifyResponse is the boolean result of POST /proof/verify.
// Ok is true only when the proof shows the account included under RootHash.
type VerifyResponse struct {
	// True when the proof shows the address included under rootHash. It does not
	// authenticate the root itself or any separately supplied account value.
	Ok bool `json:"ok"`
}
