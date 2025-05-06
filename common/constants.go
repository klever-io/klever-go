package common

import "time"

// DefaultDirPermission represents the default directory permissions
const DefaultDirPermission = 0750

// FileModeUserReadWrite represents the permission for a file which allows the user for reading and writing
const FileModeUserReadWrite = 0600

// BaseOperationCost represents the field name for base operation costs
const BaseOperationCost = "BaseOperationCost"

// BuiltInCost represents the field name for built-in operation costs
const BuiltInCost = "BuiltInCost"

// BaseOpsAPICost represents the field name of the SC API (EEI) gas costs
const BaseOpsAPICost = "BaseOpsAPICost"

// MaxPerTransaction represents the field name of max counts per transaction in block chain hook
const MaxPerTransaction = "MaxPerTransaction"

const EmptyJSonMarshalData = "{}"

// ExtraDelayForRequestBlockInfo represents the number of seconds to wait since a block has been received and the
// moment when its components,like transactions, would be requested too if they are still missing
const ExtraDelayForRequestBlockInfo = ExtraDelayForBroadcastBlockInfo + time.Second

// ExtraDelayForBroadcastBlockInfo represents the number of seconds to wait since a block has been broadcast and the
// moment when its components,transactions, would be broadcast too
const ExtraDelayForBroadcastBlockInfo = 1 * time.Second
