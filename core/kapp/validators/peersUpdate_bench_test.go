package validators

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math"
	"os"
	"testing"

	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/process/rating"
	"github.com/klever-io/klever-go/crypto/hashing/sha256"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/state/factory"
	"github.com/klever-io/klever-go/data/trie"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/kvm/mock/stub"
	"github.com/klever-io/klever-go/storage"
	"github.com/klever-io/klever-go/storage/leveldb"
	"github.com/klever-io/klever-go/storage/storageUnit"
	"github.com/klever-io/klever-go/tools/marshal"
)

// BenchmarkProcessEconomicsEndOfEpoch_Production simulates production-like conditions:
// - 21 validators (like mainnet)
// - 5000 buckets per validator (~100k delegators)
// - Pre-populated user trie with 500k dummy accounts to simulate deep trie
// - Cache reset between iterations to simulate cold storage
//
// This benchmark shows the full ProcessEconomicsEndOfEpoch performance including
// bucket iteration and reward distribution.
func BenchmarkProcessEconomicsEndOfEpoch_Production(b *testing.B) {
	const prodValidators = 21
	const prodBucketsPerValidator = 4500 // Max ~5500 due to MaxLeafSize=786KB limit in trackableDataTrie.go
	const dummyAccounts = 500000         // Pre-populate trie to make it deep

	b.Run("V1_Allowance", func(b *testing.B) {
		v, validatorInfos, accCacher, cleanup := setupProductionBenchmark(b, prodValidators, prodBucketsPerValidator, dummyAccounts, false)
		defer cleanup()

		currentEpoch := uint32(10)
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			accCacher.ResetAll(true)

			for _, vi := range validatorInfos {
				peerAcc, _ := v.accountsCacher.LoadPeer(vi.PublicKey)
				peerAcc.AddToAccumulatedFees(1000000)
			}

			err := v.ProcessEconomicsEndOfEpoch(currentEpoch, validatorInfos)
			if err != nil {
				b.Fatalf("ProcessEconomicsEndOfEpoch failed: %v", err)
			}
		}
	})

	b.Run("V2_PendingRewards", func(b *testing.B) {
		v, validatorInfos, accCacher, cleanup := setupProductionBenchmark(b, prodValidators, prodBucketsPerValidator, dummyAccounts, true)
		defer cleanup()

		currentEpoch := uint32(10)
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			accCacher.ResetAll(true)

			for _, vi := range validatorInfos {
				peerAcc, _ := v.accountsCacher.LoadPeer(vi.PublicKey)
				peerAcc.AddToAccumulatedFees(1000000)
			}

			err := v.ProcessEconomicsEndOfEpoch(currentEpoch, validatorInfos)
			if err != nil {
				b.Fatalf("ProcessEconomicsEndOfEpoch failed: %v", err)
			}
		}
	})
}

// BenchmarkUserAccountLoading_V1vsV2 isolates the critical difference between V1 and V2:
// - V1: Loads N user accounts from trie and updates allowance (expensive)
// - V2: Writes N entries to KApp storage (cheap)
//
// This benchmark clearly shows the bottleneck that V2 solves:
// V1 must traverse the user accounts trie for each delegator,
// while V2 only writes to a single KApp account's storage.
func BenchmarkUserAccountLoading_V1vsV2(b *testing.B) {
	const numUsers = 100000
	const dummyAccounts = 500000

	b.Run("V1_LoadUserAccounts", func(b *testing.B) {
		tempDir, _ := os.MkdirTemp("", "bench_v1_*")
		defer os.RemoveAll(tempDir)

		userStorer := createLevelDBStorer(b, tempDir+"/users")
		defer userStorer.DestroyUnit()

		hasher := &sha256.Sha256{}
		marshalizer := marshal.NewProtoMarshalizer()
		userTrieManager, _ := trie.NewTrieStorageManagerWithoutPruning(userStorer)
		userTrie, _ := trie.NewTrie(userTrieManager, marshalizer, hasher, 5)
		userAccountsDB, _ := state.NewAccountsDB(userTrie, hasher, marshalizer, factory.NewAccountCreator(), core.Normal)

		// Pre-populate with dummy accounts for deep trie
		b.Logf("Creating %d dummy accounts...", dummyAccounts)
		dummyAddrs := generateRandomAddresses(dummyAccounts)
		for i, addr := range dummyAddrs {
			acc, _ := userAccountsDB.LoadAccount(addr)
			userAccountsDB.SaveAccount(acc)
			if i%100000 == 0 {
				userAccountsDB.Commit()
			}
		}
		userAccountsDB.Commit()

		// Create target user addresses
		userAddrs := generateRandomAddresses(numUsers)
		for _, addr := range userAddrs {
			acc, _ := userAccountsDB.LoadAccount(addr)
			userAccountsDB.SaveAccount(acc)
		}
		userAccountsDB.Commit()
		b.Logf("Setup complete: %d users in trie with %d dummy accounts", numUsers, dummyAccounts)

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			// V1: Load each user account and update allowance
			for _, addr := range userAddrs {
				acc, _ := userAccountsDB.LoadAccount(addr)
				userAcc := acc.(state.UserAccountHandler)
				userAcc.AddToAllowance(1000)
				userAccountsDB.SaveAccount(acc)
			}
			userAccountsDB.Commit()
		}
	})

	b.Run("V2_KAppStorageWrite", func(b *testing.B) {
		tempDir, _ := os.MkdirTemp("", "bench_v2_*")
		defer os.RemoveAll(tempDir)

		kappStorer := createLevelDBStorer(b, tempDir+"/kapps")
		defer kappStorer.DestroyUnit()

		hasher := &sha256.Sha256{}
		marshalizer := marshal.NewProtoMarshalizer()
		kappTrieManager, _ := trie.NewTrieStorageManagerWithoutPruning(kappStorer)
		kappTrie, _ := trie.NewTrie(kappTrieManager, marshalizer, hasher, 5)
		kappAccountsDB, _ := state.NewAccountsDB(kappTrie, hasher, marshalizer, factory.NewKAppAccountCreator(), core.Normal)

		// Load KApp account
		kappAcc, _ := kappAccountsDB.LoadAccount(kapps.ValidatorsKAppAddress)

		// Create target user addresses
		userAddrs := generateRandomAddresses(numUsers)
		b.Logf("Setup complete: ready to write %d pending rewards entries", numUsers)

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			// V2: Write pending rewards to KApp storage
			kappHandler := kappAcc.(state.KAppAccountHandler)
			for _, addr := range userAddrs {
				key := append([]byte("pendingRewards_"), addr...)
				// Simulate adding to pending rewards (8 bytes for int64)
				kappHandler.SetStorage(key, make([]byte, 8))
			}
			kappAccountsDB.SaveAccount(kappAcc)
			kappAccountsDB.Commit()
		}
	})
}

// setupProductionBenchmark creates a production-like environment with a deep user accounts trie
func setupProductionBenchmark(b *testing.B, numValidators, bucketsPerValidator, dummyAccounts int, useV2 bool) (*validatorsKApp, []*state.ValidatorInfo, state.AccountsCacher, func()) {
	b.Helper()

	tempDir, err := os.MkdirTemp("", "bench_prod_*")
	if err != nil {
		b.Fatalf("Failed to create temp dir: %v", err)
	}

	userStorer := createLevelDBStorer(b, tempDir+"/users")
	kappStorer := createLevelDBStorer(b, tempDir+"/kapps")
	peerStorer := createLevelDBStorer(b, tempDir+"/peers")

	hasher := &sha256.Sha256{}
	marshalizer := marshal.NewProtoMarshalizer()

	userTrieManager, _ := trie.NewTrieStorageManagerWithoutPruning(userStorer)
	kappTrieManager, _ := trie.NewTrieStorageManagerWithoutPruning(kappStorer)
	peerTrieManager, _ := trie.NewTrieStorageManagerWithoutPruning(peerStorer)

	userTrie, _ := trie.NewTrie(userTrieManager, marshalizer, hasher, 5)
	kappTrie, _ := trie.NewTrie(kappTrieManager, marshalizer, hasher, 5)
	peerTrie, _ := trie.NewTrie(peerTrieManager, marshalizer, hasher, 5)

	userAccountsDB, _ := state.NewAccountsDB(userTrie, hasher, marshalizer, factory.NewAccountCreator(), core.Normal)
	kappAccountsDB, _ := state.NewAccountsDB(kappTrie, hasher, marshalizer, factory.NewKAppAccountCreator(), core.Normal)
	peerAccountsDB, _ := state.NewAccountsDB(peerTrie, hasher, marshalizer, factory.NewPeerAccountCreator(), core.Normal)

	accCacher, _ := state.NewAccountsCacher(state.ArgsAcccountCacher{
		Accounts: userAccountsDB,
		Kapps:    kappAccountsDB,
		Peers:    peerAccountsDB,
	})
	accCacher.ResetAll(true)

	// PRE-POPULATE user trie with dummy accounts to simulate production depth
	b.Logf("Pre-populating user trie with %d dummy accounts...", dummyAccounts)
	dummyAddresses := generateRandomAddresses(dummyAccounts)
	for i, addr := range dummyAddresses {
		userAcc, _ := accCacher.LoadUser(addr)
		userAcc.AddToBalance(1000000, nil, false) // Give them some balance
		if i%50000 == 0 {
			// Save periodically to avoid memory buildup
			accCacher.SaveAll()
			accCacher.ResetAll(true)
		}
	}
	accCacher.SaveAll()
	accCacher.ResetAll(true)
	b.Logf("User trie pre-populated")

	// Create validators KApp
	args := createMockArgs()
	v, _ := NewValidatorKApp(args)
	v.forkController.(*mock.ForkControllerStub).EpochRewardsV2Value = useV2
	v.forkController.(*mock.ForkControllerStub).FixDelegationSameEpochValue = true

	mockProposalController := &mock.ProposalControllerStub{
		GetParameterIntCalled: func(param kapps.EnumParameter) int64 {
			switch param {
			case kapps.EnumParameter_MinSelfDelegatedAmount:
				return 100000
			case kapps.EnumParameter_MinTotalDelegatedAmount:
				return 500000
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
	v.SetAccountsCacher(accCacher)

	rd := createDefaultRatingsData()
	bsr, _ := rating.NewBlockSigningRater(rd)
	v.SetRater(bsr)

	// Generate delegator addresses (separate from dummy accounts)
	userAddresses := generateRandomAddresses(numValidators * bucketsPerValidator)
	validatorInfos := make([]*state.ValidatorInfo, numValidators)

	kappAcc, _ := accCacher.LoadKApp(kapps.ValidatorsKAppAddress)

	for i := 0; i < numValidators; i++ {
		validatorAddr := []byte(fmt.Sprintf("validator%04d________________", i))[:32]
		blsPubKey := []byte(fmt.Sprintf("blspubkey%04d_______________", i))[:32]

		validatorInfos[i] = &state.ValidatorInfo{
			PublicKey:    blsPubKey,
			OwnerAddress: validatorAddr,
			List:         string(state.List_eligible),
			TempRating:   5000000,
		}

		peerAcc, _ := accCacher.LoadPeer(blsPubKey)
		peerAcc.SetListAndIndex(state.List_eligible, 0)
		peerAcc.SetTempRating(5000000)

		validatorData := &ValidatorData{
			BlsPubKey:      blsPubKey,
			RewardsAddress: validatorAddr,
			Commission:     1000,
			SelfStake:      1000000,
		}
		validatorKey := []byte(VALIDATOR_PREFIX + kapps.Sp + string(validatorAddr))
		data, _ := v.marshalizer.Marshal(validatorData)
		kappAcc.SetStorage(validatorKey, data)

		buckets := make(map[string]*PeerBucket, bucketsPerValidator)
		for j := 0; j < bucketsPerValidator; j++ {
			userIdx := i*bucketsPerValidator + j
			userAddr := userAddresses[userIdx]
			bucketID := fmt.Sprintf("bucket%04d%06d", i, j)

			buckets[bucketID] = &PeerBucket{
				DelegatedEpoch:   5,
				UndelegatedEpoch: math.MaxUint32,
				Value:            100000,
				Address:          userAddr,
			}

			// Create user account in trie (will need to be loaded by V1)
			_, _ = accCacher.LoadUser(userAddr)
		}

		peerData := &PeerData{Buckets: buckets}
		peerDataKey := []byte(VALIDATOR_BUCKETS + kapps.Sp + string(validatorAddr))
		peerDataBytes, err := v.marshalizer.Marshal(peerData)
		if err != nil {
			b.Fatalf("Failed to marshal peerData for validator %d: %v (buckets: %d)", i, err, len(buckets))
		}
		if len(peerDataBytes) == 0 {
			b.Fatalf("peerDataBytes is empty for validator %d (buckets: %d)", i, len(buckets))
		}
		if err := kappAcc.SetStorage(peerDataKey, peerDataBytes); err != nil {
			b.Fatalf("Failed to SetStorage for validator %d: %v (size: %d bytes, MaxLeafSize: 786KB)", i, err, len(peerDataBytes))
		}
		if i == 0 {
			b.Logf("Validator %d: %d buckets, serialized size: %d bytes", i, len(buckets), len(peerDataBytes))
		}
	}

	b.Logf("Total buckets created: %d across %d validators", numValidators*bucketsPerValidator, numValidators)
	accCacher.SaveAll()

	// Verify data can be retrieved after reset
	accCacher.ResetAll(true)
	verifyKapp, _ := accCacher.LoadKApp(kapps.ValidatorsKAppAddress)
	testKey := []byte(VALIDATOR_BUCKETS + kapps.Sp + string(validatorInfos[0].OwnerAddress))
	testData := verifyKapp.GetStorage(testKey)
	if len(testData) == 0 {
		b.Fatalf("VERIFICATION FAILED: Cannot retrieve bucket data after reset (key length: %d)", len(testKey))
	}
	testPeerData := &PeerData{}
	if err := v.marshalizer.Unmarshal(testPeerData, testData); err != nil {
		b.Fatalf("VERIFICATION FAILED: Cannot unmarshal bucket data: %v", err)
	}
	b.Logf("VERIFICATION OK: Retrieved %d buckets for validator 0 after reset", len(testPeerData.Buckets))
	accCacher.ResetAll(true)

	cleanup := func() {
		userStorer.DestroyUnit()
		kappStorer.DestroyUnit()
		peerStorer.DestroyUnit()
		os.RemoveAll(tempDir)
	}

	return v, validatorInfos, accCacher, cleanup
}

func createLevelDBStorer(b *testing.B, path string) storage.Storer {
	b.Helper()

	cache, err := storageUnit.NewCache(storageUnit.CacheConfig{
		Type:        storageUnit.LRUCache,
		Capacity:    10000,
		Shards:      1,
		SizeInBytes: 0,
	})
	if err != nil {
		b.Fatalf("Failed to create cache: %v", err)
	}

	lvdb, err := leveldb.NewDB(path, 10, 100, 10)
	if err != nil {
		b.Fatalf("Failed to create LevelDB: %v", err)
	}

	unit, err := storageUnit.NewStorageUnit(cache, lvdb)
	if err != nil {
		b.Fatalf("Failed to create storage unit: %v", err)
	}

	return unit
}

func generateRandomAddresses(count int) [][]byte {
	addresses := make([][]byte, count)
	for i := 0; i < count; i++ {
		addr := make([]byte, 32)
		rand.Read(addr)
		// Use hex encoding to ensure valid address format
		addresses[i] = []byte(hex.EncodeToString(addr)[:32])
	}
	return addresses
}
