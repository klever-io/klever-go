package config_test

import (
	"testing"

	"github.com/klever-io/klever-go/config"
	"github.com/stretchr/testify/assert"
)

func TestEnableEpochs_Validate(t *testing.T) {
	t.Parallel()

	assert.NoError(t, config.EnableEpochs{}.Validate(), "template placeholder")
	assert.NoError(t, config.EnableEpochs{
		FixMarketBuyOverflow: 100,
		FixAuditChangesV4:    101,
		FixAuditChangesV5:    102,
		FixAuditChangesV6:    103,
		FixAuditChangesV7:    104,
		FixAuditChangesV8:    105,
	}.Validate())

	assert.Error(t, config.EnableEpochs{
		FixMarketBuyOverflow: 100,
		FixAuditChangesV4:    100,
		FixAuditChangesV5:    102,
		FixAuditChangesV6:    103,
		FixAuditChangesV7:    104,
		FixAuditChangesV8:    105,
	}.Validate(), "same epoch leaves an empty freeze window")
	assert.Error(t, config.EnableEpochs{
		FixMarketBuyOverflow: 100,
		FixAuditChangesV4:    99,
		FixAuditChangesV5:    102,
		FixAuditChangesV6:    103,
		FixAuditChangesV7:    104,
		FixAuditChangesV8:    105,
	}.Validate(), "thaw before freeze leaves an empty freeze window")
	assert.Error(t, config.EnableEpochs{FixMarketBuyOverflow: 100}.Validate(),
		"thaw left at the placeholder leaves an empty freeze window")
}

func TestEnableEpochs_Validate_AuditChangesV5AfterV4(t *testing.T) {
	t.Parallel()

	// V4 still at the placeholder is a template rather than a real schedule, so the
	// ordering is not enforced against it.
	assert.NoError(t, config.EnableEpochs{}.Validate(), "template placeholder")

	assert.Error(t, config.EnableEpochs{
		FixMarketBuyOverflow: 100,
		FixAuditChangesV4:    101,
		FixAuditChangesV5:    101,
		FixAuditChangesV6:    102,
		FixAuditChangesV7:    103,
		FixAuditChangesV8:    104,
	}.Validate(), "sharing V4's epoch applies the V5 changes retroactively")
	assert.Error(t, config.EnableEpochs{
		FixMarketBuyOverflow: 100,
		FixAuditChangesV4:    101,
		FixAuditChangesV5:    100,
		FixAuditChangesV6:    102,
		FixAuditChangesV7:    103,
		FixAuditChangesV8:    104,
	}.Validate(), "V5 before V4 applies the V5 changes retroactively")
	assert.Error(t, config.EnableEpochs{
		FixMarketBuyOverflow: 100,
		FixAuditChangesV4:    101,
		FixAuditChangesV6:    102,
		FixAuditChangesV7:    103,
		FixAuditChangesV8:    104,
	}.Validate(), "V5 left at the placeholder activates from genesis")
}

func TestEnableEpochs_Validate_SmartContractsNotBeforeProcessorFlowITOPrice(t *testing.T) {
	t.Parallel()

	assert.NoError(t, config.EnableEpochs{ProcessorFlowITOPrice: 165, SmartContracts: 4860}.Validate(), "mainnet schedule")
	assert.NoError(t, config.EnableEpochs{ProcessorFlowITOPrice: 100, SmartContracts: 100}.Validate(),
		"the cache is on from the first block of a shared epoch")

	assert.Error(t, config.EnableEpochs{ProcessorFlowITOPrice: 101, SmartContracts: 100}.Validate(),
		"SFT transfers before the accounts cache are never persisted")
	assert.Error(t, config.EnableEpochs{ProcessorFlowITOPrice: 100}.Validate(),
		"smart contracts left at the placeholder activate from genesis, before the cache")
}

func TestEnableEpochs_Validate_AuditChangesV6NotBeforeBigBucketsCompute(t *testing.T) {
	t.Parallel()

	assert.NoError(t, config.EnableEpochs{BigBucketsCompute: 1196, FixAuditChangesV6: 6200, FixAuditChangesV7: 6201, FixAuditChangesV8: 6202}.Validate(), "mainnet schedule")
	assert.NoError(t, config.EnableEpochs{BigBucketsCompute: 100, FixAuditChangesV6: 100, FixAuditChangesV7: 101, FixAuditChangesV8: 102}.Validate(),
		"V6 claims use the checked arithmetic from the first block of a shared epoch")

	assert.Error(t, config.EnableEpochs{BigBucketsCompute: 101, FixAuditChangesV6: 100}.Validate(),
		"V6 before bigBucketsCompute runs V6 claims on the legacy APR arithmetic")
	assert.Error(t, config.EnableEpochs{BigBucketsCompute: 1196}.Validate(),
		"V6 left at the placeholder activates from genesis, before bigBucketsCompute")
}

func TestEnableEpochs_Validate_AuditChangesV7AfterV6(t *testing.T) {
	t.Parallel()

	// V6 still at the placeholder is a template rather than a real schedule, so the
	// ordering is not enforced against it.
	assert.NoError(t, config.EnableEpochs{}.Validate(), "template placeholder")
	assert.NoError(t, config.EnableEpochs{FixAuditChangesV7: 5, FixAuditChangesV8: 6}.Validate(), "V7 scheduled with V6 at the placeholder")

	assert.ErrorContains(t, config.EnableEpochs{
		FixMarketBuyOverflow: 100,
		FixAuditChangesV4:    101,
		FixAuditChangesV5:    102,
		FixAuditChangesV6:    103,
		FixAuditChangesV7:    103,
	}.Validate(), "fixAuditChangesV7 (103) must be after fixAuditChangesV6 (103)",
		"sharing V6's epoch applies the V7 changes retroactively")
	assert.ErrorContains(t, config.EnableEpochs{
		FixMarketBuyOverflow: 100,
		FixAuditChangesV4:    101,
		FixAuditChangesV5:    102,
		FixAuditChangesV6:    103,
		FixAuditChangesV7:    102,
	}.Validate(), "fixAuditChangesV7 (102) must be after fixAuditChangesV6 (103)",
		"V7 before V6 applies the V7 changes retroactively")
	assert.ErrorContains(t, config.EnableEpochs{
		FixMarketBuyOverflow: 100,
		FixAuditChangesV4:    101,
		FixAuditChangesV5:    102,
		FixAuditChangesV6:    103,
	}.Validate(), "fixAuditChangesV7 (0) must be after fixAuditChangesV6 (103)",
		"V7 left at the placeholder activates from genesis")
}

func TestEnableEpochs_Validate_AuditChangesV8AfterV7(t *testing.T) {
	t.Parallel()

	// V7 still at the placeholder is a template rather than a real schedule, so the
	// ordering is not enforced against it.
	assert.NoError(t, config.EnableEpochs{}.Validate(), "template placeholder")
	assert.NoError(t, config.EnableEpochs{FixAuditChangesV8: 5}.Validate(), "V8 scheduled with V7 at the placeholder")

	assert.ErrorContains(t, config.EnableEpochs{
		FixMarketBuyOverflow: 100,
		FixAuditChangesV4:    101,
		FixAuditChangesV5:    102,
		FixAuditChangesV6:    103,
		FixAuditChangesV7:    104,
		FixAuditChangesV8:    104,
	}.Validate(), "fixAuditChangesV8 (104) must be after fixAuditChangesV7 (104)",
		"sharing V7's epoch applies the V8 changes retroactively")
	assert.ErrorContains(t, config.EnableEpochs{
		FixMarketBuyOverflow: 100,
		FixAuditChangesV4:    101,
		FixAuditChangesV5:    102,
		FixAuditChangesV6:    103,
		FixAuditChangesV7:    104,
		FixAuditChangesV8:    103,
	}.Validate(), "fixAuditChangesV8 (103) must be after fixAuditChangesV7 (104)",
		"V8 before V7 applies the V8 changes retroactively")
	assert.ErrorContains(t, config.EnableEpochs{
		FixMarketBuyOverflow: 100,
		FixAuditChangesV4:    101,
		FixAuditChangesV5:    102,
		FixAuditChangesV6:    103,
		FixAuditChangesV7:    104,
	}.Validate(), "fixAuditChangesV8 (0) must be after fixAuditChangesV7 (104)",
		"V8 left at the placeholder activates from genesis")
}
