package processorNode

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core/accumulator"
	"github.com/klever-io/klever-go/core/bootstrap/disabled"
	consensusMock "github.com/klever-io/klever-go/core/consensus/mock"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/block/bootstrapStorage"
	"github.com/klever-io/klever-go/core/process/headerCheck"
	"github.com/klever-io/klever-go/core/process/transactionLog"
	"github.com/klever-io/klever-go/core/watchdog"
	"github.com/klever-io/klever-go/crypto/hashing/blake2b"
	cryptoMock "github.com/klever-io/klever-go/crypto/mock"
	"github.com/klever-io/klever-go/crypto/signing"
	"github.com/klever-io/klever-go/crypto/signing/ed25519/singlesig"
	"github.com/klever-io/klever-go/crypto/signing/mcl"
	llsig "github.com/klever-io/klever-go/crypto/signing/mcl/multisig"
	mclsinglesig "github.com/klever-io/klever-go/crypto/signing/mcl/singlesig"
	"github.com/klever-io/klever-go/crypto/signing/multisig"
	"github.com/klever-io/klever-go/data/endProcess"
	"github.com/klever-io/klever-go/eventNotifier/notifier"
	integrationTestsMock "github.com/klever-io/klever-go/integrationTest/mock"
	"github.com/klever-io/klever-go/kapps"
	networkMock "github.com/klever-io/klever-go/network/p2p/mock"
	"github.com/klever-io/klever-go/node"
	heartbeatMock "github.com/klever-io/klever-go/node/heartbeat/mock"
	"github.com/klever-io/klever-go/ntp"
	"github.com/klever-io/klever-go/sharding"
	shardingMocks "github.com/klever-io/klever-go/sharding/networksharding"
	"github.com/klever-io/klever-go/statusHandler"
	"github.com/klever-io/klever-go/storage/lrucache"
	"github.com/klever-io/klever-go/tools/debug"
	"github.com/klever-io/klever-go/tools/typeConverters/uint64ByteSlice"
)

// NewProcessorNode creates a new Node instance
func NewProcessorNode(opts ...Option) (*ProcessorNode, error) {
	node := &ProcessorNode{
		Ctx:                      context.Background(),
		CurrentSendingGoRoutines: 0,
		AppStatusHandler:         statusHandler.NewNilStatusHandler(),
		QueryHandlers:            make(map[string]debug.QueryHandler),
	}
	for _, opt := range opts {
		err := opt(node)
		if err != nil {
			return nil, errors.New("error applying option: " + err.Error())
		}
	}

	return node, nil
}

func CreateNodesWithNodesCoordinatorAndHeaderSigVerifier(numOfNodes int, numConsensusSize int, mainConfig config.Config) ([]*ProcessorNode, error) {
	singleSigner := &mclsinglesig.BlsSingleSigner{}
	blockKeyGen := signing.NewKeyGenerator(mcl.NewSuiteBLS12())

	nodeList := make([]*ProcessorNode, numOfNodes)
	completeNodesList := make([]Connectable, numOfNodes)

	txSignKeys, txSignPkList, blkSignKeys, blkSignPkList := GenerateNodeKeys(numOfNodes)
	electedValidatorsList := GenValidatorsFromPubKeys(blkSignPkList, txSignPkList)
	electedMapForNodesCoordinator, err := sharding.NodesInfoToValidators(electedValidatorsList)
	if err != nil {
		return nil, err
	}
	eligibleValidatorsList := []sharding.GenesisNodeInfoHandler{}

	nodesSetup := &mock.NodesSetupStub{
		InitialNodesInfoCalled: func() ([]sharding.GenesisNodeInfoHandler, []sharding.GenesisNodeInfoHandler, error) {
			return electedValidatorsList, eligibleValidatorsList, nil
		},
		MinNumberOfNodesCalled: func() uint32 {
			return uint32(numOfNodes) // #nosec G115
		},
	}

	shufflerArgs := &sharding.NodesShufflerArgs{
		Nodes:                uint32(numOfNodes), // #nosec G115
		MaxNodesEnableConfig: []config.MaxNodesChangeConfig{{}},
	}

	nodeShuffler, err := sharding.NewHashValidatorsShuffler(shufflerArgs)
	if err != nil {
		return nil, err
	}
	epochStartSubscriber := notifier.NewEpochStartSubscriptionHandler()
	bootStorer := CreateMemUnit()
	for i, v := range electedValidatorsList {
		cache, err := lrucache.NewCache(10000)
		if err != nil {
			return nil, err
		}

		argumentsNodesCoordinator := sharding.ArgNodesCoordinator{
			ConsensusGroupSize:  numConsensusSize,
			Marshalizer:         TestMarshalizer,
			Hasher:              TestHasher,
			Shuffler:            nodeShuffler,
			EpochStartNotifier:  epochStartSubscriber,
			BootStorer:          bootStorer,
			ElectedNodes:        electedMapForNodesCoordinator,
			EligibleNodes:       []sharding.Validator{},
			SelfPublicKey:       v.PubKeyBytes(),
			ConsensusGroupCache: cache,
			ShuffledOutHandler:  &integrationTestsMock.ShuffledOutHandlerStub{},
		}

		nodesCoordinatorInstance, err := sharding.NewNodesCoordinator(argumentsNodesCoordinator)
		if err != nil {
			fmt.Println("error creating node coordinator")
			return nil, err
		}

		hasher := &blake2b.Blake2b{HashSize: multisig.BlsHashSize}
		llsig := &llsig.BlsMultiSigner{Hasher: hasher}

		multiSigner, err := multisig.NewBLSMultisig(
			llsig,
			blkSignPkList,
			blkSignKeys[i].Sk,
			blockKeyGen,
			uint16(i), // #nosec G115
		)
		if err != nil {
			fmt.Println("error multisig")
			return nil, err
		}

		args := headerCheck.ArgsHeaderSigVerifier{
			Marshalizer:             TestMarshalizer,
			Hasher:                  TestHasher,
			NodesCoordinator:        nodesCoordinatorInstance,
			MultiSigVerifier:        multiSigner,
			SingleSigVerifier:       singleSigner,
			KeyGen:                  blockKeyGen,
			FallbackHeaderValidator: &mock.FallBackHeaderValidatorStub{},
		}
		headerSig, err := headerCheck.NewHeaderSigVerifier(&args)
		if err != nil {
			return nil, err
		}
		headerIntegrityVerifier, err := createHeaderIntegrityVerifier()
		if err != nil {
			return nil, err
		}
		processorNode, err := NewTestProcessorNodeWithCustomNodesCoordinator(
			epochStartSubscriber,
			nodesCoordinatorInstance,
			txSignKeys[i],
			txSignPkList,
			blkSignKeys[i],
			blkSignPkList,
			i,
			headerSig,
			headerIntegrityVerifier,
			nodesSetup,
			mainConfig,
		)
		if err != nil {
			return nil, err
		}

		if elastcEnabled {
			idx, err := processorNode.createIndexer()
			if err != nil {
				return nil, err
			}

			processorNode.Indexer = idx
		}

		nodeList[i] = processorNode
		completeNodesList[i] = processorNode
	}

	ConnectNodes(completeNodesList)

	return nodeList, nil
}

func NewBaseProcessorNode(mainConfig config.Config) (*ProcessorNode, error) {
	txSignKeys, txSignPubkeyList, blkSignKeys, blockSignPubkeyList := GenerateNodeKeys(1)
	electedValidatorsList := GenValidatorsFromPubKeys(blockSignPubkeyList, txSignPubkeyList)
	eligibleValidatorsList := []sharding.GenesisNodeInfoHandler{}
	validatorBlockSignKey, _ := blkSignKeys[0].Pk.ToByteArray()

	nodeAccount := CreateNodeAccount()

	nodesSetup := &mock.NodesSetupStub{
		InitialNodesInfoCalled: func() ([]sharding.GenesisNodeInfoHandler, []sharding.GenesisNodeInfoHandler, error) {
			return electedValidatorsList, eligibleValidatorsList, nil
		},
		MinNumberOfNodesCalled: func() uint32 {
			return 1
		},
	}

	p2pConfig := CreateP2PConfigWithNoDiscovery()
	messenger, err := CreateMessengerFromP2P(p2pConfig)
	if err != nil {
		return nil, err
	}

	//* Considering just one node
	nodesCooordinatorStub := &shardingMocks.NodesCoordinatorStub{
		GetValidatorsPublicKeysCalled: func(randomness []byte, slot uint64, epoch uint32) ([]string, error) {
			return blockSignPubkeyList, nil
		},
		ComputeValidatorsGroupCalled: func(randomness []byte, round uint64, epoch uint32) (validators []sharding.Validator, err error) {
			validatorInstance, _ := sharding.NewValidator(validatorBlockSignKey, validatorBlockSignKey, 1, defaultChancesSelection)
			return []sharding.Validator{validatorInstance}, nil
		},
		GetValidatorWithPublicKeyCalled: func(publicKey []byte) (sharding.Validator, error) {
			validatorInstance, _ := sharding.NewValidator(validatorBlockSignKey, validatorBlockSignKey, 1, defaultChancesSelection)
			return validatorInstance, nil
		},
		ConsensusSize:            1,
		ConsensusGroupSizeCalled: func() int { return 1 },
	}

	peerSign := &mock.PeerSignatureHandler{Signer: &singlesig.Ed25519Signer{}}

	var txAccumulator node.Accumulator
	txAccumulatorConfig := mainConfig.Antiflood.TxAccumulator
	txAccumulator, err = accumulator.NewTimeAccumulator(
		time.Duration(txAccumulatorConfig.MaxAllowedTimeInMilliseconds)*time.Millisecond,
		time.Duration(txAccumulatorConfig.MaxDeviationTimeInMilliseconds)*time.Millisecond,
	)
	if err != nil {
		return nil, err
	}

	forkControllerStub := &integrationTestsMock.ForkControllerStub{
		ProcessorFlowITOPriceCalled: func() bool {
			return true
		},
		ClaimKFICalled: func() bool {
			return true
		},
		FixStakingBucketsCalled: func() bool {
			return true
		},
		KdaFprCalled: func() bool {
			return true
		},
		BigBucketsComputeCalled: func() bool {
			return true
		},
		FPRComputeAndKdaFeeFlowCalled: func() bool {
			return true
		},
		FixDelegationSameEpochCalled: func() bool {
			return true
		},
		EnableSmartContractsCalled: func() bool {
			return true
		},
	}

	proposalController, err := kapps.NewProposalController(forkControllerStub)
	if err != nil {
		return nil, err
	}

	syncer := ntp.NewSyncTime(mainConfig.NTP, nil)
	syncer.StartSyncingTime()

	time.Sleep(1000 * time.Millisecond)
	ntpTime := syncer.CurrentTime()

	pn := &ProcessorNode{
		GenesisTime:              ntpTime,
		AddressPubkeyConverter:   TestAddressPubkeyConverter,
		PubkeyTxSignList:         txSignPubkeyList,
		PubkeyBlockSignList:      blockSignPubkeyList,
		Hasher:                   getHasher(),
		TxSignHasher:             getHasher(),
		TxSignMarshalizer:        getMarshalizer(),
		InternalMarshalizer:      getMarshalizer(),
		PeerSigHandler:           peerSign,
		SingleSigner:             &singlesig.Ed25519Signer{},
		Uint64ByteSliceConverter: uint64ByteSlice.NewBigEndianConverter(),
		MainConfig:               mainConfig,
		Messenger:                messenger,
		ConsensusGroupSize:       1,
		SyncTimer:                &consensusMock.SyncTimerMock{},
		NodesCoordinator:         nodesCooordinatorStub,
		HeaderSigVerifier:        &consensusMock.HeaderSigVerifierStub{},
		ChainID:                  ChainID,
		MinTransactionVersion:    MinTransactionVersion,
		NodesSetup:               nodesSetup, // review
		NodeTxSignKeyPair:        txSignKeys[0],
		NodeBlockSignKeyPair:     blkSignKeys[0],
		NodeAccount:              nodeAccount,
		EpochNotifier:            notifier.NewGenericEpochNotifier(), // review
		MultiSigner:              InitMultiSignerMock(),
		BootStorer: &mock.BoostrapStorerMock{
			PutCalled: func(slot int64, bootData *bootstrapStorage.BootstrapData) error {
				return nil
			},
		},
		EnableEpochsConfig: config.EnableEpochsConfig{
			EnableEpochs: config.EnableEpochs{
				ProcessorFlowITOPrice: 0,
			},
		},
		TxSingleSigner: &singlesig.Ed25519Signer{},
		AppStatusHandler: &mock.AppStatusHandlerStub{
			SetUInt64ValueHandler: func(key string, value uint64) {

			},
		},
		PeerDenialEvaluator:     &networkMock.PeerDenialEvaluatorStub{},
		ValidatorsProvider:      &heartbeatMock.ValidatorsProviderStub{},
		InputAntifloodHandler:   disabled.NewAntiFloodHandler(),
		TxAccumulator:           txAccumulator,
		ChanStopNodeProcess:     make(chan endProcess.ArgEndProcess),
		PeerHonestyHandler:      &mock.PeerHonestyHandlerStub{},
		Watchdog:                &watchdog.DisabledWatchdog{},
		FallbackHeaderValidator: &mock.FallBackHeaderValidatorStub{},
		NodeRedundancyHandler:   &consensusMock.NodeRedundancyHandlerStub{},
		Indexer:                 &consensusMock.IndexerMock{}, //disabled
		FeeHandler:              freeFeeHandlerMock(),         //todo: denominate fees
		ProposalController:      proposalController,
	}

	pn.ForkController = forkControllerStub

	err = pn.initDataPools()
	if err != nil {
		return nil, err
	}
	return pn, nil
}

func CreateNodesWithTxSetup(numOfNodes int, numConsensusSize int, mainConfig config.Config) ([]*ProcessorNode, error) {
	nodeList := make([]*ProcessorNode, numOfNodes)
	completeNodesList := make([]Connectable, numOfNodes)

	txSignKeys, txSignPkList, blkSignKeys, blkSignPkList := GenerateNodeKeys(numOfNodes)
	electedValidatorsList := GenValidatorsFromPubKeys(blkSignPkList, txSignPkList)
	electedMapForNodesCoordinator, err := sharding.NodesInfoToValidators(electedValidatorsList)
	if err != nil {
		return nil, err
	}
	eligibleValidatorsList := []sharding.GenesisNodeInfoHandler{}

	nodesSetup := &mock.NodesSetupStub{
		InitialNodesInfoCalled: func() ([]sharding.GenesisNodeInfoHandler, []sharding.GenesisNodeInfoHandler, error) {
			return electedValidatorsList, eligibleValidatorsList, nil
		},
		MinNumberOfNodesCalled: func() uint32 {
			return uint32(numOfNodes) // #nosec G115
		},
	}

	shufflerArgs := &sharding.NodesShufflerArgs{
		Nodes:                uint32(numOfNodes), // #nosec G115
		MaxNodesEnableConfig: []config.MaxNodesChangeConfig{{}},
	}

	nodeShuffler, err := sharding.NewHashValidatorsShuffler(shufflerArgs)
	if err != nil {
		return nil, err
	}
	epochStartSubscriber := notifier.NewEpochStartSubscriptionHandler()
	bootStorer := CreateMemUnit()
	for i, v := range electedValidatorsList {
		cache, err := lrucache.NewCache(10000)
		if err != nil {
			return nil, err
		}

		argumentsNodesCoordinator := sharding.ArgNodesCoordinator{
			ConsensusGroupSize:  numConsensusSize,
			Marshalizer:         TestMarshalizer,
			Hasher:              TestHasher,
			Shuffler:            nodeShuffler,
			EpochStartNotifier:  epochStartSubscriber,
			BootStorer:          bootStorer,
			ElectedNodes:        electedMapForNodesCoordinator,
			EligibleNodes:       []sharding.Validator{},
			SelfPublicKey:       v.PubKeyBytes(),
			ConsensusGroupCache: cache,
			ShuffledOutHandler:  &integrationTestsMock.ShuffledOutHandlerStub{},
		}

		nodesCoordinatorInstance, err := sharding.NewNodesCoordinator(argumentsNodesCoordinator)
		if err != nil {
			fmt.Println("error creating node coordinator")
			return nil, err
		}

		processorNode, err := NewTestProcessorNodeWithTxSetup(
			epochStartSubscriber,
			nodesCoordinatorInstance,
			txSignKeys[i],
			txSignPkList,
			blkSignKeys[i],
			blkSignPkList,
			i,
			nodesSetup,
			mainConfig,
		)
		if err != nil {
			return nil, err
		}

		nodeList[i] = processorNode
		completeNodesList[i] = processorNode
	}

	ConnectNodes(completeNodesList)

	return nodeList, nil
}

// NewTestProcessorNodeWithTxSetup returns a new TestProcessorNode instance with custom NodesCoordinator
func NewTestProcessorNodeWithTxSetup(
	epochStartNotifier notifier.EpochStartNotifier,
	nodesCoordinator sharding.NodesCoordinator,
	txSignKeyPair *NodeKeyPair,
	txSignPubkeyList []string,
	blockSignKeyPair *NodeKeyPair,
	blockSignPubkeyList []string,
	keyIndex int,
	nodeSetup sharding.GenesisNodesSetupHandler,
	mainConfig config.Config,
) (*ProcessorNode, error) {
	singleSigner := &singlesig.Ed25519Signer{}
	blockSingleSigner := &mclsinglesig.BlsSingleSigner{}

	p2pConfig := CreateP2PConfigWithNoDiscovery()
	messenger, err := CreateMessengerFromP2P(p2pConfig)
	if err != nil {
		return nil, err
	}

	var txAccumulator node.Accumulator
	txAccumulatorConfig := mainConfig.Antiflood.TxAccumulator
	txAccumulator, err = accumulator.NewTimeAccumulator(
		time.Duration(txAccumulatorConfig.MaxAllowedTimeInMilliseconds)*time.Millisecond,
		time.Duration(txAccumulatorConfig.MaxDeviationTimeInMilliseconds)*time.Millisecond,
	)
	if err != nil {
		return nil, err
	}

	forkControllerStub := &integrationTestsMock.ForkControllerStub{
		ProcessorFlowITOPriceCalled: func() bool {
			return true
		},
		ClaimKFICalled: func() bool {
			return true
		},
		FixStakingBucketsCalled: func() bool {
			return true
		},
		KdaFprCalled: func() bool {
			return true
		},
	}

	proposalController, err := kapps.NewProposalController(forkControllerStub)
	if err != nil {
		return nil, err
	}

	syncer := ntp.NewSyncTime(mainConfig.NTP, nil)
	syncer.StartSyncingTime()

	time.Sleep(1000 * time.Millisecond)
	ntpTime := syncer.CurrentTime()

	pn := &ProcessorNode{
		GenesisTime:              ntpTime,
		AddressPubkeyConverter:   TestAddressPubkeyConverter,
		PubkeyTxSignList:         txSignPubkeyList,
		PubkeyBlockSignList:      blockSignPubkeyList,
		MainConfig:               mainConfig,
		Hasher:                   getHasher(),
		TxSignHasher:             getHasher(),
		TxSignMarshalizer:        getMarshalizer(),
		InternalMarshalizer:      getMarshalizer(),
		SingleSigner:             &singlesig.Ed25519Signer{},
		TxSingleSigner:           &singlesig.Ed25519Signer{},
		Uint64ByteSliceConverter: uint64ByteSlice.NewBigEndianConverter(),
		PeerSigHandler:           &mock.PeerSignatureHandler{Signer: &singlesig.Ed25519Signer{}},
		BootStorer: &mock.BoostrapStorerMock{
			PutCalled: func(slot int64, bootData *bootstrapStorage.BootstrapData) error {
				return nil
			},
		},
		Messenger:          messenger,
		NodesCoordinator:   nodesCoordinator,
		HeaderSigVerifier:  &consensusMock.HeaderSigVerifierStub{},
		ChainID:            ChainID,
		NodesSetup:         nodeSetup,
		ConsensusGroupSize: len(blockSignPubkeyList), // check
		SyncTimer:          syncer,
		AppStatusHandler: &mock.AppStatusHandlerStub{
			SetUInt64ValueHandler: func(key string, value uint64) {

			},
		},
		PeerDenialEvaluator:     &networkMock.PeerDenialEvaluatorStub{},
		ValidatorsProvider:      &heartbeatMock.ValidatorsProviderStub{},
		InputAntifloodHandler:   disabled.NewAntiFloodHandler(),
		TxAccumulator:           txAccumulator,
		MinTransactionVersion:   MinTransactionVersion,
		EpochNotifier:           notifier.NewGenericEpochNotifier(),
		ChanStopNodeProcess:     make(chan endProcess.ArgEndProcess),
		PeerHonestyHandler:      &mock.PeerHonestyHandlerStub{},
		Watchdog:                &watchdog.DisabledWatchdog{},
		FallbackHeaderValidator: &mock.FallBackHeaderValidatorStub{},
		NodeRedundancyHandler:   &consensusMock.NodeRedundancyHandlerStub{},
		Indexer:                 &consensusMock.IndexerMock{}, //disabled
		ProposalController:      proposalController,
		FeeHandler:              freeFeeHandlerMock(),
	}

	pn.ForkController = forkControllerStub

	//Add keys
	pn.PrivKey = blockSignKeyPair.Sk
	pn.PubKey = blockSignKeyPair.Pk
	pn.NodeBlockSignKeyPair = blockSignKeyPair
	pn.NodeTxSignKeyPair = txSignKeyPair

	pn.NodeAccount, err = CreateNodeAccountWithExistingKeys(pn.NodeBlockSignKeyPair.Sk, pn.NodeBlockSignKeyPair.Pk, singleSigner, blockSingleSigner)
	if err != nil {
		return nil, err
	}

	hasher := &blake2b.Blake2b{HashSize: multisig.BlsHashSize}
	llsig := &llsig.BlsMultiSigner{Hasher: hasher}

	pn.MultiSigner, err = multisig.NewBLSMultisig(
		llsig,
		blockSignPubkeyList,
		blockSignKeyPair.Sk,
		&mock.KeyGenMock{},
		uint16(keyIndex), // #nosec G115
	)
	if err != nil {
		return nil, err
	}
	if pn.MultiSigner == nil {
		fmt.Println("Error generating multisigner")
	}

	pn.EpochStartNotifier = epochStartNotifier
	err = pn.initDataPools()
	if err != nil {
		return nil, err
	}

	err = pn.InitTestNode()
	if err != nil {
		return nil, err
	}

	return pn, nil
}

// NewTestProcessorNodeWithCustomNodesCoordinator returns a new TestProcessorNode instance with custom NodesCoordinator
func NewTestProcessorNodeWithCustomNodesCoordinator(
	epochStartNotifier notifier.EpochStartNotifier,
	nodesCoordinator sharding.NodesCoordinator,
	txSignKeyPair *NodeKeyPair,
	txSignPubkeyList []string,
	blockSignKeyPair *NodeKeyPair,
	blockSignPubkeyList []string,
	keyIndex int,
	headerSigVerifier process.InterceptedHeaderSigVerifier,
	headerIntegrityVerifier process.HeaderIntegrityVerifier,
	nodeSetup sharding.GenesisNodesSetupHandler,
	mainConfig config.Config,
) (*ProcessorNode, error) {
	singleSigner := &singlesig.Ed25519Signer{}
	blockSingleSigner := &mclsinglesig.BlsSingleSigner{}

	p2pConfig := CreateP2PConfigWithNoDiscovery()
	messenger, err := CreateMessengerFromP2P(p2pConfig)
	if err != nil {
		return nil, err
	}

	var txAccumulator node.Accumulator
	txAccumulatorConfig := mainConfig.Antiflood.TxAccumulator
	txAccumulator, err = accumulator.NewTimeAccumulator(
		time.Duration(txAccumulatorConfig.MaxAllowedTimeInMilliseconds)*time.Millisecond,
		time.Duration(txAccumulatorConfig.MaxDeviationTimeInMilliseconds)*time.Millisecond,
	)
	if err != nil {
		return nil, err
	}

	forkControllerStub := &integrationTestsMock.ForkControllerStub{
		ProcessorFlowITOPriceCalled: func() bool {
			return true
		},
		ClaimKFICalled: func() bool {
			return true
		},
		FixStakingBucketsCalled: func() bool {
			return true
		},
		KdaFprCalled: func() bool {
			return true
		},
	}

	proposalController, err := kapps.NewProposalController(forkControllerStub)
	if err != nil {
		return nil, err
	}

	syncer := ntp.NewSyncTime(mainConfig.NTP, nil)
	syncer.StartSyncingTime()

	time.Sleep(1000 * time.Millisecond)
	ntpTime := syncer.CurrentTime()

	pn := &ProcessorNode{
		GenesisTime:              ntpTime,
		AddressPubkeyConverter:   TestAddressPubkeyConverter,
		ValidatorPubkeyConverter: TestValidatorPubkeyConverter,
		PubkeyTxSignList:         txSignPubkeyList,
		PubkeyBlockSignList:      blockSignPubkeyList,
		MainConfig:               mainConfig,
		Hasher:                   getHasher(),
		TxSignHasher:             getHasher(),
		TxSignMarshalizer:        getMarshalizer(),
		InternalMarshalizer:      getMarshalizer(),
		SingleSigner:             &singlesig.Ed25519Signer{},
		TxSingleSigner:           &singlesig.Ed25519Signer{},
		Uint64ByteSliceConverter: uint64ByteSlice.NewBigEndianConverter(),
		PeerSigHandler:           &mock.PeerSignatureHandler{Signer: &singlesig.Ed25519Signer{}},
		BootStorer: &mock.BoostrapStorerMock{
			PutCalled: func(slot int64, bootData *bootstrapStorage.BootstrapData) error {
				return nil
			},
		},
		Messenger:               messenger,
		NodesCoordinator:        nodesCoordinator,
		HeaderSigVerifier:       headerSigVerifier,
		HeaderIntegrityVerifier: headerIntegrityVerifier,
		ChainID:                 ChainID,
		NodesSetup:              nodeSetup,
		ConsensusGroupSize:      len(blockSignPubkeyList), // check
		SyncTimer:               syncer,
		AppStatusHandler: &mock.AppStatusHandlerStub{
			SetUInt64ValueHandler: func(key string, value uint64) {

			},
		},
		PeerDenialEvaluator:     &networkMock.PeerDenialEvaluatorStub{},
		ValidatorsProvider:      &heartbeatMock.ValidatorsProviderStub{},
		InputAntifloodHandler:   disabled.NewAntiFloodHandler(),
		TxAccumulator:           txAccumulator,
		MinTransactionVersion:   MinTransactionVersion,
		EpochNotifier:           notifier.NewGenericEpochNotifier(),
		ChanStopNodeProcess:     make(chan endProcess.ArgEndProcess),
		PeerHonestyHandler:      &mock.PeerHonestyHandlerStub{},
		Watchdog:                &watchdog.DisabledWatchdog{},
		FallbackHeaderValidator: &mock.FallBackHeaderValidatorStub{},
		NodeRedundancyHandler:   &consensusMock.NodeRedundancyHandlerStub{},
		Indexer:                 &consensusMock.IndexerMock{}, //disabled
		ProposalController:      proposalController,
		FeeHandler:              freeFeeHandlerMock(),
	}

	pn.ForkController = forkControllerStub

	//Add keys
	pn.PrivKey = blockSignKeyPair.Sk
	pn.PubKey = blockSignKeyPair.Pk
	pn.NodeBlockSignKeyPair = blockSignKeyPair
	pn.NodeTxSignKeyPair = txSignKeyPair

	pn.NodeAccount, err = CreateNodeAccountWithExistingKeys(pn.NodeBlockSignKeyPair.Sk, pn.NodeBlockSignKeyPair.Pk, singleSigner, blockSingleSigner)
	if err != nil {
		return nil, err
	}

	hasher := &cryptoMock.HasherSpongeMock{}
	llsig := &llsig.BlsMultiSigner{Hasher: hasher}

	suite := mcl.NewSuiteBLS12()
	keyGen := signing.NewKeyGenerator(suite)

	pn.MultiSigner, err = multisig.NewBLSMultisig(
		llsig,
		blockSignPubkeyList,
		blockSignKeyPair.Sk,
		keyGen,
		uint16(keyIndex), // #nosec G115
	)
	if err != nil {
		return nil, err
	}
	if pn.MultiSigner == nil {
		fmt.Println("Error generating multisigner")
	}

	// logs processor
	pn.LogProcessor, err = transactionLog.NewTxLogProcessor(transactionLog.ArgTxLogProcessor{
		Marshalizer:          pn.InternalMarshalizer,
		SaveInStorageEnabled: false,
	})
	if err != nil {
		return nil, err
	}

	pn.EpochStartNotifier = epochStartNotifier
	err = pn.initDataPools()
	if err != nil {
		return nil, err
	}

	err = pn.InitTestNode()
	if err != nil {
		return nil, err
	}

	return pn, nil
}
