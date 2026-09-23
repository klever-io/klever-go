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
	}.Validate())

	assert.Error(t, config.EnableEpochs{
		FixMarketBuyOverflow: 100,
		FixAuditChangesV4:    100,
		FixAuditChangesV5:    102,
		FixAuditChangesV6:    103,
	}.Validate(), "same epoch leaves an empty freeze window")
	assert.Error(t, config.EnableEpochs{
		FixMarketBuyOverflow: 100,
		FixAuditChangesV4:    99,
		FixAuditChangesV5:    102,
		FixAuditChangesV6:    103,
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
	}.Validate(), "sharing V4's epoch applies the V5 changes retroactively")
	assert.Error(t, config.EnableEpochs{
		FixMarketBuyOverflow: 100,
		FixAuditChangesV4:    101,
		FixAuditChangesV5:    100,
		FixAuditChangesV6:    102,
	}.Validate(), "V5 before V4 applies the V5 changes retroactively")
	assert.Error(t, config.EnableEpochs{
		FixMarketBuyOverflow: 100,
		FixAuditChangesV4:    101,
		FixAuditChangesV6:    102,
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

func TestEnableEpochs_Validate_AuditChangesV6AfterV5(t *testing.T) {
	t.Parallel()

	// V5 still at the placeholder is a template rather than a real schedule, so the
	// ordering is not enforced against it.
	assert.NoError(t, config.EnableEpochs{}.Validate(), "template placeholder")

	assert.Error(t, config.EnableEpochs{
		FixMarketBuyOverflow: 100,
		FixAuditChangesV4:    101,
		FixAuditChangesV5:    102,
		FixAuditChangesV6:    102,
	}.Validate(), "sharing V5's epoch applies the V6 changes retroactively")
	assert.Error(t, config.EnableEpochs{
		FixMarketBuyOverflow: 100,
		FixAuditChangesV4:    101,
		FixAuditChangesV5:    102,
		FixAuditChangesV6:    101,
	}.Validate(), "V6 before V5 applies the V6 changes retroactively")
	assert.Error(t, config.EnableEpochs{
		FixMarketBuyOverflow: 100,
		FixAuditChangesV4:    101,
		FixAuditChangesV5:    102,
	}.Validate(), "V6 left at the placeholder activates from genesis")
}
