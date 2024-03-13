package transaction

import (
	"github.com/klever-io/klever-go/core/process"
)

func (inTx *InterceptedTransaction) SetWhitelistHandler(handler process.WhiteListHandler) {
	inTx.whiteListerVerifiedTxs = handler
}
