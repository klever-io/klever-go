package processorNode

import (
	cMock "github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/process/rating"
	"github.com/klever-io/klever-go/crypto"
	"github.com/klever-io/klever-go/crypto/signing"
	"github.com/klever-io/klever-go/crypto/signing/ed25519"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/trie"
)

func GenerateSkAndPk() (crypto.PrivateKey, crypto.PublicKey, crypto.KeyGenerator) {
	suite := ed25519.NewEd25519()
	keygen := signing.NewKeyGenerator(suite)
	sk, pk := keygen.GeneratePair()

	return sk, pk, keygen
}

func (n *ProcessorNode) CreateAccountsDB(
	accountFactory state.AccountFactory,
	trieStorageManager data.StorageManager,
) (*state.AccountsDB, data.Trie) {
	tr, _ := trie.NewTrie(trieStorageManager, n.InternalMarshalizer, n.Hasher, 5)
	adb, _ := state.NewAccountsDB(tr, n.Hasher, n.InternalMarshalizer, accountFactory, core.Normal)
	return adb, tr
}

func InitMultiSignerMock() *cMock.BelNevMock {
	multiSigner := cMock.NewMultiSigner()
	multiSigner.CreateCommitmentMock = func() ([]byte, []byte) {
		return []byte("commSecret"), []byte("commitment")
	}
	multiSigner.VerifySignatureShareMock = func(index uint16, sig []byte, msg []byte, bitmap []byte) error {
		return nil
	}
	multiSigner.VerifyMock = func(msg []byte, bitmap []byte) error {
		return nil
	}
	multiSigner.AggregateSigsMock = func(bitmap []byte) ([]byte, error) {
		return []byte("aggregatedSig"), nil
	}
	multiSigner.AggregateCommitmentsMock = func(bitmap []byte) error {
		return nil
	}
	multiSigner.CreateSignatureShareMock = func(msg []byte, bitmap []byte) ([]byte, error) {
		return []byte("partialSign"), nil
	}
	return multiSigner
}

func CreateRatingsData() *rating.RatingsData {
	ratingsConfig := config.RatingsConfig{
		RatingSteps: config.RatingSteps{
			HoursToMaxRatingFromStartRating: 2,
			ProposerValidatorImportance:     1,
			ProposerDecreaseFactor:          -4,
			ValidatorDecreaseFactor:         -4,
			ConsecutiveMissedBlocksPenalty:  1.1,
		},
		General: config.General{
			StartRating:           500000,
			MaxRating:             1000000,
			MinRating:             1,
			SignedBlocksThreshold: 0.025,
			SelectionChances: []*config.SelectionChance{
				{
					MaxThreshold:  0,
					ChancePercent: 5,
				},
				{
					MaxThreshold:  100000,
					ChancePercent: 0,
				},
				{
					MaxThreshold:  200000,
					ChancePercent: 16,
				},
				{
					MaxThreshold:  300000,
					ChancePercent: 17,
				},
				{
					MaxThreshold:  400000,
					ChancePercent: 18,
				},
				{
					MaxThreshold:  500000,
					ChancePercent: 19,
				},
				{
					MaxThreshold:  600000,
					ChancePercent: 20,
				},
				{
					MaxThreshold:  700000,
					ChancePercent: 21,
				},
				{
					MaxThreshold:  800000,
					ChancePercent: 22,
				},
				{
					MaxThreshold:  900000,
					ChancePercent: 23,
				},
				{
					MaxThreshold:  1000000,
					ChancePercent: 24,
				},
			},
		},
	}

	ratingDataArgs := rating.RatingsDataArg{
		Config:        ratingsConfig,
		MinNodes:      400,
		ConsensusSize: 63,

		SlotDurationMilliseconds: 6000,
	}

	ratingsData, _ := rating.NewRatingsData(ratingDataArgs)
	return ratingsData
}
