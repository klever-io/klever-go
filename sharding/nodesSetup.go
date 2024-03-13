package sharding

import (
	"fmt"

	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/tools"
	"github.com/klever-io/klever-go/tools/check"
)

const defaultInitialRating = uint32(5000001)

// InitialNode holds data from json
type InitialNode struct {
	PubKey        string `json:"pubkey"`
	Address       string `json:"address"`
	InitialRating uint32 `json:"initialRating"`
	NodeInfo
}

// NodeInfo holds node info
type NodeInfo struct {
	elected       bool
	pubKey        []byte
	address       []byte
	initialRating uint32
}

// AddressBytes gets the node address as bytes
func (ni *NodeInfo) AddressBytes() []byte {
	return ni.address
}

// PubKeyBytes gets the node public key as bytes
func (ni *NodeInfo) PubKeyBytes() []byte {
	return ni.pubKey
}

// GetInitialRating gets the initial rating for a node
func (ni *NodeInfo) GetInitialRating() uint32 {
	return ni.initialRating
}

// IsInterfaceNil returns true if underlying object is nil
func (ni *NodeInfo) IsInterfaceNil() bool {
	return ni == nil
}

// NodesSetup hold data for decoded data from json file
type NodesSetup struct {
	StartTime             int64  `json:"startTime"`
	SlotInterval          uint64 `json:"slotInterval"`
	SlotsPerEpoch         uint64 `json:"slotsPerEpoch"`
	ConsensusGroupSize    uint32 `json:"consensusGroupSize"`
	MinNodes              uint32 `json:"minNodes"`
	ChainID               string `json:"chainID"`
	MinTransactionVersion uint32 `json:"minTransactionVersion"`
	KlvDenomination       uint32 `json:"klvDenomination"`

	InitialNodes []*InitialNode `json:"initialNodes"`

	nrOfNodes                uint32
	elected                  []GenesisNodeInfoHandler
	eligible                 []GenesisNodeInfoHandler
	validatorPubkeyConverter core.PubkeyConverter
	addressPubkeyConverter   core.PubkeyConverter
}

// NewNodesSetup creates a new decoded nodes structure from json config file
func NewNodesSetup(
	nodesFilePath string,
	addressPubkeyConverter core.PubkeyConverter,
	validatorPubkeyConverter core.PubkeyConverter,
) (*NodesSetup, error) {

	if check.IfNil(addressPubkeyConverter) {
		return nil, fmt.Errorf("%w for addressPubkeyConverter", ErrNilPubkeyConverter)
	}
	if check.IfNil(validatorPubkeyConverter) {
		return nil, fmt.Errorf("%w for validatorPubkeyConverter", ErrNilPubkeyConverter)
	}

	nodes := &NodesSetup{
		addressPubkeyConverter:   addressPubkeyConverter,
		validatorPubkeyConverter: validatorPubkeyConverter,
	}

	err := tools.LoadJSONFile(nodes, nodesFilePath)
	if err != nil {
		return nil, err
	}

	err = nodes.processConfig()
	if err != nil {
		return nil, err
	}

	nodes.processChainAssignment()
	nodes.createInitialNodesInfo()

	//TODO: delete this log before merging:
	log.Debug("nodes setup",
		"start time", nodes.StartTime,
		"chain id", nodes.ChainID,
		"slot duration", nodes.SlotInterval,
		"min tx version", nodes.MinTransactionVersion)
	return nodes, nil
}

func (ns *NodesSetup) processConfig() error {
	var err error

	ns.nrOfNodes = 0
	for i := 0; i < len(ns.InitialNodes); i++ {
		pubKey := ns.InitialNodes[i].PubKey
		ns.InitialNodes[i].pubKey, err = ns.validatorPubkeyConverter.Decode(pubKey)
		if err != nil {
			return fmt.Errorf("%w, %s for string %s", ErrCouldNotParsePubKey, err.Error(), pubKey)
		}

		address := ns.InitialNodes[i].Address
		ns.InitialNodes[i].address, err = ns.addressPubkeyConverter.Decode(address)
		if err != nil {
			return fmt.Errorf("%w, %s for string %s", ErrCouldNotParseAddress, err.Error(), address)
		}

		// decoder treats empty string as correct, it is not allowed to have empty string as public key
		if ns.InitialNodes[i].PubKey == "" {
			ns.InitialNodes[i].pubKey = nil
			return ErrCouldNotParsePubKey
		}

		// decoder treats empty string as correct, it is not allowed to have empty string as address
		if ns.InitialNodes[i].Address == "" {
			ns.InitialNodes[i].address = nil
			return ErrCouldNotParseAddress
		}

		initialRating := ns.InitialNodes[i].InitialRating
		if initialRating == uint32(0) {
			initialRating = defaultInitialRating
		}
		ns.InitialNodes[i].initialRating = initialRating

		ns.nrOfNodes++
	}

	if ns.ConsensusGroupSize < 1 {
		return ErrNegativeOrZeroConsensusGroupSize
	}
	if ns.MinNodes < ns.ConsensusGroupSize {
		return ErrMinNodesSmallerThanConsensusSize
	}
	if ns.nrOfNodes < ns.MinNodes {
		return ErrNodesSizeSmallerThanMinNoOfNodes
	}

	return nil
}

func (ns *NodesSetup) createInitialNodesInfo() {
	ns.elected = make([]GenesisNodeInfoHandler, 0)
	ns.eligible = make([]GenesisNodeInfoHandler, 0)
	for _, in := range ns.InitialNodes {
		if in.pubKey != nil && in.address != nil {
			nodeInfo := &NodeInfo{
				elected:       in.elected,
				pubKey:        in.pubKey,
				address:       in.address,
				initialRating: in.initialRating,
			}
			if in.elected {
				ns.elected = append(ns.elected, nodeInfo)
			} else {
				ns.eligible = append(ns.eligible, nodeInfo)
			}
		}
	}
}

// InitialNodesPubKeys - gets initial nodes public keys
func (ns *NodesSetup) InitialNodesPubKeys() []string {
	pubKeys := make([]string, len(ns.eligible))
	for i := 0; i < len(ns.eligible); i++ {
		pubKeys[i] = string(ns.eligible[i].PubKeyBytes())
	}

	return pubKeys
}

// AllInitialNodes returns all initial nodes loaded
func (ns *NodesSetup) AllInitialNodes() []GenesisNodeInfoHandler {
	list := make([]GenesisNodeInfoHandler, len(ns.InitialNodes))
	for idx, initialNode := range ns.InitialNodes {
		list[idx] = initialNode
	}

	return list
}

// InitialElectedNodesPubKeys - gets initial nodes public keys
func (ns *NodesSetup) InitialElectedNodesPubKeys() ([]string, error) {
	if ns.elected == nil {
		return nil, ErrNilElected
	}
	if len(ns.elected) == 0 {
		return nil, ErrNoPubKeys
	}

	pubKeys := make([]string, len(ns.elected))
	for i := 0; i < len(ns.elected); i++ {
		pubKeys[i] = string(ns.elected[i].PubKeyBytes())
	}

	return pubKeys, nil
}

// InitialNodesInfo - gets initial nodes info for shard
func (ns *NodesSetup) InitialNodesInfo() ([]GenesisNodeInfoHandler, []GenesisNodeInfoHandler, error) {
	if len(ns.elected) == 0 {
		return nil, nil, ErrNilElected
	}

	return ns.elected, ns.eligible, nil
}

// MinNumberOfNodes returns the minimum number of nodes
func (ns *NodesSetup) MinNumberOfNodes() uint32 {
	return ns.MinNodes
}

// MinNumberOfShardNodes returns the minimum number of nodes per shard
func (ns *NodesSetup) MinNumberOfShardNodes() uint32 {
	return ns.MinNodes
}

// GetStartTime returns the start time
func (ns *NodesSetup) GetStartTime() int64 {
	return ns.StartTime
}

// GetSlotInterval returns the slot duration
func (ns *NodesSetup) GetSlotInterval() uint64 {
	return ns.SlotInterval
}

// GetSlotsPerEpoch returns the slot per epoch
func (ns *NodesSetup) GetSlotsPerEpoch() uint64 {
	return ns.SlotsPerEpoch
}

// GetChainID returns the chain ID
func (ns *NodesSetup) GetChainID() string {
	return ns.ChainID
}

// GetMinTransactionVersion returns the minimum transaction version
func (ns *NodesSetup) GetMinTransactionVersion() uint32 {
	return ns.MinTransactionVersion
}

// GetConsensusGroupSize returns the shard consensus group size
func (ns *NodesSetup) GetConsensusGroupSize() uint32 {
	return ns.ConsensusGroupSize
}

func (ns *NodesSetup) processChainAssignment() {
	ns.nrOfNodes = 0
	for id := uint32(0); id < ns.MinNodes; id++ {
		if ns.InitialNodes[id].pubKey != nil {
			ns.InitialNodes[id].elected = true
			ns.nrOfNodes++
		}
	}
}

// IsInterfaceNil returns true if underlying object is nil
func (ns *NodesSetup) IsInterfaceNil() bool {
	return ns == nil
}
