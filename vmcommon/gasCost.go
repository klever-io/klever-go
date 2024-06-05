package vmcommon

// BaseOperationCost defines cost for base operation cost
type BaseOperationCost struct {
	StorePerByte      uint64
	ReleasePerByte    uint64
	DataCopyPerByte   uint64
	PersistPerByte    uint64
	CompilePerByte    uint64
	AoTPreparePerByte uint64
}

type BuiltInCost struct {
	Transfer                uint64
	CreateAsset             uint64
	CreateValidator         uint64
	ValidatorConfig         uint64
	Freeze                  uint64
	Unfreeze                uint64
	Delegate                uint64
	Undelegate              uint64
	Withdraw                uint64
	Claim                   uint64
	Unjail                  uint64
	AssetTrigger            uint64
	SetAccountName          uint64
	Proposal                uint64
	Vote                    uint64
	ConfigITO               uint64
	Buy                     uint64
	Sell                    uint64
	CancelMarketOrder       uint64
	CreateMarketplace       uint64
	ConfigMarketplace       uint64
	UpdateAccountPermission uint64
	Deposit                 uint64
	ITOTrigger              uint64
	ChangeOwnerAddress      uint64
}

// GasCost holds all the needed gas costs for system smart contracts
type GasCost struct {
	BaseOperationCost BaseOperationCost
	BuiltInCost       BuiltInCost
}

// SafeSubUint64 performs subtraction on uint64 and returns an error if it overflows
func SafeSubUint64(a, b uint64) (uint64, error) {
	if a < b {
		return 0, ErrSubtractionOverflow
	}
	return a - b, nil
}
