package bootstrap

import (
	"fmt"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/sharding"
	"github.com/klever-io/klever-go/tools/check"
)

const baseErrorMessage = "error with epoch start bootstrapper arguments: %w"

func checkArguments(args ArgsEpochStartBootstrap) error {
	if check.IfNil(args.PathManager) {
		return fmt.Errorf(baseErrorMessage, common.ErrNilPathManager)
	}
	if check.IfNil(args.Messenger) {
		return fmt.Errorf(baseErrorMessage, common.ErrNilMessenger)
	}
	if check.IfNil(args.PublicKey) {
		return fmt.Errorf(baseErrorMessage, sharding.ErrNilPubKey)
	}
	if check.IfNil(args.Hasher) {
		return fmt.Errorf(baseErrorMessage, common.ErrNilHasher)
	}
	if check.IfNil(args.Marshalizer) {
		return fmt.Errorf(baseErrorMessage, common.ErrNilMarshalizer)
	}
	if check.IfNil(args.BlockKeyGen) {
		return fmt.Errorf(baseErrorMessage, common.ErrNilBlockKeyGen)
	}
	if check.IfNil(args.KeyGen) {
		return fmt.Errorf(baseErrorMessage, common.ErrNilKeyGen)
	}
	if check.IfNil(args.SingleSigner) {
		return fmt.Errorf(baseErrorMessage, common.ErrNilSingleSigner)
	}
	if check.IfNil(args.BlockSingleSigner) {
		return fmt.Errorf(baseErrorMessage, common.ErrNilBlockSingleSigner)
	}
	if check.IfNil(args.TxSignMarshalizer) {
		return fmt.Errorf(baseErrorMessage, common.ErrNilTxSignMarshalizer)
	}
	if args.GenesisNodesConfig == nil {
		return fmt.Errorf(baseErrorMessage, common.ErrNilGenesisNodesConfig)
	}
	if len(args.DefaultDBPath) == 0 {
		return fmt.Errorf(baseErrorMessage, common.ErrInvalidDefaultDBPath)
	}
	if check.IfNil(args.AddressPubkeyConverter) {
		return fmt.Errorf(baseErrorMessage, common.ErrNilPubkeyConverter)
	}
	if len(args.DefaultEpochString) == 0 {
		return fmt.Errorf(baseErrorMessage, common.ErrInvalidDefaultEpochString)
	}
	if len(args.WorkingDir) == 0 {
		return fmt.Errorf(baseErrorMessage, common.ErrInvalidWorkingDir)
	}
	if check.IfNil(args.SlotManager) {
		return fmt.Errorf(baseErrorMessage, common.ErrNilSlotManager)
	}
	if check.IfNil(args.StorageUnitOpener) {
		return fmt.Errorf(baseErrorMessage, common.ErrNilStorageUnitOpener)
	}
	if check.IfNil(args.LatestStorageDataProvider) {
		return fmt.Errorf(baseErrorMessage, common.ErrNilLatestStorageDataProvider)
	}
	if check.IfNil(args.Uint64Converter) {
		return fmt.Errorf(baseErrorMessage, common.ErrNilUint64Converter)
	}
	if check.IfNil(args.NodeShuffler) {
		return fmt.Errorf(baseErrorMessage, common.ErrNilShuffler)
	}
	if check.IfNil(args.StatusHandler) {
		return fmt.Errorf(baseErrorMessage, common.ErrNilStatusHandler)
	}
	if check.IfNil(args.HeaderIntegrityVerifier) {
		return common.ErrNilHeaderIntegrityVerifier
	}
	if check.IfNil(args.TxSignHasher) {
		return fmt.Errorf(baseErrorMessage, common.ErrNilHasher)
	}
	if check.IfNil(args.EpochNotifier) {
		return fmt.Errorf(baseErrorMessage, common.ErrNilEpochNotifier)
	}
	if check.IfNil(args.ForkController) {
		return fmt.Errorf(baseErrorMessage, common.ErrNilForkController)
	}

	return nil
}
