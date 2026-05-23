package process

import (
	"errors"
	"unicode"
)

// Fixed BlacklistPeer reason identifiers. Callers must use these instead of
// concatenating err.Error() into the reason — the antiflood handler forwards
// the reason to its log stream, where attacker-controlled bytes would forge
// or amplify log lines (CWE-117). See KLC-2357.
const (
	BlacklistReasonUnmarshalable         = "unmarshalable_payload"
	BlacklistReasonDecompressFailed      = "decompress_failure"
	BlacklistReasonFactoryCreateFailed   = "invalid_factory_create"
	BlacklistReasonWrongVersion          = "invalid_version_or_chain"
	BlacklistReasonInvalidConsensus      = "invalid_consensus_message"
	BlacklistReasonInvalidHeartbeat      = "invalid_heartbeat_message"
	BlacklistReasonInconsistentHeartbeat = "inconsistent_heartbeat_message"
)

// MaxSanitizedBlacklistReason caps the rune length of an err string forwarded
// to a structured logger from a blacklist failure path, to deny length
// amplification through the log shipper.
const MaxSanitizedBlacklistReason = 256

// MaxSanitizeScanRunes caps how many runes SanitizeBlacklistReason will examine
// in the input. Without this, a long string of all-stripped runes (e.g. all
// control chars) would force O(len(s)) work even though the output cap never
// triggers — CPU/time amplification. 4× the output cap allows a generous 75 %
// strip ratio for legitimate verbose error strings while bounding worst-case
// work at a constant.
const MaxSanitizeScanRunes = MaxSanitizedBlacklistReason * 4

// SanitizeBlacklistReason strips control characters and Unicode "format" runes
// from s and caps the result at MaxSanitizedBlacklistReason runes. The filter
// removes:
//   - C0 + DEL + C1 controls (via unicode.IsControl) — including ESC (which
//     neuters ANSI CSI sequences) and U+0085 NEL;
//   - Cf format chars (via unicode.Is(unicode.Cf, r)) — zero-width joiners,
//     bidi controls (U+202A-U+202E / U+2066-U+2069, the Trojan-Source set),
//     U+FEFF BOM;
//   - U+2028 LINE SEPARATOR and U+2029 PARAGRAPH SEPARATOR, which some log
//     shippers treat as record separators.
//
// The function bounds total work at MaxSanitizeScanRunes input runes so a
// pathological input cannot force unbounded iteration even when every rune
// gets stripped.
func SanitizeBlacklistReason(s string) string {
	if s == "" {
		return s
	}
	out := make([]rune, 0, min(len(s), MaxSanitizedBlacklistReason))
	scanned := 0
	for _, r := range s {
		scanned++
		if scanned > MaxSanitizeScanRunes {
			break
		}
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) || r == '\u2028' || r == '\u2029' {
			continue
		}
		out = append(out, r)
		if len(out) == MaxSanitizedBlacklistReason {
			break
		}
	}
	return string(out)
}

// IsWrongVersionError reports whether err (or any wrapped err) indicates the
// peer sent an intercepted message with a transaction version / chain ID this
// node does not accept. The interceptors use this to decide whether to escalate
// a CheckValidity failure to a BlacklistPeer call.
func IsWrongVersionError(err error) bool {
	return errors.Is(err, ErrInvalidTransactionVersion) ||
		errors.Is(err, ErrInvalidChainID)
}
