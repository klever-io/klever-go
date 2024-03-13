package common

// Locker defines the operations used to lock different critical areas. Implemented by the RWMutex.
type Locker interface {
	Lock()
	Unlock()
	RLock()
	RUnlock()
}

// EnableEpochsHandler is used to verify the which flags are set in the current epoch based on EnableEpochs config
type EnableEpochsHandler interface {
	BlockGasAndFeesReCheckEnableEpoch() uint32
	StakingV2EnableEpoch() uint32
	ScheduledMiniBlocksEnableEpoch() uint32
	SwitchJailWaitingEnableEpoch() uint32
	BalanceWaitingListsEnableEpoch() uint32
	WaitingListFixEnableEpoch() uint32
	MultiKDATransferAsyncCallBackEnableEpoch() uint32
	FixOOGReturnCodeEnableEpoch() uint32
	RemoveNonUpdatedStorageEnableEpoch() uint32
	CreateNFTThroughExecByCallerEnableEpoch() uint32
	FixFailExecutionOnErrorEnableEpoch() uint32
	ManagedCryptoAPIEnableEpoch() uint32
	DisableExecByCallerEnableEpoch() uint32
	RefactorContextEnableEpoch() uint32
	CheckExecuteReadOnlyEnableEpoch() uint32
	StorageAPICostOptimizationEnableEpoch() uint32
	MiniBlockPartialExecutionEnableEpoch() uint32
	RefactorPeersMiniBlocksEnableEpoch() uint32
	IsSCDeployFlagEnabled() bool
	IsBuiltInFunctionsFlagEnabled() bool
	IsPenalizedTooMuchGasFlagEnabled() bool
	ResetPenalizedTooMuchGasFlag()
	IsSwitchJailWaitingFlagEnabled() bool
	IsBelowSignedThresholdFlagEnabled() bool
	IsSwitchHysteresisForMinNodesFlagEnabled() bool
	IsSwitchHysteresisForMinNodesFlagEnabledForCurrentEpoch() bool
	IsTransactionSignedWithTxHashFlagEnabled() bool
	IsMetaProtectionFlagEnabled() bool
	IsAheadOfTimeGasUsageFlagEnabled() bool
	IsRepairCallbackFlagEnabled() bool
	IsBalanceWaitingListsFlagEnabled() bool
	IsReturnDataToLastTransferFlagEnabled() bool
	IsSenderInOutTransferFlagEnabled() bool
	IsStakeFlagEnabled() bool
	IsStakingV2FlagEnabled() bool
	IsStakingV2OwnerFlagEnabled() bool
	IsStakingV2FlagEnabledForActivationEpochCompleted() bool
	IsDoubleKeyProtectionFlagEnabled() bool
	IsKDAFlagEnabled() bool
	IsKDAFlagEnabledForCurrentEpoch() bool
	IsGovernanceFlagEnabled() bool
	IsGovernanceFlagEnabledForCurrentEpoch() bool
	IsDelegationManagerFlagEnabled() bool
	IsDelegationSmartContractFlagEnabled() bool
	IsDelegationSmartContractFlagEnabledForCurrentEpoch() bool
	IsCorrectLastUnJailedFlagEnabled() bool
	IsCorrectLastUnJailedFlagEnabledForCurrentEpoch() bool
	IsUnBondTokensV2FlagEnabled() bool
	IsSaveJailedAlwaysFlagEnabled() bool
	IsReDelegateBelowMinCheckFlagEnabled() bool
	IsValidatorToDelegationFlagEnabled() bool
	IsWaitingListFixFlagEnabled() bool
	IsIncrementSCRNonceInMultiTransferFlagEnabled() bool
	IsKDAMultiTransferFlagEnabled() bool
	IsGlobalMintBurnFlagEnabled() bool
	IsKDATransferRoleFlagEnabled() bool
	IsBuiltInFunctionOnMetaFlagEnabled() bool
	IsComputeRewardCheckpointFlagEnabled() bool
	IsSCRSizeInvariantCheckFlagEnabled() bool
	IsBackwardCompSaveKeyValueFlagEnabled() bool
	IsKDANFTCreateOnMultiShardFlagEnabled() bool
	IsMetaKDASetFlagEnabled() bool
	IsAddTokensToDelegationFlagEnabled() bool
	IsMultiKDATransferFixOnCallBackFlagEnabled() bool
	IsOptimizeGasUsedInCrossMiniBlocksFlagEnabled() bool
	IsCorrectFirstQueuedFlagEnabled() bool
	IsDeleteDelegatorAfterClaimRewardsFlagEnabled() bool
	IsFixOOGReturnCodeFlagEnabled() bool
	IsRemoveNonUpdatedStorageFlagEnabled() bool
	IsOptimizeNFTStoreFlagEnabled() bool
	IsCreateNFTThroughExecByCallerFlagEnabled() bool
	IsStopDecreasingValidatorRatingWhenStuckFlagEnabled() bool
	IsFrontRunningProtectionFlagEnabled() bool
	IsPayableBySCFlagEnabled() bool
	IsCleanUpInformativeSCRsFlagEnabled() bool
	IsStorageAPICostOptimizationFlagEnabled() bool
	IsKDARegisterAndSetAllRolesFlagEnabled() bool
	IsScheduledMiniBlocksFlagEnabled() bool
	IsCorrectJailedNotUnStakedEmptyQueueFlagEnabled() bool
	IsDoNotReturnOldBlockInBlockchainHookFlagEnabled() bool
	IsAddFailedRelayedTxToInvalidMBsFlag() bool
	IsSCRSizeInvariantOnBuiltInResultFlagEnabled() bool
	IsCheckCorrectTokenIDForTransferRoleFlagEnabled() bool
	IsFailExecutionOnEveryAPIErrorFlagEnabled() bool
	IsMiniBlockPartialExecutionFlagEnabled() bool
	IsManagedCryptoAPIsFlagEnabled() bool
	IsKDAMetadataContinuousCleanupFlagEnabled() bool
	IsDisableExecByCallerFlagEnabled() bool
	IsRefactorContextFlagEnabled() bool
	IsCheckFunctionArgumentFlagEnabled() bool
	IsCheckExecuteOnReadOnlyFlagEnabled() bool
	IsFixAsyncCallbackCheckFlagEnabled() bool
	IsSaveToSystemAccountFlagEnabled() bool
	IsCheckFrozenCollectionFlagEnabled() bool
	IsSendAlwaysFlagEnabled() bool
	IsValueLengthCheckFlagEnabled() bool
	IsCheckTransferFlagEnabled() bool
	IsTransferToMetaFlagEnabled() bool
	IsKDANFTImprovementV1FlagEnabled() bool
	IsSetSenderInEeiOutputTransferFlagEnabled() bool
	IsChangeDelegationOwnerFlagEnabled() bool
	IsRefactorPeersMiniBlocksFlagEnabled() bool
	IsFixAsyncCallBackArgsListFlagEnabled() bool
	IsFixOldTokenLiquidityEnabled() bool
	IsRuntimeMemStoreLimitEnabled() bool
	IsRuntimeCodeSizeFixEnabled() bool
	IsMaxBlockchainHookCountersFlagEnabled() bool
	IsWipeSingleNFTLiquidityDecreaseEnabled() bool
	IsAlwaysSaveTokenMetaDataEnabled() bool

	IsInterfaceNil() bool
}
