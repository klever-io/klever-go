package common

import "encoding/hex"

// frozenAccounts is the canonical set of accounts from the GHSA-p7gw-2pcp-5pf8
// marketplace value-creation exploit, confirmed against the on-chain mint trace and
// consulted by IsAccountFrozen (the consensus freeze, gated by the FixMarketBuyOverflow
// fork flag, enforced in txProcessor.ProcessTransaction and the proposer build path).
//
// It only immobilises (blocks outgoing txs); the supply correction is deferred.
// Freezing is reversible (thaw in a later fork); the correction is not.
//
// Replay safety: the template keeps the epoch at 0 like every other fork; the REAL
// activation epoch is set in the operator config and MUST be a future epoch (after the
// last outgoing tx of every address below, after fleet rollout). Never ship a live
// config active from genesis or a past epoch, or a re-sync / import-db reindex replays
// these accounts' historical txs, rejects them, and diverges from chain history.
var frozenAccounts = map[string]struct{}{
	// klv12n4z3ef86sfk2z97j4fhfta9f2xztsv6frr8faqj7l8q9kc0fcdsfjfqez (root / minter)
	"54ea28e527d4136508be955374afa54a8c25c19a48c674f412f7ce02db0f4e1b": {},
	// klv1fzemma2s9d35l0hm38wt88srdyyqweufljqtps9jqkv32due3yqswqlfae (direct send from attacker)
	"48b3bdf5502b634fbefb89dcb39e036908076789fc80b0c0b205991537998901": {},
	// klv1qfhu856w9gu8k9skay5sgquvl95erjygkmemycq07xspykqzm6vsqpty2m (direct send from attacker)
	"026fc3d34e2a387b1616e92904038cf96991c888b6f3b2600ff1a0125802de99": {},
	// klv1wuug6007dnvgard8yvj5cydt70vuejm0kaasqrjs8r7rl7ftjexsglalf6 (direct recipient, 25.6M, idle)
	"77388d3dfe6cd88e8da723254c11abf3d9cccb6fb77b000e5038fc3ff92b964d": {},
	// klv1hd58mwaz8cvyflkxwj3jewyqnuxnyp6sd3flc0tr0eqdc4ns343skngdjq (collector hop, ~125M KLV)
	"bb687dbba23e1844fec674a32cb8809f0d3207506c53fc3d637e40dc56708d63": {},
	// klv1qh5swknt7z4zr9e73ax87vflpvv7u4z9cce4wsa5uccsjffq4tpquv67ej (NFT buy/sell operations account, attacker-controlled)
	"05e9075a6bf0aa21973e8f4c7f313f0b19ee5445c6335743b4e631092520aac2": {},
}

// frozenAccountKeys is frozenAccounts keyed by raw public key (decoded once), so the
// hot-path IsAccountFrozen lookup is a zero-allocation map access with no per-call hex.
var frozenAccountKeys = buildFrozenAccountKeys()

func buildFrozenAccountKeys() map[string]struct{} {
	keys := make(map[string]struct{}, len(frozenAccounts))
	for hexKey := range frozenAccounts {
		raw, err := hex.DecodeString(hexKey)
		if err != nil {
			// A malformed key would silently fail to freeze; fail loud at start-up.
			panic("common: invalid frozen account hex key: " + hexKey)
		}
		keys[string(raw)] = struct{}{}
	}
	return keys
}

// IsAccountFrozen reports whether the given raw public key belongs to a
// consensus-frozen account. Callers MUST gate this behind the FixMarketBuyOverflow
// fork flag so it only takes effect from the activation epoch onward.
func IsAccountFrozen(address []byte) bool {
	if len(address) == 0 {
		return false
	}
	_, frozen := frozenAccountKeys[string(address)]
	return frozen
}
