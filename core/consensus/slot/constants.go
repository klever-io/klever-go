package slot

//TODO consider moving these constants in config file

// MaxThresholdPercent specifies the max allocated time percent for doing Job as a percentage of the total time of one slot
const MaxThresholdPercent = 90 //

// LeaderPeerHonestyIncreaseFactor specifies the factor with which the honesty of the leader should be increased
// if it proposed a block or sent the final info, in its correct allocated slot/time-frame/slot
const LeaderPeerHonestyIncreaseFactor = 2

// ValidatorPeerHonestyIncreaseFactor specifies the factor with which the honesty of the validator should be increased
// if it sent the signature, in its correct allocated slot/time-frame/slot
const ValidatorPeerHonestyIncreaseFactor = 1

// LeaderPeerHonestyDecreaseFactor specifies the factor with which the honesty of the leader should be decreased
// if it proposed a block or sent the final info, in an incorrect allocated slot/time-frame/slot
const LeaderPeerHonestyDecreaseFactor = -4

// ValidatorPeerHonestyDecreaseFactor specifies the factor with which the honesty of the validator should be decreased
// if it sent the signature, in an incorrect allocated slot/time-frame/slot
const ValidatorPeerHonestyDecreaseFactor = -2

// MaxNumOfMessageTypeAccepted represents the maximum number of the same message type accepted in one slot to be
// received from the same public key
const MaxNumOfMessageTypeAccepted = 1
