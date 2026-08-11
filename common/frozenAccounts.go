package common

import "encoding/hex"

// ForkStatus exposes the fork flags consulted by the account freeze. It is declared
// here rather than imported from core because core/fork already imports common, so
// depending the other way would form an import cycle; the concrete forkController
// satisfies this interface structurally.
type ForkStatus interface {
	FixMarketBuyOverflow() bool
	FixAuditChangesV4() bool
}

// freezeWindow is the fork range over which an account is immobilised. frozenFrom is
// the flag that starts the freeze; thawedFrom is the flag that lifts it, or nil when
// the account is frozen indefinitely. Both are method expressions on ForkStatus, so a
// renamed or removed flag fails the build instead of silently never firing.
type freezeWindow struct {
	frozenFrom func(ForkStatus) bool
	thawedFrom func(ForkStatus) bool
}

// frozenAccounts is the canonical set of accounts from the GHSA-p7gw-2pcp-5pf8
// marketplace value-creation exploit, confirmed against the on-chain mint trace and
// consulted by IsAccountFrozen (the consensus freeze, enforced in
// txProcessor.ProcessTransaction and the proposer build path).
//
// It only immobilises (blocks outgoing txs); the supply correction is deferred.
// Freezing is reversible (thaw in a later fork); the correction is not.
//
// Each entry carries its own fork window, so freezing or thawing a further account is
// a change to this table alone — never a new conditional at a call site.
//
// Replay safety: the template keeps the epoch at 0 like every other fork; the REAL
// activation epoch is set in the operator config and MUST be a future epoch (after the
// last outgoing tx of every address below, after fleet rollout). Never ship a live
// config active from genesis or a past epoch, or a re-sync / import-db reindex replays
// these accounts' historical txs, rejects them, and diverges from chain history.
//
// Epoch ordering: an entry carrying a thawedFrom flag is only ever frozen while its
// freeze fork is active and its thaw fork is not, so the thaw epoch MUST be strictly
// greater than the freeze epoch or the window is empty and the account is never
// immobilised. config.EnableEpochs.Validate enforces that for the only pair in use
// today, fixMarketBuyOverflow / fixAuditChangesV4; a further pair needs its own check
// added there, since ForkStatus exposes flags rather than the epochs behind them.
var frozenAccounts = map[string]freezeWindow{
	// klv12n4z3ef86sfk2z97j4fhfta9f2xztsv6frr8faqj7l8q9kc0fcdsfjfqez (root / minter)
	"54ea28e527d4136508be955374afa54a8c25c19a48c674f412f7ce02db0f4e1b": {
		frozenFrom: ForkStatus.FixMarketBuyOverflow,
	},
	// klv1fzemma2s9d35l0hm38wt88srdyyqweufljqtps9jqkv32due3yqswqlfae (direct send from attacker)
	"48b3bdf5502b634fbefb89dcb39e036908076789fc80b0c0b205991537998901": {
		frozenFrom: ForkStatus.FixMarketBuyOverflow,
	},
	// klv1qfhu856w9gu8k9skay5sgquvl95erjygkmemycq07xspykqzm6vsqpty2m (direct send from attacker)
	"026fc3d34e2a387b1616e92904038cf96991c888b6f3b2600ff1a0125802de99": {
		frozenFrom: ForkStatus.FixMarketBuyOverflow,
	},
	// klv1wuug6007dnvgard8yvj5cydt70vuejm0kaasqrjs8r7rl7ftjexsglalf6 (direct recipient, 25.6M, idle)
	"77388d3dfe6cd88e8da723254c11abf3d9cccb6fb77b000e5038fc3ff92b964d": {
		frozenFrom: ForkStatus.FixMarketBuyOverflow,
	},
	// klv1hd58mwaz8cvyflkxwj3jewyqnuxnyp6sd3flc0tr0eqdc4ns343skngdjq (collector hop, ~125M KLV)
	// Third-party swap service, released again at the FixAuditChangesV4 fork.
	"bb687dbba23e1844fec674a32cb8809f0d3207506c53fc3d637e40dc56708d63": {
		frozenFrom: ForkStatus.FixMarketBuyOverflow,
		thawedFrom: ForkStatus.FixAuditChangesV4,
	},
	// klv1qh5swknt7z4zr9e73ax87vflpvv7u4z9cce4wsa5uccsjffq4tpquv67ej (NFT buy/sell operations account, attacker-controlled)
	"05e9075a6bf0aa21973e8f4c7f313f0b19ee5445c6335743b4e631092520aac2": {
		frozenFrom: ForkStatus.FixMarketBuyOverflow,
	},
}

// frozenAccountKeys is frozenAccounts keyed by raw public key (decoded once), so the
// hot-path IsAccountFrozen lookup is a zero-allocation map access with no per-call hex.
var frozenAccountKeys = buildFrozenAccountKeys()

func buildFrozenAccountKeys() map[string]freezeWindow {
	keys := make(map[string]freezeWindow, len(frozenAccounts))
	for hexKey, window := range frozenAccounts {
		raw, err := hex.DecodeString(hexKey)
		if err != nil {
			// A malformed key would silently fail to freeze; fail loud at start-up.
			panic("common: invalid frozen account hex key: " + hexKey)
		}
		if window.frozenFrom == nil {
			// An entry with no freeze flag would never immobilise; fail loud at start-up.
			panic("common: frozen account has no frozenFrom flag: " + hexKey)
		}
		keys[string(raw)] = window
	}
	return keys
}

// IsAccountFrozen reports whether the given raw public key belongs to an account that
// is consensus-frozen at the current fork state. The fork gating lives here rather than
// at the call sites, so an account thawing at a later fork than it froze at needs no
// change outside frozenAccounts.
func IsAccountFrozen(address []byte, forks ForkStatus) bool {
	if len(address) == 0 {
		return false
	}

	window, listed := frozenAccountKeys[string(address)]
	if !listed || !window.frozenFrom(forks) {
		return false
	}

	return window.thawedFrom == nil || !window.thawedFrom(forks)
}

// IsAccountFrozenAtAPI reports whether this node refuses to ingest a transaction from
// the given raw public key through its own API. It deliberately ignores frozenFrom:
// local ingestion policy applies from binary rollout, not from the freeze fork, so the
// gap between fleet rollout and the activation epoch stays covered. The thaw side is
// honoured, so an account released at a later fork is accepted here too.
func IsAccountFrozenAtAPI(address []byte, forks ForkStatus) bool {
	if len(address) == 0 {
		return false
	}

	window, listed := frozenAccountKeys[string(address)]
	if !listed {
		return false
	}

	return window.thawedFrom == nil || !window.thawedFrom(forks)
}
