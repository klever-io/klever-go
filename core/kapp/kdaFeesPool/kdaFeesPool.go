//go:generate protoc -I=proto -I=$GOPATH/src -I=$GOPATH/src/github.com/klever-io/klever-go/protobuf  --go_out=Mgoogle/protobuf/any.proto=google.golang.org/protobuf/types/known/anypb:. kdaFeesPoolData.proto
package kdafeespool

import (
	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/tools/marshal"
)

var _ kapp.KDAFeesPoolKapp = (*kdaFeesPoolKApp)(nil)

var log = logger.GetOrCreate("kapp/kdaFeesPool")

type kdaFeesPoolKApp struct {
	marshalizer    marshal.Marshalizer
	pubkeyConv     core.PubkeyConverter
	accountsCacher state.AccountsCacher
	forkController core.ForkController
	KAppController kapp.KAppController
}

// ArgsNewValidatorKApp holds the arguments needed to create a ValidatorsKApp
type ArgsNewKDAFeesPoolKApp struct {
	Marshalizer    marshal.Marshalizer
	PubkeyConv     core.PubkeyConverter
	ForkController core.ForkController
}

// NewKDAFeesPoolKApp creates a token fees pool KApp
func NewKDAFeesPoolKApp(
	args *ArgsNewKDAFeesPoolKApp,
) (*kdaFeesPoolKApp, error) {
	if check.IfNil(args.Marshalizer) {
		return nil, common.ErrNilMarshalizer
	}
	if check.IfNil(args.PubkeyConv) {
		return nil, common.ErrNilPubkeyConverter
	}
	if check.IfNil(args.ForkController) {
		return nil, common.ErrNilForkController
	}

	v := &kdaFeesPoolKApp{
		marshalizer:    args.Marshalizer,
		pubkeyConv:     args.PubkeyConv,
		forkController: args.ForkController,
	}

	return v, nil
}

func (k *kdaFeesPoolKApp) SetKAppController(controller kapp.KAppController) error {
	k.KAppController = controller

	return nil
}

func (v *kdaFeesPoolKApp) SetAccountsCacher(cacher state.AccountsCacher) error {
	if check.IfNil(cacher) {
		return common.ErrNilAccountsAdapter
	}

	v.accountsCacher = cacher

	return nil
}

func (v *kdaFeesPoolKApp) GetAccountsCacher() state.AccountsCacher {
	return v.accountsCacher
}

func (v *kdaFeesPoolKApp) getKApp() (state.KAppAccountHandler, error) {
	kapp, err := v.accountsCacher.LoadKApp(kapps.KDAFeesPoolKAppAddress)
	if err != nil {
		return nil, err
	}

	return kapp, nil
}

func (v *kdaFeesPoolKApp) saveKApp(app state.KAppAccountHandler) error {
	return v.accountsCacher.UpdateKapp(app)
}

func (v *kdaFeesPoolKApp) getPool(app state.KAppAccountHandler, kda []byte) (*KDAFeesPoolData, error) {
	// Check if account already registered as validator
	data := app.GetStorage(kda)
	if len(data) == 0 {
		return nil, common.ErrAssetPoolNotFound
	}

	val := &KDAFeesPoolData{}
	err := v.marshalizer.Unmarshal(val, data)
	if err != nil {
		log.Debug("getKDAFeePool", "kda", string(kda), "err", err.Error())
		return nil, common.ErrAssetPoolNotFound
	}

	return val, nil
}

func (v *kdaFeesPoolKApp) setPool(app state.KAppAccountHandler, kda []byte, val *KDAFeesPoolData) error {
	data, err := v.marshalizer.Marshal(val)
	if err != nil {
		log.Debug("setKDAFeePool", "kda", string(kda), "err", err.Error())
		return err
	}

	return app.SetStorage(kda, data)
}

// IsInterfaceNil verifies if the underlying object is nil or not
func (v *kdaFeesPoolKApp) IsInterfaceNil() bool {
	return v == nil
}
