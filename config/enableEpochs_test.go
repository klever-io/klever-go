package config_test

import (
	"testing"

	"github.com/klever-io/klever-go/config"
	"github.com/stretchr/testify/assert"
)

func TestEnableEpochs_Validate(t *testing.T) {
	t.Parallel()

	assert.NoError(t, config.EnableEpochs{}.Validate(), "template placeholder")
	assert.NoError(t, config.EnableEpochs{FixMarketBuyOverflow: 100, FixAuditChangesV4: 101}.Validate())

	assert.Error(t, config.EnableEpochs{FixMarketBuyOverflow: 100, FixAuditChangesV4: 100}.Validate(),
		"same epoch leaves an empty freeze window")
	assert.Error(t, config.EnableEpochs{FixMarketBuyOverflow: 100, FixAuditChangesV4: 99}.Validate(),
		"thaw before freeze leaves an empty freeze window")
	assert.Error(t, config.EnableEpochs{FixMarketBuyOverflow: 100}.Validate(),
		"thaw left at the placeholder leaves an empty freeze window")
}
