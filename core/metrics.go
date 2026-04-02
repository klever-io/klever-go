package core

// MetricAppVersion is the metric for the current app version
const MetricAppVersion = "klv_app_version"

// MetricTotalSupply holds the total supply value for the last epoch
const MetricTotalSupply = "klv_total_supply"

// MetricTotalStakedValue holds the total staked value
const MetricTotalStakedValue = "klv_total_staked_value"

// MetricInflation holds the inflation value for the last epoch
const MetricInflation = "klv_inflation"

// MetricDevRewards holds the developers' rewards value for the last epoch
const MetricDevRewards = "klv_dev_rewards"

// MetricKFIRewards holds the developers' rewards value for the last epoch
const MetricKFIRewards = "klv_kfi_rewards"

// MetricTotalFees holds the total fees value for the last epoch
const MetricTotalFees = "klv_total_fees"

// MetricEpochForEconomicsData holds the epoch for which economics data are computed
const MetricEpochForEconomicsData = "klv_epoch_for_economics_data"

// MetricNumMetachainNodes is the metric which holds the number of nodes in metachain
const MetricNumMetachainNodes = "klv_num_metachain_nodes"

// MetricChainID is the metric that specifies current chain id
const MetricChainID = "klv_chain_id"

// MetricSlotInterval is the metric that specifies the slot duration in milliseconds
const MetricSlotInterval = "klv_slot_duration"

// MetricStartTime is the metric that specifies the genesis start time
const MetricStartTime = "klv_start_time"

// MetricCurrentSlot is the metric for monitoring the current slot of a node
const MetricCurrentSlot = "klv_current_slot"

// MetricSlotTime is the metric for slot time in seconds
const MetricSlotTime = "klv_slot_time"

// MetricSlotAtEpochStart is the metric for storing the first slot of the current epoch
const MetricSlotAtEpochStart = "klv_slot_at_epoch_start"

// MetricNonceAtEpochStart is the metric for storing the first nonce of the current epoch
const MetricNonceAtEpochStart = "klv_nonce_at_epoch_start"

// MetricSlotsPerEpoch is the metric that tells the number of slots in an epoch
const MetricSlotsPerEpoch = "klv_slots_per_epoch"

// MetricSlotsPassedInCurrentEpoch is the metric that tells the number of slots passed in current epoch
const MetricSlotsPassedInCurrentEpoch = "klv_slots_passed_in_current_epoch" // #nosec G101: false positive

// MetricNoncesPassedInCurrentEpoch is the metric that tells the number of nonces passed in current epoch
const MetricNoncesPassedInCurrentEpoch = "klv_nonces_passed_in_current_epoch" // #nosec G101: false positive

// MetricNonce is the metric for monitoring the nonce of a node
const MetricNonce = "klv_nonce"

// MetricNonceForTPS is the metric for monitoring the nonce of a node used in TPS benchmarks
const MetricNonceForTPS = "klv_nonce_for_tps"

// MetricEpochNumber is the metric for the number of epoch
const MetricEpochNumber = "klv_epoch_number"

// MetricHighestFinalBlock is the metric for the nonce of the highest final block
const MetricHighestFinalBlock = "klv_highest_final_nonce"

// MetricSynchronizedSlot is the metric for monitoring the synchronized slot of a node
const MetricSynchronizedSlot = "klv_synchronized_slot"

// MetricNodeDisplayName is the metric that stores the name of the node
const MetricNodeDisplayName = "klv_node_display_name"

// MetricNumTxInBlock is the metric for the number of transactions in the proposed block
const MetricNumTxInBlock = "klv_num_tx_block"

// MetricBaseTxSize is the metric that specifies the base size for transactions
const MetricBaseTxSize = "klv_base_tx_size"

// MetricConsensusState is the metric for consensus state of node proposer,participant or not consensus group
const MetricConsensusState = "klv_consensus_state"

// MetricConsensusSlotState is the metric for consensus slot state for a block
const MetricConsensusSlotState = "klv_consensus_slot_state"

// MetricCurrentBlockHash is the metric that stores the current block hash
const MetricCurrentBlockHash = "klv_current_block_hash"

// MetricCurrentSlotTimestamp is the metric that stores current slot timestamp
const MetricCurrentSlotTimestamp = "klv_current_slot_timestamp"

// MetricAverageTPS is the metric that stores the average tps
const MetricAverageTPS = "klv_average_tps"

// MetricHeaderSize is the metric that stores the current block size
const MetricHeaderSize = "klv_current_header_block_size"

// MetricBodyBlocksSize is the metric that stores the current block size
const MetricBodyBlocksSize = "klv_body_blocks_size"

// MetricTXsBlocksSize is the metric that stores the current block txs size
const MetricTXsBlocksSize = "klv_txs_blocks_size"

// MetricCPULoadPercent is the metric for monitoring CPU load [%]
const MetricCPULoadPercent = "klv_cpu_load_percent"

// MetricMemLoadPercent is the metric for monitoring memory load [%]
const MetricMemLoadPercent = "klv_mem_load_percent"

// MetricMemTotal is the metric for monitoring total memory bytes
const MetricMemTotal = "klv_mem_total"

// MetricMemUsedGolang is a metric for monitoring the memory ("total")
const MetricMemUsedGolang = "klv_mem_used_golang"

// MetricMemUsedSystem is a metric for monitoring the memory ("sys mem")
const MetricMemUsedSystem = "klv_mem_used_sys"

// MetricMemHeapInUse is a metric for monitoring the memory ("heap in use")
const MetricMemHeapInUse = "klv_mem_heap_inuse"

// MetricMemStackInUse is a metric for monitoring the memory ("stack in use")
const MetricMemStackInUse = "klv_mem_stack_inuse"

// MetricNetworkRecvPercent is the metric for monitoring network receive load [%]
const MetricNetworkRecvPercent = "klv_network_recv_percent"

// MetricNetworkRecvBps is the metric for monitoring network received bytes per second
const MetricNetworkRecvBps = "klv_network_recv_bps"

// MetricNetworkRecvBpsPeak is the metric for monitoring network received peak bytes per second
const MetricNetworkRecvBpsPeak = "klv_network_recv_bps_peak"

// MetricNetworkRecvBytesInCurrentEpochPerHost is the metric for monitoring network received bytes in current epoch per host
const MetricNetworkRecvBytesInCurrentEpochPerHost = "klv_network_recv_bytes_in_epoch_per_host"

// MetricNetworkSendBytesInCurrentEpochPerHost is the metric for monitoring network send bytes in current epoch per host
const MetricNetworkSendBytesInCurrentEpochPerHost = "klv_network_sent_bytes_in_epoch_per_host"

// MetricNetworkSentPercent is the metric for monitoring network sent load [%]
const MetricNetworkSentPercent = "klv_network_sent_percent"

// MetricNetworkSentBps is the metric for monitoring network sent bytes per second
const MetricNetworkSentBps = "klv_network_sent_bps"

// MetricNetworkSentBpsPeak is the metric for monitoring network sent peak bytes per second
const MetricNetworkSentBpsPeak = "klv_network_sent_bps_peak"

// MetricPublicKeyBlockSign is the metric for monitoring public key of a node used in block signing
const MetricPublicKeyBlockSign = "klv_public_key_block_sign"

// MetricCountConsensus is the metric for monitoring number of slots when a node was in consensus group
const MetricCountConsensus = "klv_count_consensus"

// MetricNodeType is the metric for monitoring the type of the node
const MetricNodeType = "klv_node_type"

// MetricPeerType is the metric which tells the peer's type (in eligible list, in waiting list, or observer)
const MetricPeerType = "klv_peer_type"

// MetricNumValidators is the metric for the number of validators
const MetricNumValidators = "klv_num_validators"

// MetricCountConsensusAcceptedBlocks is the metric for monitoring number of blocks accepted when the node was in consensus group
const MetricCountConsensusAcceptedBlocks = "klv_count_consensus_accepted_blocks"

// MetricCountLeader is the metric for monitoring number of slots when a node was leader
const MetricCountLeader = "klv_count_leader"

// MetricCountAcceptedBlocks is the metric for monitoring number of blocks that was accepted proposed by a node
const MetricCountAcceptedBlocks = "klv_count_accepted_blocks"

// MetricLatestTagSoftwareVersion is the metric that stores the latest tag software version
const MetricLatestTagSoftwareVersion = "klv_latest_tag_software_version"

// MetricProbableHighestNonce is the metric for monitoring the max speculative nonce received by the node by listening on the network
const MetricProbableHighestNonce = "klv_probable_highest_nonce"

// MetricRewardsValue is the metric that stores rewards value
const MetricRewardsValue = "klv_rewards_value"

// MetricTxPoolLoad is the metric for monitoring number of transactions from pool of a node
const MetricTxPoolLoad = "klv_tx_pool_load"

// MetricIsSyncing is the metric for monitoring if a node is syncing
const MetricIsSyncing = "klv_is_syncing"

// MetricLiveValidatorNodes is the metric for monitoring live validators on the network
const MetricLiveValidatorNodes = "klv_live_validator_nodes"

// MetricConnectedNodes is the metric for monitoring total connected nodes on the network
const MetricConnectedNodes = "klv_connected_nodes"

// MetricNumConnectedPeers is the metric for monitoring the number of connected peers
const MetricNumConnectedPeers = "klv_num_connected_peers"

// MetricNumProcessedTxs is the metric that stores the number of transactions processed
const MetricNumProcessedTxs = "klv_num_transactions_processed"

// MetricPeakTPS holds the peak transactions per second
const MetricPeakTPS = "klv_peak_tps"

// MetricNumProcessedTxsTPSBenchmark is the metric that stores the number of transactions processed for tps benchmark
const MetricNumProcessedTxsTPSBenchmark = "klv_num_transactions_processed_tps_benchmark"

// MetricAverageBlockTxCount holds the average count of transactions in a block
const MetricAverageBlockTxCount = "klv_average_block_tx_count"

// MetricCurrentBlockTxCount holds the number of transactions in the current block
const MetricCurrentBlockTxCount = "klv_current_block_tx_count"

// MetricConsensusGroupSize is the metric for consensus group size for the current
const MetricConsensusGroupSize = "klv_consensus_group_size"

// MetricMinTransactionVersion is the metric that specifies the minimum transaction version
const MetricMinTransactionVersion = "klv_min_transaction_version"

// MetricNumNodes is the metric which holds the number of nodes
const MetricNumNodes = "klv_num_nodes"

// MetricNumShardHeadersProcessed is the metric that stores number of shard header processed
const MetricNumShardHeadersProcessed = "klv_num_shard_headers_processed"

// MetricNumTimesInForkChoice is the metric that counts how many time a node was in fork choice
const MetricNumTimesInForkChoice = "klv_fork_choice_count"

// HighestSlotFromBootStorage is the key for the highest slot that is saved in storage
const HighestSlotFromBootStorage = "highestSlotFromBootStorage"

// MetricReceivedProposedBlock is the metric that specify the moment in the slot when the received block has reached the
// current node. The value is provided in percent (0 meaning it has been received just after the slot started and
// 100 meaning that the block has been received in the last moment of the slot)
const MetricReceivedProposedBlock = "klv_consensus_received_proposed_block"

// MetricCreatedProposedBlock is the metric that specify the percent of the block subslot used for header and body
// creation (0 meaning that the block was created in no-time and 100 meaning that the block creation used all the
// subslot spare duration)
const MetricCreatedProposedBlock = "klv_consensus_created_proposed_block"

// MetricProcessedProposedBlock is the metric that specify the percent of the block subslot used for header and body
// processing (0 meaning that the block was processed in no-time and 100 meaning that the block processing used all the
// subslot spare duration)
const MetricProcessedProposedBlock = "klv_consensus_processed_proposed_block"

// MetricRedundancyLevel is the metric that specifies the redundancy level of the current node
const MetricRedundancyLevel = "klv_redundancy_level" // #nosec G101: false positive

// MetricRedundancyIsMainActive is the metric that specifies data about the redundancy main machine
const MetricRedundancyIsMainActive = "klv_redundancy_is_main_active"

// MetricBlockProcessDuration is the metric for the duration in milliseconds of ProcessBlock()
const MetricBlockProcessDuration = "klv_block_process_duration_ms"

// MetricBlockCommitDuration is the metric for the duration in milliseconds of CommitBlock()
const MetricBlockCommitDuration = "klv_block_commit_duration_ms"

// MetricTxProcessingDuration is the metric for the duration in milliseconds of transaction processing
const MetricTxProcessingDuration = "klv_tx_processing_duration_ms"

// MetricSystemCPUPercent is the metric for total system-wide CPU usage percent (all processes)
const MetricSystemCPUPercent = "klv_system_cpu_percent"

// MetricDiskTotalBytes is the metric for total disk space in bytes
const MetricDiskTotalBytes = "klv_disk_total_bytes"

// MetricDiskAvailableBytes is the metric for available disk space in bytes
const MetricDiskAvailableBytes = "klv_disk_available_bytes"

// MetricDiskUsagePercent is the metric for disk usage percentage
const MetricDiskUsagePercent = "klv_disk_usage_percent"

// MetricDbSizeBytes is the metric for total database directory size in bytes
const MetricDbSizeBytes = "klv_db_size_bytes"

// MetricNodeUptimeSeconds is the metric for node uptime in seconds since start
const MetricNodeUptimeSeconds = "klv_node_uptime_seconds"

// MetricNodeStartTimestamp is the metric for node start unix timestamp
const MetricNodeStartTimestamp = "klv_node_start_timestamp"

// MetricRedundancySlotsInactive is the metric for the redundancy slots of inactivity counter
const MetricRedundancySlotsInactive = "klv_redundancy_slots_inactive"
