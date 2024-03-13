package factory

import (
	"testing"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/config"
	"github.com/stretchr/testify/assert"
)

func TestNewSoftwareVersionFactory_NilStatusHandlerShouldErr(t *testing.T) {
	t.Parallel()

	softwareVersionFactory, err := NewSoftwareVersionFactory(nil, config.SoftwareVersionConfig{})

	assert.Equal(t, common.ErrNilAppStatusHandler, err)
	assert.Nil(t, softwareVersionFactory)
}

func TestSoftwareVersionFactory_Create(t *testing.T) {
	t.Parallel()

	statusHandler := &mock.AppStatusHandlerStub{}
	softwareVersionFactory, _ := NewSoftwareVersionFactory(statusHandler, config.SoftwareVersionConfig{PollingIntervalInMinutes: 1})
	softwareVersionChecker, err := softwareVersionFactory.Create()

	assert.Nil(t, err)
	assert.NotNil(t, softwareVersionChecker)
}
