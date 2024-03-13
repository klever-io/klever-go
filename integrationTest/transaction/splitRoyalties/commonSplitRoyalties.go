package splitroyalties

import "github.com/klever-io/klever-go/core"

func computeRoyalty(amount int64, totalPercentage uint32) int64 {
	return int64(float64(amount) * float64(totalPercentage) / float64(core.HundredPercent))
}
