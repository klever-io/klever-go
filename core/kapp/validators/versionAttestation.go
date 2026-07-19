package validators

import (
	"strconv"
	"strings"

	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/data/state"
)

// versionWildcard is the versions.versionsByEpochs entry meaning "no specific version required"
const versionWildcard = "*"

// Outdated validators are only demoted when at least 2/3 of the active set
// (elected + eligible + waiting) already attested a satisfying version, so
// enforcement can never demote enough validators to endanger consensus liveness.
const (
	versionEnforcementNumerator   = 2
	versionEnforcementDenominator = 3
)

// requiredVersionForEpoch returns the version required for the given epoch based on the
// versions.versionsByEpochs configuration, and whether a specific version is required at all.
// A wildcard or missing entry means no requirement (mechanism stays dormant).
func (v *validatorsKApp) requiredVersionForEpoch(epoch uint32) (string, bool) {
	best := -1
	for i, entry := range v.versionsByEpochs {
		if entry.StartEpoch > epoch {
			continue
		}
		if best < 0 || entry.StartEpoch > v.versionsByEpochs[best].StartEpoch {
			best = i
		}
	}
	if best < 0 {
		return "", false
	}

	version := v.versionsByEpochs[best].Version
	if version == versionWildcard || len(version) == 0 {
		return "", false
	}

	return version, true
}

// parseSemver parses "v1.2.3", "1.2.3", "1.2" or "1" (optional pre-release/build suffix
// after '-' or '+' is ignored) into numeric components.
func parseSemver(version string) ([3]uint64, bool) {
	var out [3]uint64

	trimmed := strings.TrimSpace(version)
	trimmed = strings.TrimPrefix(trimmed, "v")
	trimmed = strings.TrimPrefix(trimmed, "V")
	if idx := strings.IndexAny(trimmed, "-+"); idx >= 0 {
		trimmed = trimmed[:idx]
	}

	parts := strings.Split(trimmed, ".")
	if len(parts) == 0 || len(parts) > 3 {
		return out, false
	}

	for i, part := range parts {
		number, err := strconv.ParseUint(part, 10, 32)
		if err != nil {
			return out, false
		}
		out[i] = number
	}

	return out, true
}

// versionSatisfies returns true when the attested version fulfills the required one:
// semantic compare (attested >= required) when both parse as versions, exact match otherwise.
func versionSatisfies(attested, required string) bool {
	if attested == required {
		return true
	}

	attestedParts, okAttested := parseSemver(attested)
	requiredParts, okRequired := parseSemver(required)
	if !okAttested || !okRequired {
		return false
	}

	for i := 0; i < len(attestedParts); i++ {
		if attestedParts[i] != requiredParts[i] {
			return attestedParts[i] > requiredParts[i]
		}
	}

	return true
}

// versionEnforcement is the per-epoch decision on demoting validators without a
// satisfying attested node version.
type versionEnforcement struct {
	enforce  bool
	required string
}

// computeVersionEnforcement decides, once per end-of-epoch processing, whether validators
// without a satisfying attested version should be demoted to the observer list when
// preparing the lists for targetEpoch. Enforcement requires the versionAttestation fork
// to be active, a non-wildcard required version for targetEpoch, and a 2/3 supermajority
// of the active set having already attested a satisfying version.
func (v *validatorsKApp) computeVersionEnforcement(
	app state.KAppAccountHandler,
	validatorInfos []*state.ValidatorInfo,
	targetEpoch uint32,
) versionEnforcement {
	if !v.forkController.VersionAttestation() {
		return versionEnforcement{}
	}

	required, ok := v.requiredVersionForEpoch(targetEpoch)
	if !ok {
		return versionEnforcement{}
	}

	active := 0
	satisfied := 0
	for _, info := range validatorInfos {
		switch info.List {
		case string(core.ElectedList), string(core.EligibleList), string(core.WaitingList):
		default:
			continue
		}
		active++

		val, err := v.getValidator(app, info.OwnerAddress)
		if err != nil {
			continue
		}
		if versionSatisfies(val.AttestedVersion, required) {
			satisfied++
		}
	}

	if active == 0 || satisfied*versionEnforcementDenominator < active*versionEnforcementNumerator {
		log.Debug("version enforcement skipped: attested supermajority not reached",
			"requiredVersion", required,
			"targetEpoch", targetEpoch,
			"active", active,
			"satisfied", satisfied,
		)
		return versionEnforcement{}
	}

	log.Debug("version enforcement active",
		"requiredVersion", required,
		"targetEpoch", targetEpoch,
		"active", active,
		"satisfied", satisfied,
	)

	return versionEnforcement{enforce: true, required: required}
}
