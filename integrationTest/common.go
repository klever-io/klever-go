package integrationTest

import (
	"time"

	logger "github.com/klever-io/klever-go-logger"
)

var log = logger.GetOrCreate("IntegrationTest")

// P2pBootstrapDelay is used so that nodes have enough time to bootstrap
var P2pBootstrapDelay = 5 * time.Second

// StepDelay is used so that transactions can disseminate properly
var StepDelay = time.Millisecond * 180
