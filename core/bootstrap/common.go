package bootstrap

import (
	"fmt"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/sharding"
	"github.com/klever-io/klever-go/tools/check"
)

const baseErrorMessage = "error with epoch start bootstrapper arguments"

func checkArguments(args ArgsEpochStartBootstrap) error {
	if check.IfNil(args.PathManager) {
		return fmt.Errorf("%s: %w", baseErrorMessage, common.ErrNilPathManager)
	}
	if check.IfNil(args.Messenger) {
		return fmt.Errorf("%s: %w", baseErrorMessage, common.ErrNilMessenger)
	}
	if check.IfNil(args.PublicKey) {
		return fmt.Errorf("%s: %w", baseErrorMessage, sharding.ErrNilPubKey)
	}
	if check.IfNil(args.Hasher) {
		return fmt.Errorf("%s: %w", baseErrorMessage, common.ErrNilHasher)
	}
	if check.IfNil(args.Marshalizer) {
		return fmt.Errorf("%s: %w", baseErrorMessage, common.ErrNilMarshalizer)
	}
	if check.IfNil(args.BlockKeyGen) {
		return fmt.Errorf("%s: %w", baseErrorMessage, common.ErrNilBlockKeyGen)
	}
	if check.IfNil(args.KeyGen) {
		return fmt.Errorf("%s: %w", baseErrorMessage, common.ErrNilKeyGen)
	}
	if check.IfNil(args.SingleSigner) {
		return fmt.Errorf("%s: %w", baseErrorMessage, common.ErrNilSingleSigner)
	}
	if check.IfNil(args.BlockSingleSigner) {
		return fmt.Errorf("%s: %w", baseErrorMessage, common.ErrNilBlockSingleSigner)
	}
	if check.IfNil(args.TxSignMarshalizer) {
		return fmt.Errorf("%s: %w", baseErrorMessage, common.ErrNilTxSignMarshalizer)
	}
	if args.GenesisNodesConfig == nil {
		return fmt.Errorf("%s: %w", baseErrorMessage, common.ErrNilGenesisNodesConfig)
	}
	if len(args.DefaultDBPath) == 0 {
		return fmt.Errorf("%s: %w", baseErrorMessage, common.ErrInvalidDefaultDBPath)
	}
	if check.IfNil(args.AddressPubkeyConverter) {
		return fmt.Errorf("%s: %w", baseErrorMessage, common.ErrNilPubkeyConverter)
	}
	if len(args.DefaultEpochString) == 0 {
		return fmt.Errorf("%s: %w", baseErrorMessage, common.ErrInvalidDefaultEpochString)
	}
	if len(args.WorkingDir) == 0 {
		return fmt.Errorf("%s: %w", baseErrorMessage, common.ErrInvalidWorkingDir)
	}
	if check.IfNil(args.SlotManager) {
		return fmt.Errorf("%s: %w", baseErrorMessage, common.ErrNilSlotManager)
	}
	if check.IfNil(args.StorageUnitOpener) {
		return fmt.Errorf("%s: %w", baseErrorMessage, common.ErrNilStorageUnitOpener)
	}
	if check.IfNil(args.LatestStorageDataProvider) {
		return fmt.Errorf("%s: %w", baseErrorMessage, common.ErrNilLatestStorageDataProvider)
	}
	if check.IfNil(args.Uint64Converter) {
		return fmt.Errorf("%s: %w", baseErrorMessage, common.ErrNilUint64Converter)
	}
	if check.IfNil(args.NodeShuffler) {
		return fmt.Errorf("%s: %w", baseErrorMessage, common.ErrNilShuffler)
	}
	if check.IfNil(args.StatusHandler) {
		return fmt.Errorf("%s: %w", baseErrorMessage, common.ErrNilStatusHandler)
	}
	if check.IfNil(args.HeaderIntegrityVerifier) {
		return common.ErrNilHeaderIntegrityVerifier
	}
	if check.IfNil(args.TxSignHasher) {
		return fmt.Errorf("%s: %w", baseErrorMessage, common.ErrNilHasher)
	}
	if check.IfNil(args.EpochNotifier) {
		return fmt.Errorf("%s: %w", baseErrorMessage, common.ErrNilEpochNotifier)
	}

	return nil
}
