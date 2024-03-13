package kapps

import "github.com/klever-io/klever-go/common"

func (i *ITOData) GetPackByAmount(assetID []byte, amount int64) (*Pack, error) {
	packData := i.GetPackData()[string(assetID)]

	if packData == nil {
		return nil, common.ErrInvalidValue
	}

	if len(packData.Packs) == 0 {
		return nil, common.ErrInvalidValue
	}

	chosenPack := packData.Packs[len(packData.Packs)-1]
	for _, pack := range packData.Packs {
		if amount > pack.Amount {
			continue
		}

		chosenPack = pack
		break
	}

	return chosenPack, nil
}
