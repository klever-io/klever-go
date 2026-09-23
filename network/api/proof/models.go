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
	Proof    []string `json:"proof"`
	Value    string   `json:"value"`
	RootHash string   `json:"rootHash"`
}

// VerifyRequest is the body of POST /proof/verify.
// RootHash and each proof node are hex. Address is a bech32 account address.
type VerifyRequest struct {
	RootHash string   `json:"rootHash"`
	Address  string   `json:"address"`
	Proof    []string `json:"proof"`
}

// VerifyResponse is the boolean result of POST /proof/verify.
// Ok is true only when the proof shows the account included under RootHash.
type VerifyResponse struct {
	Ok bool `json:"ok"`
}
