package validators

import (
	"fmt"
	"testing"

	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/kvm/mock/stub"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseSemver(t *testing.T) {
	t.Parallel()

	tests := []struct {
		version  string
		expected [3]uint64
		ok       bool
	}{
		{"v1.2.3", [3]uint64{1, 2, 3}, true},
		{"1.2.3", [3]uint64{1, 2, 3}, true},
		{"V2.0.0", [3]uint64{2, 0, 0}, true},
		{"1.2", [3]uint64{1, 2, 0}, true},
		{"1", [3]uint64{1, 0, 0}, true},
		{"v1.2.3-rc1", [3]uint64{1, 2, 3}, true},
		{"v1.2.3+meta", [3]uint64{1, 2, 3}, true},
		{" v1.2.3 ", [3]uint64{1, 2, 3}, true},
		{"", [3]uint64{}, false},
		{"abc", [3]uint64{}, false},
		{"1.2.3.4", [3]uint64{}, false},
		{"1.x.3", [3]uint64{}, false},
		{"*", [3]uint64{}, false},
	}

	for _, tc := range tests {
		parsed, ok := parseSemver(tc.version)
		assert.Equal(t, tc.ok, ok, "version %q", tc.version)
		if tc.ok {
			assert.Equal(t, tc.expected, parsed, "version %q", tc.version)
		}
	}
}

func TestVersionSatisfies(t *testing.T) {
	t.Parallel()

	tests := []struct {
		attested string
		required string
		expected bool
	}{
		{"v1.9.0", "v1.9.0", true},
		{"v1.9.1", "v1.9.0", true},
		{"v1.10.0", "v1.9.0", true},
		{"v2.0.0", "v1.9.0", true},
		{"v1.8.9", "v1.9.0", false},
		{"v1.9.0", "v2.0.0", false},
		{"", "v1.9.0", false},
		{"1.9.0", "v1.9.0", true},   // prefix-insensitive semantic match
		{"custom", "custom", true},  // exact match works for non-semver tags
		{"custom", "v1.9.0", false}, // non-parsable mismatch fails closed
		{"v1.9.0", "custom", false},
	}

	for _, tc := range tests {
		assert.Equal(t, tc.expected, versionSatisfies(tc.attested, tc.required),
			"attested %q required %q", tc.attested, tc.required)
	}
}

func TestRequiredVersionForEpoch(t *testing.T) {
	t.Parallel()

	t.Run("no config means no requirement", func(t *testing.T) {
		v := setupValidatorsKApp(t)
		v.versionsByEpochs = nil

		_, ok := v.requiredVersionForEpoch(10)
		assert.False(t, ok)
	})

	t.Run("wildcard means no requirement", func(t *testing.T) {
		v := setupValidatorsKApp(t)
		v.versionsByEpochs = []config.VersionByEpochs{
			{StartEpoch: 0, Version: "*"},
		}

		_, ok := v.requiredVersionForEpoch(10)
		assert.False(t, ok)
	})

	t.Run("epoch ranges resolve to the latest started entry", func(t *testing.T) {
		v := setupValidatorsKApp(t)
		// deliberately unsorted to prove order independence
		v.versionsByEpochs = []config.VersionByEpochs{
			{StartEpoch: 20, Version: "v2.0.0"},
			{StartEpoch: 0, Version: "*"},
			{StartEpoch: 10, Version: "v1.9.0"},
		}

		_, ok := v.requiredVersionForEpoch(5)
		assert.False(t, ok, "wildcard entry active before epoch 10")

		required, ok := v.requiredVersionForEpoch(10)
		assert.True(t, ok)
		assert.Equal(t, "v1.9.0", required)

		required, ok = v.requiredVersionForEpoch(19)
		assert.True(t, ok)
		assert.Equal(t, "v1.9.0", required)

		required, ok = v.requiredVersionForEpoch(25)
		assert.True(t, ok)
		assert.Equal(t, "v2.0.0", required)
	})
}

func TestUpdatePeerListStatus_VersionEnforcement(t *testing.T) {
	t.Parallel()

	const minSelf = int64(100)
	const minTotal = int64(500)
	const totalDelegated = int64(1000)

	enforced := versionEnforcement{enforce: true, required: "v1.9.0"}

	newPeer := func(t *testing.T, list state.List) state.PeerAccountHandler {
		peerAcc, err := state.NewPeerAccount([]byte("peer"))
		require.NoError(t, err)
		peerAcc.SetList(list)
		return peerAcc
	}

	t.Run("outdated validator is demoted to observer", func(t *testing.T) {
		v := setupValidatorsKApp(t)
		val := &ValidatorData{SelfStake: minSelf, AttestedVersion: "v1.8.0"}
		peerAcc := newPeer(t, state.List_eligible)

		v.updatePeerListStatus(val, peerAcc, minSelf, minTotal, totalDelegated, enforced)
		assert.Equal(t, state.List_observer, peerAcc.GetList())
	})

	t.Run("validator without attestation is demoted to observer", func(t *testing.T) {
		v := setupValidatorsKApp(t)
		val := &ValidatorData{SelfStake: minSelf}
		peerAcc := newPeer(t, state.List_eligible)

		v.updatePeerListStatus(val, peerAcc, minSelf, minTotal, totalDelegated, enforced)
		assert.Equal(t, state.List_observer, peerAcc.GetList())
	})

	t.Run("elected outdated validator is demoted to observer", func(t *testing.T) {
		v := setupValidatorsKApp(t)
		val := &ValidatorData{SelfStake: minSelf, AttestedVersion: "v1.8.0"}
		peerAcc := newPeer(t, state.List_elected)

		v.updatePeerListStatus(val, peerAcc, minSelf, minTotal, totalDelegated, enforced)
		assert.Equal(t, state.List_observer, peerAcc.GetList())
	})

	t.Run("attested validator stays eligible", func(t *testing.T) {
		v := setupValidatorsKApp(t)
		val := &ValidatorData{SelfStake: minSelf, AttestedVersion: "v1.9.0"}
		peerAcc := newPeer(t, state.List_eligible)

		v.updatePeerListStatus(val, peerAcc, minSelf, minTotal, totalDelegated, enforced)
		assert.Equal(t, state.List_eligible, peerAcc.GetList())
	})

	t.Run("newer attested version stays eligible", func(t *testing.T) {
		v := setupValidatorsKApp(t)
		val := &ValidatorData{SelfStake: minSelf, AttestedVersion: "v2.0.0"}
		peerAcc := newPeer(t, state.List_eligible)

		v.updatePeerListStatus(val, peerAcc, minSelf, minTotal, totalDelegated, enforced)
		assert.Equal(t, state.List_eligible, peerAcc.GetList())
	})

	t.Run("demoted validator is restored after attesting", func(t *testing.T) {
		v := setupValidatorsKApp(t)
		val := &ValidatorData{SelfStake: minSelf, AttestedVersion: "v1.9.0"}
		peerAcc := newPeer(t, state.List_observer)

		v.updatePeerListStatus(val, peerAcc, minSelf, minTotal, totalDelegated, enforced)
		assert.Equal(t, state.List_eligible, peerAcc.GetList())
	})

	t.Run("no enforcement keeps unattested validator eligible", func(t *testing.T) {
		v := setupValidatorsKApp(t)
		val := &ValidatorData{SelfStake: minSelf}
		peerAcc := newPeer(t, state.List_eligible)

		v.updatePeerListStatus(val, peerAcc, minSelf, minTotal, totalDelegated, versionEnforcement{})
		assert.Equal(t, state.List_eligible, peerAcc.GetList())
	})

	t.Run("stake checks take precedence over version", func(t *testing.T) {
		v := setupValidatorsKApp(t)
		val := &ValidatorData{SelfStake: minSelf - 1, AttestedVersion: "v1.8.0"}
		peerAcc := newPeer(t, state.List_eligible)

		v.updatePeerListStatus(val, peerAcc, minSelf, minTotal, totalDelegated, enforced)
		assert.Equal(t, state.List_inactive, peerAcc.GetList())
	})

	t.Run("jailed validator list is not touched", func(t *testing.T) {
		v := setupValidatorsKApp(t)
		val := &ValidatorData{SelfStake: minSelf, Jailed: true}
		peerAcc := newPeer(t, state.List_jailed)

		v.updatePeerListStatus(val, peerAcc, minSelf, minTotal, totalDelegated, enforced)
		assert.Equal(t, state.List_jailed, peerAcc.GetList())
	})
}

// attestationTestSetup builds a validators KApp with in-memory kapp storage holding
// one ValidatorData entry per owner, plus matching validator infos.
func attestationTestSetup(
	t *testing.T,
	attestedByOwner map[string]string,
	listByOwner map[string]string,
) (*validatorsKApp, []*state.ValidatorInfo, state.KAppAccountHandler) {
	v := setupValidatorsKApp(t)
	addFunctionalCacher(t, v)

	rawData := make(map[string][]byte)
	loadKApp := func(address []byte) (state.KAppAccountHandler, error) {
		return &mock.KAppAccountHandlerStub{
			GetStorageCalled: func(key []byte) []byte {
				return rawData[string(key)]
			},
			SetStorageCalled: func(key []byte, value []byte) error {
				rawData[string(key)] = value
				return nil
			},
		}, nil
	}
	v.accountsCacher.(*mock.AccountsCacherStub).LoadKAppCalled = loadKApp
	v.accountsCacher.(*mock.AccountsCacherStub).LoadKAppUncachedCalled = loadKApp

	app, err := v.getKApp()
	require.NoError(t, err)

	validatorInfos := make([]*state.ValidatorInfo, 0, len(attestedByOwner))
	for owner, attested := range attestedByOwner {
		val := &ValidatorData{
			BlsPubKey:       []byte("bls_" + owner),
			SelfStake:       1000,
			AttestedVersion: attested,
		}
		require.NoError(t, v.setValidator(app, []byte(owner), val))

		list := state.List_eligible.String()
		if fromMap, ok := listByOwner[owner]; ok {
			list = fromMap
		}

		validatorInfos = append(validatorInfos, &state.ValidatorInfo{
			OwnerAddress: []byte(owner),
			PublicKey:    []byte("bls_" + owner),
			List:         list,
		})
	}

	return v, validatorInfos, app
}

func TestComputeVersionEnforcement(t *testing.T) {
	t.Parallel()

	requiredVersions := []config.VersionByEpochs{
		{StartEpoch: 0, Version: "*"},
		{StartEpoch: 10, Version: "v1.9.0"},
	}

	t.Run("fork flag disabled means no enforcement", func(t *testing.T) {
		v, infos, app := attestationTestSetup(t, map[string]string{
			"owner1": "v1.9.0",
			"owner2": "v1.9.0",
			"owner3": "v1.9.0",
		}, nil)
		v.versionsByEpochs = requiredVersions
		v.forkController.(*mock.ForkControllerStub).VersionAttestationValue = false

		enforcement := v.computeVersionEnforcement(app, infos, 10)
		assert.False(t, enforcement.enforce)
	})

	t.Run("wildcard epoch means no enforcement", func(t *testing.T) {
		v, infos, app := attestationTestSetup(t, map[string]string{
			"owner1": "v1.9.0",
			"owner2": "v1.9.0",
			"owner3": "v1.9.0",
		}, nil)
		v.versionsByEpochs = requiredVersions

		enforcement := v.computeVersionEnforcement(app, infos, 5)
		assert.False(t, enforcement.enforce)
	})

	t.Run("exact two thirds attested enables enforcement", func(t *testing.T) {
		v, infos, app := attestationTestSetup(t, map[string]string{
			"owner1": "v1.9.0",
			"owner2": "v2.0.0",
			"owner3": "",
		}, nil)
		v.versionsByEpochs = requiredVersions

		enforcement := v.computeVersionEnforcement(app, infos, 10)
		assert.True(t, enforcement.enforce)
		assert.Equal(t, "v1.9.0", enforcement.required)
	})

	t.Run("below two thirds attested disables enforcement", func(t *testing.T) {
		v, infos, app := attestationTestSetup(t, map[string]string{
			"owner1": "v1.9.0",
			"owner2": "v1.8.0",
			"owner3": "",
		}, nil)
		v.versionsByEpochs = requiredVersions

		enforcement := v.computeVersionEnforcement(app, infos, 10)
		assert.False(t, enforcement.enforce)
	})

	t.Run("only active lists count towards the threshold", func(t *testing.T) {
		v, infos, app := attestationTestSetup(t, map[string]string{
			"owner1": "v1.9.0",
			"owner2": "v1.9.0",
			"owner3": "", // jailed: excluded from the active set
			"owner4": "", // inactive: excluded from the active set
		}, map[string]string{
			"owner3": state.List_jailed.String(),
			"owner4": state.List_inactive.String(),
		})
		v.versionsByEpochs = requiredVersions

		enforcement := v.computeVersionEnforcement(app, infos, 10)
		assert.True(t, enforcement.enforce)
	})

	t.Run("no active validators disables enforcement", func(t *testing.T) {
		v, infos, app := attestationTestSetup(t, map[string]string{
			"owner1": "v1.9.0",
		}, map[string]string{
			"owner1": state.List_inactive.String(),
		})
		v.versionsByEpochs = requiredVersions

		enforcement := v.computeVersionEnforcement(app, infos, 10)
		assert.False(t, enforcement.enforce)
	})
}

func TestUpdateValidator_VersionAttestation(t *testing.T) {
	t.Parallel()

	ownerAddress := makeAddress("owner")

	setup := func(t *testing.T, forkActive bool) *validatorsKApp {
		v := setupValidatorsKApp(t)
		addFunctionalCacher(t, v)
		addStorageCacher(v)
		registerValidator(t, v, ownerAddress, []byte("blspubkey"))
		v.forkController.(*mock.ForkControllerStub).VersionAttestationValue = forkActive
		return v
	}

	t.Run("attestation is stored when fork is active", func(t *testing.T) {
		v := setup(t, true)

		tc := &transaction.ValidatorConfigContract{
			Config: &transaction.ValidatorConfig{
				BLSPublicKey: []byte("blspubkey"),
				NodeVersion:  "v1.9.0",
			},
		}

		resultCode, err := v.UpdateValidator(ownerAddress, tc)
		require.NoError(t, err)
		require.Equal(t, transaction.Transaction_Ok, resultCode)

		app, _ := v.getKApp()
		val, err := v.getValidator(app, ownerAddress)
		require.NoError(t, err)
		assert.Equal(t, "v1.9.0", val.AttestedVersion)
	})

	t.Run("attestation is ignored before the fork", func(t *testing.T) {
		v := setup(t, false)

		tc := &transaction.ValidatorConfigContract{
			Config: &transaction.ValidatorConfig{
				BLSPublicKey: []byte("blspubkey"),
				NodeVersion:  "v1.9.0",
			},
		}

		resultCode, err := v.UpdateValidator(ownerAddress, tc)
		require.NoError(t, err)
		require.Equal(t, transaction.Transaction_Ok, resultCode)

		app, _ := v.getKApp()
		val, err := v.getValidator(app, ownerAddress)
		require.NoError(t, err)
		assert.Equal(t, "", val.AttestedVersion)
	})

	t.Run("oversized version is rejected", func(t *testing.T) {
		v := setup(t, true)

		tc := &transaction.ValidatorConfigContract{
			Config: &transaction.ValidatorConfig{
				BLSPublicKey: []byte("blspubkey"),
				NodeVersion:  "v1.9.0-longbuildsuffix",
			},
		}

		resultCode, _ := v.UpdateValidator(ownerAddress, tc)
		assert.Equal(t, transaction.Transaction_ParameterInvalid, resultCode)
	})

	t.Run("invalid utf8 version is rejected", func(t *testing.T) {
		v := setup(t, true)

		tc := &transaction.ValidatorConfigContract{
			Config: &transaction.ValidatorConfig{
				BLSPublicKey: []byte("blspubkey"),
				NodeVersion:  string([]byte{0xff, 0xfe}),
			},
		}

		resultCode, _ := v.UpdateValidator(ownerAddress, tc)
		assert.Equal(t, transaction.Transaction_ParameterInvalid, resultCode)
	})
}

func TestProcessEconomicsEndOfEpoch_VersionDemotionAndRestore(t *testing.T) {
	t.Parallel()

	const currentEpoch = uint32(10)

	setupEconomics := func(t *testing.T) *validatorsKApp {
		v := setupValidatorsKApp(t)
		addFunctionalCacher(t, v)

		mockProposalController := &mock.ProposalControllerStub{
			GetParameterIntCalled: func(param kapps.EnumParameter) int64 {
				switch param {
				case kapps.EnumParameter_MinSelfDelegatedAmount:
					return 100000
				case kapps.EnumParameter_MinTotalDelegatedAmount:
					return 100000
				default:
					return 0
				}
			},
		}
		v.KAppController = &stub.KAppControllerStub{
			GetProposalControllerCalled: func() kapps.ActiveProposalController {
				return mockProposalController
			},
		}

		v.versionsByEpochs = []config.VersionByEpochs{
			{StartEpoch: 0, Version: "*"},
			{StartEpoch: 10, Version: "v1.9.0"},
		}

		return v
	}

	buildStorage := func(v *validatorsKApp, attested map[string]string) []*state.ValidatorInfo {
		rawData := make(map[string][]byte)
		loadKApp := func(address []byte) (state.KAppAccountHandler, error) {
			return &mock.KAppAccountHandlerStub{
				GetStorageCalled: func(key []byte) []byte {
					return rawData[string(key)]
				},
				SetStorageCalled: func(key []byte, value []byte) error {
					rawData[string(key)] = value
					return nil
				},
			}, nil
		}
		v.accountsCacher.(*mock.AccountsCacherStub).LoadKAppCalled = loadKApp
		v.accountsCacher.(*mock.AccountsCacherStub).LoadKAppUncachedCalled = loadKApp

		app, _ := v.getKApp()

		infos := make([]*state.ValidatorInfo, 0, len(attested))
		for owner, version := range attested {
			blsKey := []byte("bls_" + owner)
			val := &ValidatorData{
				BlsPubKey:       blsKey,
				RewardsAddress:  []byte(owner),
				SelfStake:       200000,
				AttestedVersion: version,
			}
			if err := v.setValidator(app, []byte(owner), val); err != nil {
				t.Fatal(err)
			}

			pd := &PeerData{
				Buckets: map[string]*PeerBucket{
					fmt.Sprintf("bucket_%s", owner): {
						DelegatedEpoch:   5,
						UndelegatedEpoch: 4294967295,
						Value:            200000,
						Address:          []byte(owner),
					},
				},
			}
			if err := v.setValidatorBuckets(app, []byte(owner), pd); err != nil {
				t.Fatal(err)
			}

			peerAcc, _ := v.accountsCacher.LoadPeer(blsKey)
			peerAcc.SetList(state.List_eligible)

			infos = append(infos, &state.ValidatorInfo{
				OwnerAddress: []byte(owner),
				PublicKey:    blsKey,
				List:         state.List_eligible.String(),
			})
		}

		return infos
	}

	t.Run("outdated validator is demoted and restored after attesting", func(t *testing.T) {
		v := setupEconomics(t)
		infos := buildStorage(v, map[string]string{
			"owner1": "v1.9.0",
			"owner2": "v1.9.0",
			"owner3": "v1.8.0",
		})

		require.NoError(t, v.ProcessEconomicsEndOfEpoch(currentEpoch, infos))

		outdatedPeer, err := v.accountsCacher.GetExistingPeer([]byte("bls_owner3"))
		require.NoError(t, err)
		assert.Equal(t, state.List_observer, outdatedPeer.GetList(), "outdated validator should be observer")

		updatedPeer, err := v.accountsCacher.GetExistingPeer([]byte("bls_owner1"))
		require.NoError(t, err)
		assert.Equal(t, state.List_eligible, updatedPeer.GetList(), "attested validator should stay eligible")

		// validator attests the required version (as the ValidatorConfig tx would do)
		app, _ := v.getKApp()
		val, err := v.getValidator(app, []byte("owner3"))
		require.NoError(t, err)
		val.AttestedVersion = "v1.9.0"
		val.AttestedEpoch = currentEpoch
		require.NoError(t, v.setValidator(app, []byte("owner3"), val))
		require.NoError(t, v.saveKApp(app))

		// observer stays in validator infos, so the next end-of-epoch restores it
		infos[2].List = state.List_observer.String()
		require.NoError(t, v.ProcessEconomicsEndOfEpoch(currentEpoch+1, infos))

		restoredPeer, err := v.accountsCacher.GetExistingPeer([]byte("bls_owner3"))
		require.NoError(t, err)
		assert.Equal(t, state.List_eligible, restoredPeer.GetList(), "attested validator should be restored")
	})

	t.Run("no demotion without attested supermajority", func(t *testing.T) {
		v := setupEconomics(t)
		infos := buildStorage(v, map[string]string{
			"owner1": "v1.9.0",
			"owner2": "v1.8.0",
			"owner3": "",
		})

		require.NoError(t, v.ProcessEconomicsEndOfEpoch(currentEpoch, infos))

		for _, blsKey := range []string{"bls_owner1", "bls_owner2", "bls_owner3"} {
			peerAcc, err := v.accountsCacher.GetExistingPeer([]byte(blsKey))
			require.NoError(t, err)
			assert.Equal(t, state.List_eligible, peerAcc.GetList(),
				"validator %s should stay eligible without supermajority", blsKey)
		}
	})

	t.Run("no demotion before the required epoch", func(t *testing.T) {
		v := setupEconomics(t)
		infos := buildStorage(v, map[string]string{
			"owner1": "v1.9.0",
			"owner2": "v1.9.0",
			"owner3": "",
		})

		require.NoError(t, v.ProcessEconomicsEndOfEpoch(9, infos))

		outdatedPeer, err := v.accountsCacher.GetExistingPeer([]byte("bls_owner3"))
		require.NoError(t, err)
		assert.Equal(t, state.List_eligible, outdatedPeer.GetList())
	})
}
