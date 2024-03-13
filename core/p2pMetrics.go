package core

// MetricP2PPeerNumReceivedMessages represents the current maximum number of received messages in the amount of time
// counted on a connected peer
const MetricP2PPeerNumReceivedMessages = "klv_p2p_peer_num_received_messages"

// MetricP2PPeerSizeReceivedMessages represents the current maximum size of received data (sum of all messages) in
// the amount of time counted on a connected peer
const MetricP2PPeerSizeReceivedMessages = "klv_p2p_peer_size_received_messages"

// MetricP2PPeerNumProcessedMessages represents the current maximum number of processed messages in the amount of time
// counted on a connected peer
const MetricP2PPeerNumProcessedMessages = "klv_p2p_peer_num_processed_messages"

// MetricP2PPeerSizeProcessedMessages represents the current maximum size of processed data (sum of all messages) in
// the amount of time counted on a connected peer
const MetricP2PPeerSizeProcessedMessages = "klv_p2p_peer_size_processed_messages"

// MetricP2PPeakPeerNumReceivedMessages represents the peak maximum number of received messages in the amount of time
// counted on a connected peer
const MetricP2PPeakPeerNumReceivedMessages = "klv_p2p_peak_peer_num_received_messages"

// MetricP2PPeakPeerSizeReceivedMessages represents the peak maximum size of received data (sum of all messages) in
// the amount of time counted on a connected peer
const MetricP2PPeakPeerSizeReceivedMessages = "klv_p2p_peak_peer_size_received_messages"

// MetricP2PPeakPeerNumProcessedMessages represents the peak maximum number of processed messages in the amount of time
// counted on a connected peer
const MetricP2PPeakPeerNumProcessedMessages = "klv_p2p_peak_peer_num_processed_messages"

// MetricP2PPeakPeerSizeProcessedMessages represents the peak maximum size of processed data (sum of all messages) in
// the amount of time counted on a connected peer
const MetricP2PPeakPeerSizeProcessedMessages = "klv_p2p_peak_peer_size_processed_messages"

// MetricP2PNumReceiverPeers represents the number of connected peer sent messages to the current peer (and have been
// received by the current peer) in the amount of time
const MetricP2PNumReceiverPeers = "klv_p2p_num_receiver_peers"

// MetricP2PPeakNumReceiverPeers represents the peak number of connected peer sent messages to the current peer
// (and have been received by the current peer) in the amount of time
const MetricP2PPeakNumReceiverPeers = "klv_p2p_peak_num_receiver_peers"

// MetricP2PPeerInfo is the metric for the node's p2p info
const MetricP2PPeerInfo = "klv_p2p_peer_info"

// MetricP2PIntraShardValidators is the metric that outputs the intra-shard connected validators
const MetricP2PIntraShardValidators = "klv_p2p_intra_shard_validators"

// MetricP2PCrossShardValidators is the metric that outputs the cross-shard connected validators
const MetricP2PCrossShardValidators = "klv_p2p_cross_shard_validators"

// MetricP2PIntraShardObservers is the metric that outputs the intra-shard connected observers
const MetricP2PIntraShardObservers = "klv_p2p_intra_shard_observers"

// MetricP2PCrossShardObservers is the metric that outputs the cross-shard connected observers
const MetricP2PCrossShardObservers = "klv_p2p_cross_shard_observers"

// MetricP2PUnknownPeers is the metric that outputs the unknown-shard connected peers
const MetricP2PUnknownPeers = "klv_p2p_unknown_shard_peers"

// MetricNumConnectedPeersClassification is the metric for monitoring the number of connected peers split on the connection type
const MetricNumConnectedPeersClassification = "klv_num_connected_peers_classification"

// MetricP2PNumConnectedPeersClassification is the metric for monitoring the number of connected peers split on the connection type
const MetricP2PNumConnectedPeersClassification = "klv_p2p_num_connected_peers_classification"
