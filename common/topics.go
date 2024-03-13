package common

const (
	// TransactionTopic is the topic used for sharing transactions
	TransactionTopic = "transactions"
	// ConsensusTopic is the topic used in consensus algorithm
	ConsensusTopic = "consensus"
	// HeartbeatTopic is the topic used for heartbeat signaling
	HeartbeatTopic = "heartbeat"
	// BlocksTopic is the topic used for blocks
	BlocksTopic = "txBlock"
	// AccountTrieNodesTopic is used for sharing state trie nodes
	AccountTrieNodesTopic = "accountTrieNodes"
	// ValidatorTrieNodesTopic is used for sharding validator state trie nodes
	ValidatorTrieNodesTopic = "validatorTrieNodes"
	// KappTrieNodesTopic is used for sharding kapp state trie nodes
	KappTrieNodesTopic = "kappTrieNodes"
)

// WasmVirtualMachine is a byte array identifier for the smart contract address created for Wasm VM
var WasmVirtualMachine = []byte{5, 0}
