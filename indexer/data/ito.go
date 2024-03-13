package data

type AlteredITOHandler interface {
	Add(key string, order *AlteredITOs)
	Get(key string) (*AlteredITOs, bool)
	GetAll() map[string]*AlteredITOs
	Len() int
	IsInterfaceNil() bool
}

type AlteredITOs struct {
	IsNew                  bool
	IsDisabled             bool
	IsEnabled              bool
	RemovedAddresses       map[string]WhitelistInfo
	AddedAddresses         map[string]WhitelistInfo
	AlteredAddresses       map[string]AlteredITOAddresses
	DefaultLimitPerAddress int64
}

type AlteredITOAddresses struct {
	NftsBuyed int64
}

func NewAlteredITOs() *alteredITOs {
	return &alteredITOs{
		altered: make(map[string]*AlteredITOs),
	}
}

type alteredITOs struct {
	altered map[string]*AlteredITOs
}

func (ao *alteredITOs) Add(key string, ito *AlteredITOs) {
	if ito.AddedAddresses == nil {
		ito.AddedAddresses = make(map[string]WhitelistInfo)
	}
	if ito.RemovedAddresses == nil {
		ito.RemovedAddresses = make(map[string]WhitelistInfo)
	}
	if ito.AlteredAddresses == nil {
		ito.AlteredAddresses = make(map[string]AlteredITOAddresses)
	}

	if item, ok := ao.altered[key]; ok {
		for k, info := range ito.AlteredAddresses {
			item.AlteredAddresses[k] = info
		}

		for k, info := range ito.RemovedAddresses {
			item.RemovedAddresses[k] = info
		}

		for k, info := range ito.AddedAddresses {
			item.AddedAddresses[k] = info
		}

		item.IsDisabled = ito.IsDisabled
		item.IsEnabled = ito.IsEnabled

		ao.altered[key] = item

		return
	}

	ao.altered[key] = ito
}

func (ao *alteredITOs) Get(key string) (*AlteredITOs, bool) {
	altered, ok := ao.altered[key]

	return altered, ok
}

func (ao *alteredITOs) Len() int {
	return len(ao.altered)
}

func (ao *alteredITOs) GetAll() map[string]*AlteredITOs {
	if ao == nil || ao.altered == nil {
		return make(map[string]*AlteredITOs, 0)
	}

	return ao.altered
}

// IsInterfaceNil returns true if underlying object is nil
func (ao *alteredITOs) IsInterfaceNil() bool {
	return ao == nil
}
