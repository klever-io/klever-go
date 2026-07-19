package validators

import (
	"strconv"
	"strings"

	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/data/state"
)

// versionWildcard is the versions.versionsByEpochs entry meaning "no specific version required"
const versionWildcard = "*"

// Outdated validators are only demoted when at least 2/3 of the electable set
// (elected + eligible) already attested a satisfying version, so enforcement can
// never demote more than 1/3 of the validators that carry consensus.
const (
	versionEnforcementNumerator   = 2
	versionEnforcementDenominator = 3
)

// versionEnforcement is the per-epoch decision on demoting validators without a
// satisfying attested node version.
type versionEnforcement struct {
	// active reports that a specific node version is required for the target epoch
	// (versionAttestation fork enabled and a non-wildcard versionsByEpochs entry)
	active bool
	// demote reports that the safety guards passed and unsatisfied validators are
	// demoted to the observer list this epoch. When active is set but demote is not,
	// no new demotions happen, and already-demoted observers stay demoted until they
	// attest a satisfying version (prevents unattested restore/demote oscillation).
	demote bool
	// required is the version demanded for the target epoch
	required string
	// minAttestedEpoch is the minimum AttestedEpoch for an attestation to count.
	// It is the start epoch of the versionsByEpochs entry preceding the required one,
	// so every validator must have (re-)attested at least once per release cycle;
	// a single inflated attestation (e.g. "99.9.9") cannot grant a permanent exemption.
	minAttestedEpoch uint32
}

// isSatisfiedBy reports whether the validator's attestation fulfills the active requirement
func (e versionEnforcement) isSatisfiedBy(val *ValidatorData) bool {
	return val.AttestedEpoch >= e.minAttestedEpoch &&
		versionSatisfies(val.AttestedVersion, e.required)
}

// requiredVersionForEpoch returns the version required for the given epoch based on the
// versions.versionsByEpochs configuration, the minimum epoch an attestation must have
// been made in to count for it, and whether a specific version is required at all.
// A wildcard or missing entry means no requirement (mechanism stays dormant).
//
// NOTE: this table is the same release-shipped config validated and consumed by
// headerCheck.headerIntegrityVerifier; both lookups must keep agreeing on which
// entry covers an epoch.
func (v *validatorsKApp) requiredVersionForEpoch(epoch uint32) (string, uint32, bool) {
	current := -1
	previous := -1
	for i, entry := range v.versionsByEpochs {
		if entry.StartEpoch > epoch {
			continue
		}
		if current < 0 || entry.StartEpoch > v.versionsByEpochs[current].StartEpoch {
			previous = current
			current = i
			continue
		}
		if previous < 0 || entry.StartEpoch > v.versionsByEpochs[previous].StartEpoch {
			previous = i
		}
	}
	if current < 0 {
		return "", 0, false
	}

	version := v.versionsByEpochs[current].Version
	if version == versionWildcard || len(version) == 0 {
		return "", 0, false
	}

	minAttestedEpoch := uint32(0)
	if previous >= 0 {
		minAttestedEpoch = v.versionsByEpochs[previous].StartEpoch
	}

	return version, minAttestedEpoch, true
}

// parseSemver parses "v1.2.3", "1.2.3", "1.2" or "1" into numeric components and
// reports whether a pre-release suffix ("-rc1") is present; build metadata ("+meta")
// is ignored.
func parseSemver(version string) ([3]uint64, bool, bool) {
	var out [3]uint64

	trimmed := strings.TrimSpace(version)
	trimmed = strings.TrimPrefix(trimmed, "v")
	trimmed = strings.TrimPrefix(trimmed, "V")

	if idx := strings.IndexByte(trimmed, '+'); idx >= 0 {
		trimmed = trimmed[:idx]
	}
	hasPrerelease := false
	if idx := strings.IndexByte(trimmed, '-'); idx >= 0 {
		hasPrerelease = true
		trimmed = trimmed[:idx]
	}

	parts := strings.Split(trimmed, ".")
	if len(parts) == 0 || len(parts) > 3 {
		return out, false, false
	}

	for i, part := range parts {
		number, err := strconv.ParseUint(part, 10, 32)
		if err != nil {
			return out, false, false
		}
		out[i] = number
	}

	return out, hasPrerelease, true
}

// versionSatisfies returns true when the attested version fulfills the required one:
// semantic compare (attested >= required) when both parse as versions, exact match
// otherwise. On equal numeric components, a pre-release attestation does not satisfy
// a release requirement (semver: 1.9.0-rc1 < 1.9.0).
func versionSatisfies(attested, required string) bool {
	if attested == required {
		return true
	}

	attestedParts, attestedPre, okAttested := parseSemver(attested)
	requiredParts, requiredPre, okRequired := parseSemver(required)
	if !okAttested || !okRequired {
		return false
	}

	for i := 0; i < len(attestedParts); i++ {
		if attestedParts[i] != requiredParts[i] {
			return attestedParts[i] > requiredParts[i]
		}
	}

	return !attestedPre || requiredPre
}

// computeVersionEnforcement decides, once per end-of-epoch processing, whether validators
// without a satisfying attested version should be demoted to the observer list when
// preparing the lists for targetEpoch. Demotion requires the versionAttestation fork to
// be active, a non-wildcard required version for targetEpoch, and two safety guards over
// the electable set (elected + eligible — the population demotion actually removes from):
//
//  1. supermajority: at least 2/3 of the electable set already attested a satisfying
//     version, bounding demotion to less than 1/3 of the consensus-carrying validators;
//  2. floor: the attested validators alone can still satisfy the nodes shuffler's
//     minimum electable count (minElectableNodes, from genesis MinNumberOfNodes), so
//     demotion can never reduce elected+eligible below what EpochStartPrepare needs.
//
// Waiting/inactive/jailed validators neither count towards the guards nor get demoted.
func (v *validatorsKApp) computeVersionEnforcement(
	app state.KAppAccountHandler,
	validatorInfos []*state.ValidatorInfo,
	targetEpoch uint32,
) versionEnforcement {
	if !v.forkController.VersionAttestation() {
		return versionEnforcement{}
	}

	required, minAttestedEpoch, ok := v.requiredVersionForEpoch(targetEpoch)
	if !ok {
		return versionEnforcement{}
	}

	enforcement := versionEnforcement{
		active:           true,
		required:         required,
		minAttestedEpoch: minAttestedEpoch,
	}

	electable := 0
	satisfied := 0
	for _, info := range validatorInfos {
		switch info.List {
		case string(core.ElectedList), string(core.EligibleList):
		default:
			continue
		}
		electable++

		val, err := v.getValidator(app, info.OwnerAddress)
		if err != nil {
			continue
		}
		if enforcement.isSatisfiedBy(val) {
			satisfied++
		}
	}

	hasSupermajority := electable > 0 &&
		satisfied*versionEnforcementDenominator >= electable*versionEnforcementNumerator
	holdsFloor := v.minElectableNodes == 0 || uint32(satisfied) >= v.minElectableNodes // #nosec G115

	if !hasSupermajority || !holdsFloor {
		log.Debug("version demotion skipped: safety guards not met",
			"requiredVersion", required,
			"targetEpoch", targetEpoch,
			"electable", electable,
			"satisfied", satisfied,
			"minElectableNodes", v.minElectableNodes,
			"hasSupermajority", hasSupermajority,
		)
		return enforcement
	}

	log.Debug("version demotion active",
		"requiredVersion", required,
		"targetEpoch", targetEpoch,
		"electable", electable,
		"satisfied", satisfied,
	)

	enforcement.demote = true

	return enforcement
}
