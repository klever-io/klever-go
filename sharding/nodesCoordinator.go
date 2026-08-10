package sharding

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"

	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/storage"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/tools/marshal"
)

const (
	keyFormat               = "%s_%v_%v"
	DefaultSelectionChances = uint32(1)
)

type validatorList []Validator

// Len will return the length of the validatorList
func (v validatorList) Len() int { return len(v) }

// Swap will interchange the objects on input indexes
func (v validatorList) Swap(i, j int) { v[i], v[j] = v[j], v[i] }

// Less will return true if object on index i should appear before object in index j
// Sorting of validators should be by index and public key
func (v validatorList) Less(i, j int) bool {
	if v[i].Index() == v[j].Index() {
		return bytes.Compare(v[i].PubKey(), v[j].PubKey()) < 0
	}
	return v[i].Index() < v[j].Index()
}

// TODO: move this to config parameters
const nodeCoordinatorStoredEpochs = 10

type epochNodesConfig struct {
	electedList  []Validator
	eligibleList []Validator
	waitingList  []Validator
	selector     RandomSelector
	leavingList  []Validator
	mutNodesMaps sync.RWMutex
}

type indexHashedNodesCoordinator struct {
	selfPubKey                    []byte
	mutNodesConfig                sync.RWMutex
	epochStartRegistrationHandler EpochStartEventNotifier
	nodesConfig                   map[uint32]*epochNodesConfig
	validatorsInfo                map[uint32][]*block.EValidatorInfo
	publicKeyToValidatorMap       map[string]Validator
	savedStateKey                 []byte
	marshalizer                   marshal.Marshalizer
	mutSavedStateKey              sync.RWMutex
	bootStorer                    storage.Storer
	hasher                        hashing.Hasher
	loadingFromDisk               atomic.Value
	shuffler                      NodesShuffler
	consensusGroupCacher          Cacher
	consensusGroupSize            int
	currentEpoch                  atomic.Uint32
	startEpoch                    uint32

	stateReady atomic.Bool
}

type indexHashedNodesCoordinatorWithRater struct {
	*indexHashedNodesCoordinator
}

// NewNodesCoordinator -
func NewNodesCoordinator(arguments ArgNodesCoordinator) (*indexHashedNodesCoordinatorWithRater, error) {
	err := checkArguments(arguments)
	if err != nil {
		return nil, err
	}

	nodesConfig := make(map[uint32]*epochNodesConfig, nodeCoordinatorStoredEpochs)

	nodesConfig[arguments.Epoch] = &epochNodesConfig{
		electedList:  arguments.ElectedNodes,
		eligibleList: arguments.EligibleNodes,
		selector:     nil, // TODO:
		leavingList:  make([]Validator, 0),
	}

	savedKey := arguments.Hasher.Compute(string(arguments.SelfPublicKey))

	ihgs := &indexHashedNodesCoordinator{
		marshalizer:                   arguments.Marshalizer,
		hasher:                        arguments.Hasher,
		shuffler:                      arguments.Shuffler,
		epochStartRegistrationHandler: arguments.EpochStartNotifier,
		selfPubKey:                    arguments.SelfPublicKey,
		savedStateKey:                 savedKey,
		bootStorer:                    arguments.BootStorer,
		nodesConfig:                   nodesConfig,
		validatorsInfo:                make(map[uint32][]*block.EValidatorInfo),
		consensusGroupCacher:          arguments.ConsensusGroupCache,
		consensusGroupSize:            arguments.ConsensusGroupSize,
		publicKeyToValidatorMap:       make(map[string]Validator),
		startEpoch:                    arguments.StartEpoch,
	}
	ihgs.currentEpoch.Store(arguments.StartEpoch)
	// no need to wait for load state, as we have the initial configuration
	ihgs.stateReady.Store(arguments.StartEpoch == 0)

	ihgs.loadingFromDisk.Store(false)

	ihgs.fillPublicKeyToValidatorMap()

	ihncr := &indexHashedNodesCoordinatorWithRater{
		indexHashedNodesCoordinator: ihgs,
	}

	if arguments.StartEpoch == 0 {
		err = ihgs.saveState(ihgs.savedStateKey)
		if err != nil {
			log.Error("saving initial nodes coordinator config failed",
				"error", err.Error())
		}
	}

	log.Info("new nodes config is set for epoch", "epoch", arguments.Epoch)
	currentNodesConfig := ihgs.nodesConfig[arguments.Epoch]
	if currentNodesConfig == nil {
		return nil, fmt.Errorf("%w epoch=%v", ErrEpochNodesConfigDoesNotExist, arguments.Epoch)
	}

	if len(currentNodesConfig.electedList) < ihgs.consensusGroupSize {
		return nil, ErrSmallElectedListSize
	}

	currentNodesConfig.selector, err = ihgs.createSelector(currentNodesConfig)
	if err != nil {
		return nil, err
	}

	if len(arguments.CurrValidatorsInfo) > 0 {
		err := ihgs.SetEpochValidatorsInfo(arguments.Epoch, arguments.CurrValidatorsInfo)
		if err != nil {
			log.Error("saving initial nodes coordinator config failed",
				"error", err.Error())
		}
	}

	if len(arguments.PrevValidatorsInfo) > 0 {
		err := ihgs.SetEpochValidatorsInfo(arguments.Epoch-1, arguments.PrevValidatorsInfo)
		if err != nil {
			log.Error("saving initial nodes coordinator config failed",
				"error", err.Error())
		}
	}

	displayNodesConfiguration(
		currentNodesConfig.electedList,
		currentNodesConfig.eligibleList,
		currentNodesConfig.waitingList,
		currentNodesConfig.leavingList,
	)

	ihncr.epochStartRegistrationHandler.RegisterHandler(ihncr)

	return ihncr, nil
}

func checkArguments(arguments ArgNodesCoordinator) error {
	if arguments.ConsensusGroupSize < 1 {
		return ErrInvalidConsensusGroupSize
	}
	if check.IfNil(arguments.Marshalizer) {
		return ErrNilMarshalizer
	}
	if check.IfNil(arguments.Hasher) {
		return ErrNilHasher
	}
	if len(arguments.SelfPublicKey) == 0 {
		return ErrNilPubKey
	}
	if check.IfNil(arguments.Shuffler) {
		return ErrNilShuffler
	}
	if check.IfNil(arguments.BootStorer) {
		return ErrNilBootStorer
	}
	if check.IfNilReflect(arguments.ConsensusGroupCache) {
		return ErrNilCacher
	}

	return nil
}

// EpochStartAction is called upon a start of epoch event.
// NodeCoordinator has to get the nodes assignment to shards using the shuffler.
func (ihgs *indexHashedNodesCoordinator) EpochStartAction(hdr data.HeaderHandler) {
	newEpoch := hdr.GetEpoch()
	epochToRemove := int32(newEpoch) - nodeCoordinatorStoredEpochs // #nosec G115
	needToRemove := epochToRemove >= 0
	ihgs.currentEpoch.Store(newEpoch)

	ihgs.mutSavedStateKey.RLock()
	savedStateKey := bytes.Clone(ihgs.savedStateKey)
	ihgs.mutSavedStateKey.RUnlock()
	err := ihgs.saveState(savedStateKey)
	if err != nil {
		log.Error("saving nodes coordinator config failed", "error", err.Error())
	}

	ihgs.mutNodesConfig.Lock()
	if needToRemove {
		for epoch := range ihgs.nodesConfig {
			// #nosec G115
			if epoch <= uint32(epochToRemove) {
				delete(ihgs.nodesConfig, epoch)
			}
		}
	}
	ihgs.mutNodesConfig.Unlock()
}

// NotifyOrder returns the notification order for a start of epoch event
func (ihgs *indexHashedNodesCoordinator) NotifyOrder() uint32 {
	return core.NodesCoordinatorOrder
}

func (ihgs *indexHashedNodesCoordinator) saveState(key []byte) error {
	registry := ihgs.NodesCoordinatorToRegistry()
	data, err := json.Marshal(registry)
	if err != nil {
		return err
	}

	ncInternalkey := append([]byte(core.NodesCoordinatorRegistryKeyPrefix), key...)

	log.Debug("saving nodes coordinator config", "key", ncInternalkey)

	return ihgs.bootStorer.Put(ncInternalkey, data)
}

func (ihgs *indexHashedNodesCoordinator) LoadState(key []byte) error {
	ncInternalkey := append([]byte(core.NodesCoordinatorRegistryKeyPrefix), key...)

	log.Debug("getting nodes coordinator config", "key", ncInternalkey)

	ihgs.loadingFromDisk.Store(true)
	defer ihgs.loadingFromDisk.Store(false)

	data, err := ihgs.bootStorer.Get(ncInternalkey)
	if err != nil {
		return err
	}

	config := &NodesCoordinatorRegistry{}
	err = json.Unmarshal(data, config)
	if err != nil {
		return err
	}

	ihgs.mutSavedStateKey.Lock()
	ihgs.savedStateKey = key
	ihgs.mutSavedStateKey.Unlock()

	ihgs.currentEpoch.Store(config.CurrentEpoch)
	log.Debug("loaded nodes config", "current epoch", config.CurrentEpoch)

	nodesConfig, err := ihgs.registryToNodesCoordinator(config)
	if err != nil {
		return err
	}

	displayNodesConfigInfo(nodesConfig)

	ihgs.mutNodesConfig.Lock()
	ihgs.nodesConfig = nodesConfig
	publicKeyToValidatorMap := ihgs.computePublicKeyToValidatorMap(ihgs.nodesConfig)
	if len(publicKeyToValidatorMap) > 0 {
		ihgs.publicKeyToValidatorMap = publicKeyToValidatorMap
	} else {
		log.Warn("LoadState: restored registry produced empty validator map, keeping existing lookup")
	}
	ihgs.mutNodesConfig.Unlock()

	ihgs.stateReady.Store(true)

	return nil
}

func displayNodesConfigInfo(config map[uint32]*epochNodesConfig) {
	for epoch, cfg := range config {
		log.Debug("restored config for",
			"epoch", epoch,
			"elected length", len(cfg.electedList),
			"eligible length", len(cfg.eligibleList),
		)
	}
}

func (ihgs *indexHashedNodesCoordinator) registryToNodesCoordinator(
	config *NodesCoordinatorRegistry,
) (map[uint32]*epochNodesConfig, error) {
	var err error
	var epoch int64
	result := make(map[uint32]*epochNodesConfig)

	for epochStr, epochValidators := range config.EpochsConfig {
		epoch, err = strconv.ParseInt(epochStr, 10, 64)
		if err != nil {
			return nil, err
		}

		var nodesConfig *epochNodesConfig
		nodesConfig, err = epochValidatorsToEpochNodesConfig(epochValidators)
		if err != nil {
			return nil, err
		}

		// shards without metachain shard
		epoch32 := uint32(epoch) // #nosec G115
		result[epoch32] = nodesConfig
		log.Debug("registry to nodes coordinator", "epoch", epoch32)
		result[epoch32].selector, err = ihgs.createSelector(nodesConfig)
		if err != nil {
			return nil, err
		}
	}

	return result, nil
}

func epochValidatorsToEpochNodesConfig(config *EpochValidators) (*epochNodesConfig, error) {
	result := &epochNodesConfig{}
	var err error

	result.electedList, err = serializableValidatorArrayToValidatorArray(config.ElectedValidators)
	if err != nil {
		return nil, err
	}

	result.eligibleList, err = serializableValidatorArrayToValidatorArray(config.EligibleValidators)
	if err != nil {
		return nil, err
	}

	result.waitingList, err = serializableValidatorArrayToValidatorArray(config.WaitingValidators)
	if err != nil {
		return nil, err
	}

	// registries saved before the leaving list was persisted have no
	// LeavingValidators field; the list stays empty until the next epoch start
	result.leavingList, err = serializableValidatorArrayToValidatorArray(config.LeavingValidators)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func serializableValidatorArrayToValidatorArray(sValidators []*SerializableValidator) ([]Validator, error) {
	result := make([]Validator, len(sValidators))
	var err error

	for i, v := range sValidators {
		result[i], err = NewValidator(v.OwnerAddress, v.PubKey, DefaultSelectionChances, v.Index)
		if err != nil {
			return nil, err
		}
	}

	return result, nil
}

func (ihgs *indexHashedNodesCoordinator) fillPublicKeyToValidatorMap() {
	ihgs.mutNodesConfig.Lock()
	defer ihgs.mutNodesConfig.Unlock()

	ihgs.publicKeyToValidatorMap = ihgs.computePublicKeyToValidatorMap(ihgs.nodesConfig)
}

// computePublicKeyToValidatorMap merges the elected and eligible validators of
// all given epoch configs into one lookup map, later epochs taking precedence
func (ihgs *indexHashedNodesCoordinator) computePublicKeyToValidatorMap(
	nodesConfig map[uint32]*epochNodesConfig,
) map[string]Validator {
	index := 0
	epochList := make([]uint32, len(nodesConfig))
	mapAllValidators := make(map[uint32]map[string]Validator)
	for epoch, epochConfig := range nodesConfig {
		epochConfig.mutNodesMaps.RLock()
		mapAllValidators[epoch] = ihgs.createPublicKeyToValidatorMap(epochConfig.electedList, epochConfig.eligibleList)
		epochConfig.mutNodesMaps.RUnlock()

		epochList[index] = epoch
		index++
	}

	sort.Slice(epochList, func(i, j int) bool {
		return epochList[i] < epochList[j]
	})

	publicKeyToValidatorMap := make(map[string]Validator)
	for _, epoch := range epochList {
		validatorsForEpoch := mapAllValidators[epoch]
		for pubKey, vInfo := range validatorsForEpoch {
			publicKeyToValidatorMap[pubKey] = vInfo
		}
	}

	return publicKeyToValidatorMap
}

func (ihgs *indexHashedNodesCoordinator) createPublicKeyToValidatorMap(
	elected []Validator,
	eligible []Validator,
) map[string]Validator {
	publicKeyToValidatorMap := make(map[string]Validator)

	for i := 0; i < len(elected); i++ {
		publicKeyToValidatorMap[string(elected[i].PubKey())] = elected[i]
	}

	for i := 0; i < len(eligible); i++ {
		publicKeyToValidatorMap[string(eligible[i].PubKey())] = eligible[i]
	}

	return publicKeyToValidatorMap
}

// GetValidatorWithPublicKey gets the validator with the given public key
func (ihgs *indexHashedNodesCoordinator) GetValidatorWithPublicKey(publicKey []byte) (Validator, error) {
	if len(publicKey) == 0 {
		return nil, ErrNilPubKey
	}
	ihgs.mutNodesConfig.RLock()
	v, ok := ihgs.publicKeyToValidatorMap[string(publicKey)]
	ihgs.mutNodesConfig.RUnlock()
	if ok {
		return v, nil
	}

	return nil, ErrValidatorNotFound
}

// ConsensusGroupSize returns the consensus group size for a specific shard
func (ihgs *indexHashedNodesCoordinator) ConsensusGroupSize() int {

	return ihgs.consensusGroupSize
}

// GetConsensusWhitelistedNodes return the whitelisted nodes allowed to send consensus messages, for each of the shards
func (ihgs *indexHashedNodesCoordinator) GetConsensusWhitelistedNodes(
	epoch uint32,
) (map[string]struct{}, error) {
	var err error
	elected := make(map[string]struct{})
	prevEpochConfigExists := false
	publicKeysPrevEpoch := make([][]byte, 0)

	if epoch > ihgs.startEpoch {
		publicKeysPrevEpoch, err = ihgs.GetAllElectedValidatorsKeys(epoch-1, false)
		if err == nil {
			prevEpochConfigExists = true
		} else {
			log.Warn("get consensus whitelisted nodes", "error", err.Error())
		}
	}

	if prevEpochConfigExists {
		for _, pubKey := range publicKeysPrevEpoch {
			elected[string(pubKey)] = struct{}{}
		}
	}

	publicKeysNewEpoch, errGetElected := ihgs.GetAllElectedValidatorsKeys(epoch, false)
	if errGetElected != nil {
		return nil, errGetElected
	}

	for _, pubKey := range publicKeysNewEpoch {
		elected[string(pubKey)] = struct{}{}
	}

	return elected, nil
}

// ComputeConsensusGroup will generate a list of validators based on the the elected list
// and each elected validator weight/chance
func (ihgs *indexHashedNodesCoordinator) ComputeConsensusGroup(
	randomness []byte,
	slot uint64,
	epoch uint32,
) (validatorsGroup []Validator, err error) {
	// check if component is ready (previous epoch nodes config is loaded)
	if !ihgs.stateReady.Load() {
		return nil, ErrNodesCoordinatorNotReady
	}

	var selector RandomSelector
	var electedList []Validator

	log.Trace("computing consensus group for",
		"epoch", epoch,
		"slot", slot,
		"randomness", randomness)

	if len(randomness) == 0 {
		return nil, ErrNilRandomness
	}

	ihgs.mutNodesConfig.RLock()
	nodesConfig, ok := ihgs.nodesConfig[epoch]
	if ok {
		selector = nodesConfig.selector
		electedList = nodesConfig.electedList
	}
	ihgs.mutNodesConfig.RUnlock()

	if !ok {
		return nil, fmt.Errorf("%w epoch=%v", ErrEpochNodesConfigDoesNotExist, epoch)
	}

	key := []byte(fmt.Sprintf(keyFormat, string(randomness), slot, epoch))
	validators := ihgs.searchConsensusForKey(key)
	if validators != nil {
		return validators, nil
	}

	consensusSize := uint32(ihgs.ConsensusGroupSize()) // #nosec G115
	randomness = []byte(fmt.Sprintf("%d-%s", slot, randomness))

	log.Debug("computeValidatorsGroup",
		"epoch", epoch,
		"slot", slot,
		"randomness", randomness,
		"consensus size", consensusSize,
		"electedList list length", len(electedList))

	tempList, err := selectValidators(selector, randomness, consensusSize, electedList, slot)
	if err != nil {
		return nil, err
	}

	size := 0
	for _, v := range tempList {
		size += v.Size()
	}

	ihgs.consensusGroupCacher.Put(key, tempList, size)

	return tempList, nil
}

func (ihgs *indexHashedNodesCoordinator) searchConsensusForKey(key []byte) []Validator {
	value, ok := ihgs.consensusGroupCacher.Get(key)
	if ok {
		consensusGroup, typeOk := value.([]Validator)
		if typeOk {
			return consensusGroup
		}
	}
	return nil
}

func selectValidators(
	selector RandomSelector,
	randomness []byte,
	consensusSize uint32,
	electedList []Validator,
	slot uint64,
) ([]Validator, error) {
	if check.IfNil(selector) {
		return nil, ErrNilRandomSelector
	}
	if len(randomness) == 0 {
		return nil, ErrNilRandomness
	}

	selectedIndexes, err := selector.Select(randomness, consensusSize)
	if err != nil {
		return nil, err
	}

	electedListLen := uint32(len(electedList)) // #nosec G115
	// check if the selected indexes are within the range of the elected list
	if slices.Max(selectedIndexes) > electedListLen || len(electedList) == 0 {
		return nil, ErrSmallElectedListSize
	}

	consensusGroup := make([]Validator, consensusSize)
	for i := range consensusGroup {
		consensusGroup[i] = electedList[selectedIndexes[i]]
	}

	displayValidatorsForRandomness(consensusGroup, randomness, slot)

	return consensusGroup, nil
}

func displayValidatorsForRandomness(validators []Validator, randomness []byte, slot uint64) {
	if log.GetLevel() != logger.LogTrace {
		return
	}

	strValidators := ""

	for _, v := range validators {
		strValidators += "\n" + hex.EncodeToString(v.PubKey())
	}

	log.Trace("selectValidators", "slot", slot, "randomness", randomness, "validators", strValidators)
}

// GetAllElectedValidatorsKeys will return all validators elected public keys
func (ihgs *indexHashedNodesCoordinator) GetAllElectedValidatorsKeys(epoch uint32, ownerKey bool) ([][]byte, error) {
	validatorsPubKeys := make([][]byte, 0)

	ihgs.mutNodesConfig.RLock()
	nodesConfig, ok := ihgs.nodesConfig[epoch]
	ihgs.mutNodesConfig.RUnlock()

	if !ok {
		return nil, fmt.Errorf("%w epoch=%v", ErrEpochNodesConfigDoesNotExist, epoch)
	}

	nodesConfig.mutNodesMaps.RLock()
	defer nodesConfig.mutNodesMaps.RUnlock()

	for i := 0; i < len(nodesConfig.electedList); i++ {
		if ownerKey {
			validatorsPubKeys = append(validatorsPubKeys, nodesConfig.electedList[i].OwnerAddress())
		} else {
			validatorsPubKeys = append(validatorsPubKeys, nodesConfig.electedList[i].PubKey())
		}
	}

	return validatorsPubKeys, nil
}

// GetAllEligibleValidatorsPubGetAllEligibleValidatorsKeyslicKeys will return all validators public keys for all shards
func (ihgs *indexHashedNodesCoordinator) GetAllEligibleValidatorsKeys(epoch uint32, ownerKey bool) ([][]byte, error) {
	validatorsPubKeys := make([][]byte, 0)

	ihgs.mutNodesConfig.RLock()
	nodesConfig, ok := ihgs.nodesConfig[epoch]
	ihgs.mutNodesConfig.RUnlock()

	if !ok {
		return nil, fmt.Errorf("%w epoch=%v", ErrEpochNodesConfigDoesNotExist, epoch)
	}

	nodesConfig.mutNodesMaps.RLock()
	defer nodesConfig.mutNodesMaps.RUnlock()

	for i := 0; i < len(nodesConfig.eligibleList); i++ {
		if ownerKey {
			validatorsPubKeys = append(validatorsPubKeys, nodesConfig.eligibleList[i].OwnerAddress())
		} else {
			validatorsPubKeys = append(validatorsPubKeys, nodesConfig.eligibleList[i].PubKey())
		}
	}

	return validatorsPubKeys, nil
}

// GetAllWaitingValidatorsKeys will return all validators public keys for all shards
func (ihgs *indexHashedNodesCoordinator) GetAllWaitingValidatorsKeys(epoch uint32, ownerKey bool) ([][]byte, error) {
	validatorsPubKeys := make([][]byte, 0)

	ihgs.mutNodesConfig.RLock()
	nodesConfig, ok := ihgs.nodesConfig[epoch]
	ihgs.mutNodesConfig.RUnlock()

	if !ok {
		return nil, fmt.Errorf("%w epoch=%v", ErrEpochNodesConfigDoesNotExist, epoch)
	}

	nodesConfig.mutNodesMaps.RLock()
	defer nodesConfig.mutNodesMaps.RUnlock()

	for i := 0; i < len(nodesConfig.waitingList); i++ {
		if ownerKey {
			validatorsPubKeys = append(validatorsPubKeys, nodesConfig.waitingList[i].OwnerAddress())
		} else {
			validatorsPubKeys = append(validatorsPubKeys, nodesConfig.waitingList[i].PubKey())
		}
	}

	return validatorsPubKeys, nil
}

// GetAllLeavingValidatorsKeys will return all leaving validators public keys.
// The leaving list is populated by computeNodesConfigFromList from validators
// whose list is jailed, minus the ones promoted back to eligible to fill the
// consensus size.
func (ihgs *indexHashedNodesCoordinator) GetAllLeavingValidatorsKeys(epoch uint32, ownerKey bool) ([][]byte, error) {
	validatorsPubKeys := make([][]byte, 0)

	ihgs.mutNodesConfig.RLock()
	nodesConfig, ok := ihgs.nodesConfig[epoch]
	ihgs.mutNodesConfig.RUnlock()

	if !ok {
		return nil, fmt.Errorf("%w epoch=%v", ErrEpochNodesConfigDoesNotExist, epoch)
	}

	nodesConfig.mutNodesMaps.RLock()
	defer nodesConfig.mutNodesMaps.RUnlock()

	for i := 0; i < len(nodesConfig.leavingList); i++ {
		if ownerKey {
			validatorsPubKeys = append(validatorsPubKeys, nodesConfig.leavingList[i].OwnerAddress())
		} else {
			validatorsPubKeys = append(validatorsPubKeys, nodesConfig.leavingList[i].PubKey())
		}
	}

	return validatorsPubKeys, nil
}

// CheckValidatorSlot - TODO: refactor
func (ihgs *indexHashedNodesCoordinator) CheckValidatorSlot(epoch uint32, slotIndex int64, pubkey []byte) bool {
	ihgs.mutNodesConfig.RLock()
	nodesConfig, ok := ihgs.nodesConfig[epoch]
	ihgs.mutNodesConfig.RUnlock()

	if !ok {
		return false
	}

	nodesConfig.mutNodesMaps.RLock()
	defer nodesConfig.mutNodesMaps.RUnlock()

	if len(nodesConfig.electedList) < 1 {
		return false
	}

	index := slotIndex % int64(len(nodesConfig.electedList))

	return bytes.Equal(pubkey, nodesConfig.electedList[index].PubKey())

}

func (ihgs *indexHashedNodesCoordinator) SetEpochValidatorsInfo(epoch uint32, validatorsInfo []*state.ValidatorInfo) error {
	list := make([]*block.EValidatorInfo, 0)
	for _, vi := range validatorsInfo {
		info := &block.EValidatorInfo{
			OwnerAddress: make([]byte, len(vi.OwnerAddress)),
			PublicKey:    make([]byte, len(vi.PublicKey)),
			List:         vi.List,
			Index:        vi.Index,
			TempRating:   vi.TempRating,
		}
		copy(info.OwnerAddress, vi.OwnerAddress)
		copy(info.PublicKey, vi.PublicKey)
		list = append(list, info)
	}

	// delete old epoch history
	for vie := range ihgs.validatorsInfo {
		if vie+2 < epoch {
			delete(ihgs.validatorsInfo, vie)
		}
	}

	ihgs.validatorsInfo[epoch] = list
	return nil
}

func (ihgs *indexHashedNodesCoordinator) getEpochValidatorsInfo(epoch uint32) ([]*block.EValidatorInfo, error) {
	info, ok := ihgs.validatorsInfo[epoch]
	if !ok {
		return nil, ErrValidatorListNotFound
	}

	return info, nil
}

// EpochStartPrepare is called when an epoch start event is observed, but not yet confirmed/committed.
// Some components may need to do some initialization on this event
func (ihgs *indexHashedNodesCoordinator) EpochStartPrepare(metaHdr data.HeaderHandler) {
	if !metaHdr.GetIsEpochStart() {
		log.Error("could not process EpochStartPrepare on nodesCoordinator - not epoch start block")
		return
	}
	if _, ok := metaHdr.(*block.Block); !ok {
		log.Error("could not process EpochStartPrepare on nodesCoordinator - invalid block")
		return
	}
	randomness := metaHdr.GetPrevRandSeed()
	newEpoch := metaHdr.GetEpoch()

	allValidatorInfo, err := ihgs.getEpochValidatorsInfo(newEpoch)
	if err != nil {
		log.Error("could not get validators from list - do nothing on nodesCoordinator epochStartPrepare", "newEpoch", newEpoch, "error", err.Error())
		return
	}

	newNodesConfig, err := ihgs.computeNodesConfigFromList(allValidatorInfo)
	if err != nil {
		log.Error("could not compute nodes config from list - do nothing on nodesCoordinator epochStartPrepare", "error", err.Error())
		return
	}

	shufflerArgs := ArgsUpdateNodes{
		Elected:  newNodesConfig.electedList,
		Eligible: newNodesConfig.eligibleList,
		Rand:     randomness,
		Epoch:    newEpoch,
	}

	resUpdateNodes, err := ihgs.shuffler.UpdateNodeLists(shufflerArgs)
	if err != nil {
		log.Error("could not compute UpdateNodeLists - do nothing on nodesCoordinator epochStartPrepare", "err", err.Error())
		return
	}

	err = ihgs.SetNodes(resUpdateNodes.Elected, resUpdateNodes.Eligible, newNodesConfig.waitingList, newNodesConfig.leavingList, newEpoch)
	if err != nil {
		log.Error("set nodes per shard failed", "error", err.Error())
	}

	ihgs.fillPublicKeyToValidatorMap()
	err = ihgs.saveState(randomness)
	if err != nil {
		log.Error("saving nodes coordinator config failed", "error", err.Error())
	}

	ihgs.mutNodesConfig.RLock()
	displayCfg := ihgs.nodesConfig[newEpoch]
	ihgs.mutNodesConfig.RUnlock()

	if displayCfg != nil {
		displayCfg.mutNodesMaps.RLock()
		elected := displayCfg.electedList
		eligible := displayCfg.eligibleList
		waiting := displayCfg.waitingList
		leaving := displayCfg.leavingList
		displayCfg.mutNodesMaps.RUnlock()

		displayNodesConfiguration(elected, eligible, waiting, leaving)
	}

	ihgs.mutSavedStateKey.Lock()
	ihgs.savedStateKey = randomness
	ihgs.mutSavedStateKey.Unlock()

	ihgs.consensusGroupCacher.Clear()
}

// setNodes loads the distribution of nodes per shard into the nodes management component
func (ihgs *indexHashedNodesCoordinator) SetNodes(
	elected []Validator,
	eligible []Validator,
	waiting []Validator,
	leaving []Validator,
	epoch uint32,
) error {
	ihgs.mutNodesConfig.Lock()
	defer ihgs.mutNodesConfig.Unlock()

	nodesConfig, ok := ihgs.nodesConfig[epoch]
	if !ok {
		log.Debug("did not find nodesConfig", "epoch", epoch)
		nodesConfig = &epochNodesConfig{}
	}

	nodesConfig.mutNodesMaps.Lock()
	defer nodesConfig.mutNodesMaps.Unlock()

	if elected == nil || eligible == nil {
		return ErrNilInputNodesMap
	}

	var err error
	// nbShards holds number of shards without meta
	nodesConfig.electedList = elected
	nodesConfig.eligibleList = eligible
	nodesConfig.waitingList = waiting
	nodesConfig.leavingList = leaving
	nodesConfig.selector, err = ihgs.createSelector(nodesConfig)
	if err != nil {
		return err
	}

	ihgs.nodesConfig[epoch] = nodesConfig

	return nil
}

// createSelectors creates the consensus group selector for each shard
// Not concurrent safe, needs to be called under mutex
func (ihgs *indexHashedNodesCoordinator) createSelector(
	nodesConfig *epochNodesConfig,
) (RandomSelector, error) {
	log.Debug("create selector")
	weights, err := ihgs.ValidatorsWeights(nodesConfig.electedList)
	if err != nil {
		return nil, err
	}

	selector, err := NewSelectorExpandedList(weights, ihgs.hasher)
	if err != nil {
		return nil, err
	}

	return selector, nil
}

// ValidatorsWeights returns the weights/chances for each of the given validators
func (ihgs *indexHashedNodesCoordinator) ValidatorsWeights(validators []Validator) ([]uint32, error) {
	weights := make([]uint32, len(validators))
	for i := range validators {
		weights[i] = DefaultSelectionChances
	}

	return weights, nil
}

// GetSavedStateKey returns the key for the last nodes coordinator saved state
func (ihgs *indexHashedNodesCoordinator) GetSavedStateKey() []byte {
	ihgs.mutSavedStateKey.RLock()
	key := ihgs.savedStateKey
	ihgs.mutSavedStateKey.RUnlock()

	return key
}

// IsInterfaceNil returns true if there is no value under the interface
func (ihgs *indexHashedNodesCoordinator) IsInterfaceNil() bool {
	return ihgs == nil
}

// GetOwnPublicKey will return current node public key  for block sign
func (ihgs *indexHashedNodesCoordinator) GetOwnPublicKey() []byte {
	return ihgs.selfPubKey
}

func (ihgs *indexHashedNodesCoordinator) computeNodesConfigFromList(
	validators []*block.EValidatorInfo,
) (*epochNodesConfig, error) {
	electedList := make([]Validator, 0)
	eligibleList := make([]Validator, 0)
	waitingList := make([]Validator, 0)
	leavingList := make([]Validator, 0)

	for _, validatorInfo := range validators {
		currentValidator, err := NewValidator(validatorInfo.OwnerAddress, validatorInfo.PublicKey, DefaultSelectionChances, validatorInfo.Index)
		if err != nil {
			return nil, err
		}

		switch validatorInfo.List {
		case string(core.EligibleList):
			eligibleList = append(eligibleList, currentValidator)
		case string(core.ElectedList):
			electedList = append(electedList, currentValidator)
		case string(core.WaitingList):
			waitingList = append(waitingList, currentValidator)
		case string(core.InactiveList):
			log.Debug("inactive validator", "pk", validatorInfo.PublicKey)
		case string(core.JailedList):
			log.Debug("adding to leavingList", "pk", validatorInfo.PublicKey)
			leavingList = append(leavingList, currentValidator)
		}
	}

	numToStay := ihgs.consensusGroupSize - (len(electedList) + len(eligibleList))
	if numToStay > 0 {
		if len(leavingList) >= numToStay {
			eligibleList = append(eligibleList, leavingList[:numToStay]...)
			leavingList = leavingList[numToStay:]
		}
	}

	sort.Sort(validatorList(leavingList))
	sort.Sort(validatorList(eligibleList))
	sort.Sort(validatorList(electedList))
	sort.Sort(validatorList(waitingList))

	if len(electedList) == 0 {
		return nil, fmt.Errorf("%w elected list size is zero. No validators found", ErrListSizeZero)
	}

	newNodesConfig := &epochNodesConfig{
		electedList:  electedList,
		eligibleList: eligibleList,
		waitingList:  waitingList,
		leavingList:  leavingList,
	}

	return newNodesConfig, nil
}

func (ihgs *indexHashedNodesCoordinator) GetConsensusValidatorsPublicKeys(randomness []byte, slot uint64, epoch uint32) ([]string, error) {
	consensusNodes, err := ihgs.ComputeConsensusGroup(randomness, slot, epoch)
	if err != nil {
		return nil, err
	}

	pubKeys := make([]string, 0)

	for _, v := range consensusNodes {
		pubKeys = append(pubKeys, string(v.PubKey()))
	}

	return pubKeys, nil
}
