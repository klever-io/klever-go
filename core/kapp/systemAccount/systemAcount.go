package systemAccount

import (
	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/tools/marshal"
)

var _ kapp.SystemAccountKapp = (*systemAccountKApp)(nil)

var log = logger.GetOrCreate("kapp/systemAccount")

type systemAccountKApp struct {
	marshalizer    marshal.Marshalizer
	pubkeyConv     core.PubkeyConverter
	accountsCacher state.AccountsCacher
	forkController core.ForkController
	KAppController kapp.KAppController
}

type ArgsNewSystemAccountKApp struct {
	Marshalizer    marshal.Marshalizer
	PubkeyConv     core.PubkeyConverter
	ForkController core.ForkController
}

func NewSystemAccountKApp(
	args *ArgsNewSystemAccountKApp,
) (*systemAccountKApp, error) {
	if check.IfNil(args.Marshalizer) {
		return nil, common.ErrNilMarshalizer
	}
	if check.IfNil(args.PubkeyConv) {
		return nil, common.ErrNilPubkeyConverter
	}
	if check.IfNil(args.ForkController) {
		return nil, common.ErrNilForkController
	}

	v := &systemAccountKApp{
		marshalizer:    args.Marshalizer,
		pubkeyConv:     args.PubkeyConv,
		forkController: args.ForkController,
	}

	return v, nil
}

func (s *systemAccountKApp) IsInterfaceNil() bool {
	return s == nil
}

func (s *systemAccountKApp) getKApp() (state.KAppAccountHandler, error) {
	kapp, err := s.accountsCacher.LoadKApp(kapps.SystemAccountKAppAddress)
	if err != nil {
		return nil, err
	}

	return kapp, nil
}

func (s *systemAccountKApp) saveKApp(app state.KAppAccountHandler) error {
	return s.accountsCacher.UpdateKapp(app)
}

func (s *systemAccountKApp) SetKAppController(controller kapp.KAppController) error {
	s.KAppController = controller

	return nil
}

func (s *systemAccountKApp) SetAccountsCacher(cacher state.AccountsCacher) error {
	if check.IfNil(cacher) {
		return common.ErrNilAccountsAdapter
	}

	s.accountsCacher = cacher

	return nil
}

func (s *systemAccountKApp) SFTSetMetadata(asset, nonce []byte, args [][]byte) error {
	kapp, err := s.getKApp()
	if err != nil {
		return err
	}

	meta, err := s.getKDAData(asset, nonce)
	if err != nil {
		return err
	}

	// if one argument set only attributes
	switch len(args) {
	case 1:
		meta.Metadata.Attributes = args[0]
	case 2:
		meta.Metadata.Name = args[0]
		meta.Metadata.Attributes = args[1]
	default:
		return common.ErrInvalidParameter
	}

	log.Trace("SFTSetMetadata", "name", string(meta.Metadata.Name), "attributes", string(meta.Metadata.Attributes))

	key := kdautils.ToKDAKey(asset, nonce)
	return s.marshalAndSave(key, kapp, meta)
}

func (s *systemAccountKApp) SFTAddCirculation(asset, nonce []byte, amount int64) error {
	// amount can be negative, removing from circulation
	// this will happen when burning tokens
	if amount == 0 {
		// nothing to do
		return nil
	}

	kapp, err := s.getKApp()
	if err != nil {
		return err
	}

	meta, err := s.getKDAData(asset, nonce)
	if err != nil {
		return err
	}

	meta.Circulation += amount

	log.Trace("SFTAddCirculation", "max supply", meta.MaxSupply, "value", meta.Circulation)

	if meta.Circulation > meta.MaxSupply && meta.MaxSupply != 0 {
		return common.ErrMaxSupplyExceeded
	}

	key := kdautils.ToKDAKey(asset, nonce)

	return s.marshalAndSave(key, kapp, meta)
}

func (s *systemAccountKApp) SFTGetMeta(asset, nonce []byte) (*kapps.MetaV2, error) {
	data, err := s.getKDAData(asset, nonce)
	if err == common.ErrNilTrie {
		return &kapps.MetaV2{Metadata: &kapps.MetaV2Data{}}, nil
	}

	return data, err
}

func (s *systemAccountKApp) SFTCreateMeta(asset, nonce []byte, supply int64, hash []byte) error {
	kapp, err := s.getKApp()
	if err != nil {
		return err
	}

	_, err = s.getKDAData(asset, nonce)
	if err == nil {
		return common.ErrAssetAlreadyExists
	}

	meta := &kapps.MetaV2{}
	meta.MaxSupply = supply
	meta.Metadata = &kapps.MetaV2Data{
		Hash: hash,
	}

	key := kdautils.ToKDAKey(asset, nonce)

	log.Trace("CreateSupply", "max supply", supply)

	return s.marshalAndSave(key, kapp, meta)
}

func (s *systemAccountKApp) marshalAndSave(key []byte, kapp state.KAppAccountHandler, meta *kapps.MetaV2) error {
	metaBytes, err := s.marshalizer.Marshal(meta)
	if err != nil {
		return err
	}

	return kapp.DataTrieTracker().SaveKeyValue(key, metaBytes)
}

func (s *systemAccountKApp) getKDAData(asset, nonce []byte) (*kapps.MetaV2, error) {
	kapp, err := s.getKApp()
	if err != nil {
		return nil, err
	}

	key := kdautils.ToKDAKey(asset, nonce)
	log.Trace("getKDAData", "key", string(key))

	metaBytes, err := kapp.DataTrieTracker().RetrieveValue(key)
	if err != nil {
		return nil, err
	}

	if len(metaBytes) == 0 {
		return nil, common.ErrAssetNotFound
	}

	meta := &kapps.MetaV2{}

	err = s.marshalizer.Unmarshal(meta, metaBytes)
	if err != nil {
		return nil, err
	}

	return meta, nil
}
