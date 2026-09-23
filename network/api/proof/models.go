package proof

// ProofResponse is a Merkle inclusion proof for one account under a state root.
//
// Proof is the encoded Patricia trie nodes from the root down to the leaf, hex-encoded.
// An external verifier hashes each node with the chain hasher (blake2b; config hasher.type)
// over the raw decoded bytes: hasher.Compute(string(node)). The first node must hash to
// RootHash. The key is the decoded bech32 address, with nibbles reversed and a hex
// terminator appended, which is how the accounts trie walks a key. Value is the leaf
// bytes at that key (the protobuf-encoded account), hex-encoded.
//
// A valid proof establishes inclusion under RootHash only. Take RootHash from a
// finalized block header. A historical root is served only while that root is still
// stored; pruned nodes return a clear error instead of a proof.
type ProofResponse struct {
	// Hex-encoded trie nodes from the root down to the account leaf.
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
