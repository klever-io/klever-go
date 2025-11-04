package indexer

import (
	"encoding/hex"
	"testing"

	"github.com/klever-io/klever-go/common/mock"
	ptx "github.com/klever-io/klever-go/core/process/transaction"
	"github.com/klever-io/klever-go/indexer/data"
	"github.com/klever-io/klever-go/kapps"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_convertSFTMeta(t *testing.T) {
	cases := []struct {
		name  string
		input *kapps.MetaV2
		want  *data.Meta
	}{
		{
			"empty input",
			&kapps.MetaV2{},
			&data.Meta{
				Metadata: data.Metadata{
					ContentType: "text/plain",
				},
			},
		},
		{
			"non empty metadata hex data",
			&kapps.MetaV2{
				Metadata: &kapps.MetaV2Data{
					Name:       []byte{75, 76, 86},
					Hash:       []byte{40, 40, 40},
					Attributes: []byte{20, 20, 20, 255},
				},
			},
			&data.Meta{
				Metadata: data.Metadata{
					Name:        "KLV",
					Hash:        "282828",
					ContentType: "application/x-hex",
					Attributes:  "141414ff",
				},
			},
		},
		{
			"non empty metadata text/plain with {} in attributes",
			&kapps.MetaV2{
				Metadata: &kapps.MetaV2Data{
					Name:       []byte{75, 76, 86},
					Hash:       []byte{40, 40, 40},
					Attributes: []byte("{aaa}"),
				},
			},
			&data.Meta{
				Metadata: data.Metadata{
					Name:        "KLV",
					Hash:        "282828",
					ContentType: "text/plain",
					Attributes:  "{aaa}",
				},
			},
		},

		{
			"non empty metadata text/plain",
			&kapps.MetaV2{
				Metadata: &kapps.MetaV2Data{
					Name:       []byte{75, 76, 86},
					Hash:       []byte{40, 40, 40},
					Attributes: []byte{75, 76, 86},
				},
			},
			&data.Meta{
				Metadata: data.Metadata{
					Name:        "KLV",
					Hash:        "282828",
					ContentType: "text/plain",
					Attributes:  "KLV",
				},
			},
		},
		{
			"non empty metadata hex with {} in attributes",
			&kapps.MetaV2{
				Metadata: &kapps.MetaV2Data{
					Name:       []byte{75, 76, 86},
					Hash:       []byte{40, 40, 40},
					Attributes: append([]byte("{}"), 255),
				},
			},
			&data.Meta{
				Metadata: data.Metadata{
					Name:        "KLV",
					Hash:        "282828",
					ContentType: "application/x-hex",
					Attributes:  "7b7dff",
				},
			},
		},
		{
			"non empty metadata text/plain",
			&kapps.MetaV2{
				Metadata: &kapps.MetaV2Data{
					Name:       []byte{75, 76, 86},
					Hash:       []byte{40, 40, 40},
					Attributes: []byte(`{"name":"KLV","hash":"282828","attributes":"KLV"}`),
				},
			},
			&data.Meta{
				Metadata: data.Metadata{
					Name:        "KLV",
					Hash:        "282828",
					ContentType: "text/plain",
					Attributes:  `{"name":"KLV","hash":"282828","attributes":"KLV"}`,
				},
			},
		},
		{
			"non empty array metadata text/plain",
			&kapps.MetaV2{
				Metadata: &kapps.MetaV2Data{
					Name:       []byte{75, 76, 86},
					Hash:       []byte{40, 40, 40},
					Attributes: []byte(`[{"name":"KLV","hash":"282828","attributes":"KLV"}]`),
				},
			},
			&data.Meta{
				Metadata: data.Metadata{
					Name:        "KLV",
					Hash:        "282828",
					ContentType: "text/plain",
					Attributes:  `[{"name":"KLV","hash":"282828","attributes":"KLV"}]`,
				},
			},
		},
		{
			name: "metadata with special characters UTF-8",
			input: &kapps.MetaV2{
				Metadata: &kapps.MetaV2Data{
					Name:       []byte("KLV©"),
					Hash:       []byte{40, 40, 40},
					Attributes: []byte("áéíóú"),
				},
			},
			want: &data.Meta{
				Metadata: data.Metadata{
					Name:        "KLV©",
					Hash:        "282828",
					ContentType: "text/plain",
					Attributes:  "áéíóú",
				},
			},
		},
		{
			name: "metadata with invalid JSON",
			input: &kapps.MetaV2{
				Metadata: &kapps.MetaV2Data{
					Name:       []byte{75, 76, 86},
					Hash:       []byte{40, 40, 40},
					Attributes: []byte(`{"name":"KLV`), // JSON incompleto
				},
			},
			want: &data.Meta{
				Metadata: data.Metadata{
					Name:        "KLV",
					Hash:        "282828",
					ContentType: "text/plain",
					Attributes:  `{"name":"KLV`,
				},
			},
		},
	}

	for _, tt := range cases {
		c := commonProcessor{}
		t.Run(tt.name, func(t *testing.T) {
			as := assert.New(t)
			got := c.convertSFTMeta(tt.input)
			as.Equal(tt.want, got)
		})
	}
}

func TestSerializeAlteredSmartContracts(t *testing.T) {
	t.Parallel()

	index := "smart-contracts"

	t.Run("EmptyMap", func(t *testing.T) {
		alteredSCs := make(map[string][]*data.AlteredSmartContract)
		buffSlice := data.NewBufferSlice(data.DefaultMaxBulkSize)

		err := SerializeAlteredSmartContracts(alteredSCs, buffSlice, index)
		require.NoError(t, err)

		buffers := buffSlice.Buffers()
		require.Equal(t, 0, len(buffers))
	})

	t.Run("NilMap", func(t *testing.T) {
		buffSlice := data.NewBufferSlice(data.DefaultMaxBulkSize)

		err := SerializeAlteredSmartContracts(nil, buffSlice, index)
		require.NoError(t, err)

		buffers := buffSlice.Buffers()
		require.Equal(t, 0, len(buffers))
	})

	t.Run("SingleSmartContract", func(t *testing.T) {
		scAddress := "klv1qqqqqqqqqqqqqpgq0lrw7j5txudy4hwj9rjg0u8shwgk2xpz5wusyzujwq"
		alteredSCs := map[string][]*data.AlteredSmartContract{
			scAddress: {
				{IsNew: false},
				{IsNew: false},
				{IsNew: true},
			},
		}

		buffSlice := data.NewBufferSlice(data.DefaultMaxBulkSize)

		err := SerializeAlteredSmartContracts(alteredSCs, buffSlice, index)
		require.NoError(t, err)

		buffers := buffSlice.Buffers()
		require.Equal(t, 1, len(buffers))

		// Verify the buffer contains expected data
		bufferContent := buffers[0].String()
		require.Contains(t, bufferContent, scAddress)
		require.Contains(t, bufferContent, `"_index":"smart-contracts"`)
		require.Contains(t, bufferContent, `"update"`)
		require.Contains(t, bufferContent, `"params": {"count": 3}`)
		require.Contains(t, bufferContent, `"upsert": {"totalTransactions": 3}`)
		require.Contains(t, bufferContent, "painless")
		require.Contains(t, bufferContent, "totalTransactions")
	})

	t.Run("MultipleSmartContracts", func(t *testing.T) {
		scAddress1 := "klv1qqqqqqqqqqqqqpgq0lrw7j5txudy4hwj9rjg0u8shwgk2xpz5wusyzujwq"
		scAddress2 := "klv1qqqqqqqqqqqqqpgq5z9k5evwg7t99jqw7tlpn5kpgf4rkpr8vczsmhszh7"

		alteredSCs := map[string][]*data.AlteredSmartContract{
			scAddress1: {
				{IsNew: true},
				{IsNew: false},
			},
			scAddress2: {
				{IsNew: true},
				{IsNew: false},
				{IsNew: false},
			},
		}

		buffSlice := data.NewBufferSlice(data.DefaultMaxBulkSize)

		err := SerializeAlteredSmartContracts(alteredSCs, buffSlice, index)
		require.NoError(t, err)

		buffers := buffSlice.Buffers()
		require.Equal(t, 1, len(buffers))

		bufferContent := buffers[0].String()
		require.Contains(t, bufferContent, scAddress1)
		require.Contains(t, bufferContent, scAddress2)
		require.Contains(t, bufferContent, `"params": {"count": 2}`)
		require.Contains(t, bufferContent, `"params": {"count": 3}`)
	})

	t.Run("AddressWithSpecialCharacters", func(t *testing.T) {
		// Test address that needs JSON escaping
		scAddress := `address"with"quotes`
		alteredSCs := map[string][]*data.AlteredSmartContract{
			scAddress: {
				{IsNew: true},
			},
		}

		buffSlice := data.NewBufferSlice(data.DefaultMaxBulkSize)

		err := SerializeAlteredSmartContracts(alteredSCs, buffSlice, index)
		require.NoError(t, err)

		buffers := buffSlice.Buffers()
		require.Equal(t, 1, len(buffers))

		// Verify JSON escaping was applied
		bufferContent := buffers[0].String()
		require.Contains(t, bufferContent, `address\"with\"quotes`)
	})

	t.Run("SingleTransaction", func(t *testing.T) {
		scAddress := "klv1qqqqqqqqqqqqqpgq0lrw7j5txudy4hwj9rjg0u8shwgk2xpz5wusyzujwq"
		alteredSCs := map[string][]*data.AlteredSmartContract{
			scAddress: {
				{IsNew: false},
			},
		}

		buffSlice := data.NewBufferSlice(data.DefaultMaxBulkSize)

		err := SerializeAlteredSmartContracts(alteredSCs, buffSlice, index)
		require.NoError(t, err)

		buffers := buffSlice.Buffers()
		require.Equal(t, 1, len(buffers))

		bufferContent := buffers[0].String()
		require.Contains(t, bufferContent, `"params": {"count": 1}`)
		require.Contains(t, bufferContent, `"upsert": {"totalTransactions": 1}`)
	})

	t.Run("VerifyPainlessScript", func(t *testing.T) {
		scAddress := "klv1test"
		alteredSCs := map[string][]*data.AlteredSmartContract{
			scAddress: {
				{IsNew: true},
			},
		}

		buffSlice := data.NewBufferSlice(data.DefaultMaxBulkSize)

		err := SerializeAlteredSmartContracts(alteredSCs, buffSlice, index)
		require.NoError(t, err)

		buffers := buffSlice.Buffers()
		bufferContent := buffers[0].String()

		// Verify Painless script structure
		require.Contains(t, bufferContent, `"lang": "painless"`)
		require.Contains(t, bufferContent, "ctx._source")
		require.Contains(t, bufferContent, "totalTransactions")
		require.Contains(t, bufferContent, "params.count")
	})
}

func Test_receiptToMap(t *testing.T) {
	t.Parallel()

	// Helper to create 32-byte address
	createAddress := func(s string) []byte {
		addr := make([]byte, 32)
		copy(addr, []byte(s))
		return addr
	}

	// Create mock converter
	addressConverter := &mock.PubkeyConverterStub{
		EncodeCalled: func(pkBytes []byte) string {
			if len(pkBytes) == 0 {
				return ""
			}
			// Trim null bytes from the end
			trimmed := pkBytes
			for i := len(pkBytes) - 1; i >= 0; i-- {
				if pkBytes[i] == 0 {
					trimmed = pkBytes[:i]
				} else {
					break
				}
			}
			if len(trimmed) > 10 {
				trimmed = trimmed[:10]
			}
			return "klv1" + string(trimmed)
		},
	}

	cp := &commonProcessor{
		addressPubkeyConverter: addressConverter,
	}

	t.Run("EmptyData", func(t *testing.T) {
		result, err := cp.receiptToMap([][]byte{})
		require.NoError(t, err)
		require.NotNil(t, result)
		require.Empty(t, result)
	})

	t.Run("InvalidIDLength", func(t *testing.T) {
		// First element must have at least 2 bytes (type + contractID)
		data := [][]byte{{0}} // Only 1 byte
		_, err := cp.receiptToMap(data)
		require.Error(t, err)
		require.Contains(t, err.Error(), "ID")
	})

	t.Run("Transfer", func(t *testing.T) {
		from := createAddress("from")
		to := createAddress("to")
		collection := []byte("KLV")
		nonce := []byte("1")
		value := []byte("1000")
		assetType := []byte{byte(kapps.KDAData_Fungible)}
		marketplaceID := []byte{1, 2, 3}
		orderID := []byte{4, 5, 6}

		data := [][]byte{
			{byte(ptx.Transfer), 1}, // type + contractID
			from,
			to,
			value,
			collection,
			nonce,
			assetType,
			marketplaceID,
			orderID,
		}

		result, err := cp.receiptToMap(data)
		require.NoError(t, err)
		require.Equal(t, ptx.Transfer, result["type"])
		require.Equal(t, 1, result["cID"])
		require.Equal(t, "Transfer", result["typeString"])
		require.Equal(t, "klv1from", result["from"])
		require.Equal(t, "klv1to", result["to"])
		require.Equal(t, int64(1000), result["value"])
		require.Equal(t, "KLV/1", result["assetId"])
		require.Equal(t, "1", result["nonce"])
		require.Equal(t, "KLV", result["collection"])
		require.Equal(t, "Fungible", result["assetType"])
		require.Equal(t, hex.EncodeToString(marketplaceID), result["marketplaceId"])
		require.Equal(t, hex.EncodeToString(orderID), result["orderId"])
	})

	t.Run("Transfer_InsufficientData", func(t *testing.T) {
		// Transfer requires at least 7 elements
		data := [][]byte{
			{byte(ptx.Transfer), 1},
			[]byte("from"),
		}

		_, err := cp.receiptToMap(data)
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid indexer data len")
	})

	t.Run("Transfer_InvalidValue", func(t *testing.T) {
		data := [][]byte{
			{byte(ptx.Transfer), 1},
			createAddress("from"),
			createAddress("to"),
			[]byte("invalid-number"), // Invalid int64
			[]byte("KLV"),
			[]byte("1"),
			{byte(kapps.KDAData_Fungible)},
			{},
			{},
		}

		_, err := cp.receiptToMap(data)
		require.Error(t, err)
	})

	t.Run("CreateKDA", func(t *testing.T) {
		assetID := []byte("MYTOKEN")
		data := [][]byte{
			{byte(ptx.CreateKDA), 2},
			assetID,
		}

		result, err := cp.receiptToMap(data)
		require.NoError(t, err)
		require.Equal(t, ptx.CreateKDA, result["type"])
		require.Equal(t, 2, result["cID"])
		require.Equal(t, "MYTOKEN", result["assetId"])
	})

	t.Run("UpdateKDA", func(t *testing.T) {
		assetID := []byte("MYTOKEN")
		data := [][]byte{
			{byte(ptx.UpdateKDA), 3},
			assetID,
		}

		result, err := cp.receiptToMap(data)
		require.NoError(t, err)
		require.Equal(t, ptx.UpdateKDA, result["type"])
		require.Equal(t, "MYTOKEN", result["assetId"])
	})

	t.Run("Freeze", func(t *testing.T) {
		bucketID := []byte{1, 2, 3, 4}
		from := createAddress("freezer")
		assetID := []byte("KLV")
		value := []byte("5000")

		data := [][]byte{
			{byte(ptx.Freeze), 4},
			bucketID,
			from,
			assetID,
			value,
		}

		result, err := cp.receiptToMap(data)
		require.NoError(t, err)
		require.Equal(t, ptx.Freeze, result["type"])
		require.Equal(t, hex.EncodeToString(bucketID), result["bucketId"])
		require.Equal(t, "klv1freezer", result["from"])
		require.Equal(t, "KLV", result["assetId"])
		require.Equal(t, int64(5000), result["value"])
	})

	t.Run("Unfreeze", func(t *testing.T) {
		bucketID := []byte{1, 2, 3, 4}
		availableEpoch := []byte("100")
		from := createAddress("unfreezer")
		assetID := []byte("KLV")
		value := []byte("3000")

		data := [][]byte{
			{byte(ptx.Unfreeze), 5},
			bucketID,
			availableEpoch,
			from,
			assetID,
			value,
		}

		result, err := cp.receiptToMap(data)
		require.NoError(t, err)
		require.Equal(t, ptx.Unfreeze, result["type"])
		require.Equal(t, hex.EncodeToString(bucketID), result["bucketId"])
		require.Equal(t, "100", result["availableEpoch"])
		require.Equal(t, "klv1unfreezer", result["from"])
		require.Equal(t, "KLV", result["assetId"])
		require.Equal(t, int64(3000), result["value"])
	})

	t.Run("Proposal", func(t *testing.T) {
		proposalID := []byte("proposal-123")
		data := [][]byte{
			{byte(ptx.Proposal), 6},
			proposalID,
		}

		result, err := cp.receiptToMap(data)
		require.NoError(t, err)
		require.Equal(t, ptx.Proposal, result["type"])
		require.Equal(t, "proposal-123", result["proposalId"])
	})

	t.Run("ProposalVote", func(t *testing.T) {
		proposalID := []byte("proposal-123")
		voter := createAddress("voter")
		voteType := []byte("1")
		votes := []byte("100")

		data := [][]byte{
			{byte(ptx.ProposalVote), 7},
			proposalID,
			voter,
			voteType,
			votes,
		}

		result, err := cp.receiptToMap(data)
		require.NoError(t, err)
		require.Equal(t, ptx.ProposalVote, result["type"])
		require.Equal(t, "proposal-123", result["proposalId"])
		require.Equal(t, "klv1voter", result["voter"])
		require.Equal(t, int64(1), result["voteType"])
		require.Equal(t, int64(100), result["votes"])
	})

	t.Run("Delegate", func(t *testing.T) {
		from := createAddress("delegator")
		bucketID := []byte{1, 2, 3}
		delegate := createAddress("validator")
		amountDelegated := []byte("1000")

		data := [][]byte{
			{byte(ptx.Delegate), 8},
			from,
			bucketID,
			delegate,
			amountDelegated,
		}

		result, err := cp.receiptToMap(data)
		require.NoError(t, err)
		require.Equal(t, ptx.Delegate, result["type"])
		require.Equal(t, "klv1delegator", result["from"])
		require.Equal(t, hex.EncodeToString(bucketID), result["bucketId"])
		require.Equal(t, "klv1validator", result["delegate"])
		require.Equal(t, "1000", result["amountDelegated"])
	})

	t.Run("Delegate_EmptyDelegateAddress", func(t *testing.T) {
		from := createAddress("delegator")
		bucketID := []byte{1, 2, 3}
		delegate := []byte{} // Empty delegate address
		amountDelegated := []byte("1000")

		data := [][]byte{
			{byte(ptx.Delegate), 8},
			from,
			bucketID,
			delegate,
			amountDelegated,
		}

		result, err := cp.receiptToMap(data)
		require.NoError(t, err)
		require.Equal(t, "", result["delegate"])
	})

	t.Run("UpdateValidator", func(t *testing.T) {
		validatorID := createAddress("validator")
		data := [][]byte{
			{byte(ptx.UpdateValidator), 9},
			validatorID,
		}

		result, err := cp.receiptToMap(data)
		require.NoError(t, err)
		require.Equal(t, ptx.UpdateValidator, result["type"])
		require.Equal(t, "klv1validator", result["id"])
	})

	t.Run("ConfigITO", func(t *testing.T) {
		assetID := []byte("ITO-TOKEN")
		data := [][]byte{
			{byte(ptx.ConfigITO), 10},
			assetID,
		}

		result, err := cp.receiptToMap(data)
		require.NoError(t, err)
		require.Equal(t, ptx.ConfigITO, result["type"])
		require.Equal(t, "ITO-TOKEN", result["assetId"])
	})

	t.Run("SetITOPrices", func(t *testing.T) {
		assetID := []byte("ITO-TOKEN")
		data := [][]byte{
			{byte(ptx.SetITOPrices), 11},
			assetID,
		}

		result, err := cp.receiptToMap(data)
		require.NoError(t, err)
		require.Equal(t, ptx.SetITOPrices, result["type"])
		require.Equal(t, "ITO-TOKEN", result["assetId"])
	})

	t.Run("CreateMarketplace", func(t *testing.T) {
		marketplaceID := []byte{1, 2, 3, 4}
		data := [][]byte{
			{byte(ptx.CreateMarketplace), 12},
			marketplaceID,
		}

		result, err := cp.receiptToMap(data)
		require.NoError(t, err)
		require.Equal(t, ptx.CreateMarketplace, result["type"])
		require.Equal(t, hex.EncodeToString(marketplaceID), result["marketplaceId"])
	})

	t.Run("ConfigMarketplace", func(t *testing.T) {
		marketplaceID := []byte{5, 6, 7, 8}
		data := [][]byte{
			{byte(ptx.ConfigMarketplace), 13},
			marketplaceID,
		}

		result, err := cp.receiptToMap(data)
		require.NoError(t, err)
		require.Equal(t, ptx.ConfigMarketplace, result["type"])
		require.Equal(t, hex.EncodeToString(marketplaceID), result["marketplaceId"])
	})

	t.Run("SignedBy", func(t *testing.T) {
		signer := createAddress("signer")
		weight := []byte("50")

		data := [][]byte{
			{byte(ptx.SignedBy), 14},
			signer,
			weight,
		}

		result, err := cp.receiptToMap(data)
		require.NoError(t, err)
		require.Equal(t, ptx.SignedBy, result["type"])
		require.Equal(t, "klv1signer", result["signer"])
		require.Equal(t, "50", result["weight"])
	})

	t.Run("Claim", func(t *testing.T) {
		amount := []byte("1000")
		orderID := []byte{1, 2, 3}
		marketplaceID := []byte{4, 5, 6}
		assetID := []byte("NFT-TOKEN")
		assetIDReceived := []byte("KLV")
		claimType := []byte("1")

		data := [][]byte{
			{byte(ptx.Claim), 15},
			amount,
			orderID,
			marketplaceID,
			assetID,
			assetIDReceived,
			claimType,
		}

		result, err := cp.receiptToMap(data)
		require.NoError(t, err)
		require.Equal(t, ptx.Claim, result["type"])
		require.Equal(t, int64(1000), result["amount"])
		require.Equal(t, hex.EncodeToString(orderID), result["orderId"])
		require.Equal(t, hex.EncodeToString(marketplaceID), result["marketplaceId"])
		require.Equal(t, "NFT-TOKEN", result["assetId"])
		require.Equal(t, "KLV", result["assetIdReceived"])
		require.Equal(t, int64(1), result["claimType"])
		require.Contains(t, result, "claimTypeString")
	})

	t.Run("Sell", func(t *testing.T) {
		orderID := []byte{1, 2, 3}
		marketplaceID := []byte{4, 5, 6}

		data := [][]byte{
			{byte(ptx.Sell), 16},
			orderID,
			marketplaceID,
		}

		result, err := cp.receiptToMap(data)
		require.NoError(t, err)
		require.Equal(t, ptx.Sell, result["type"])
		require.Equal(t, hex.EncodeToString(orderID), result["orderId"])
		require.Equal(t, hex.EncodeToString(marketplaceID), result["marketplaceId"])
	})

	t.Run("Buy", func(t *testing.T) {
		orderID := []byte{1, 2, 3}
		marketplaceID := []byte{4, 5, 6}
		executed := []byte("1")
		bidder := createAddress("bidder")
		amount := []byte("5000")
		currencyID := []byte("KLV")

		data := [][]byte{
			{byte(ptx.Buy), 17},
			orderID,
			marketplaceID,
			executed,
			bidder,
			amount,
			currencyID,
		}

		result, err := cp.receiptToMap(data)
		require.NoError(t, err)
		require.Equal(t, ptx.Buy, result["type"])
		require.Equal(t, hex.EncodeToString(orderID), result["orderId"])
		require.Equal(t, hex.EncodeToString(marketplaceID), result["marketplaceId"])
		require.Equal(t, true, result["executed"])
		require.Equal(t, "klv1bidder", result["bidder"])
		require.Equal(t, int64(5000), result["amount"])
		require.Equal(t, "KLV", result["currencyId"])
	})

	t.Run("Buy_NotExecuted", func(t *testing.T) {
		data := [][]byte{
			{byte(ptx.Buy), 17},
			{1, 2, 3},
			{4, 5, 6},
			[]byte("0"), // Not executed
			createAddress("bidder"),
			[]byte("5000"),
			[]byte("KLV"),
		}

		result, err := cp.receiptToMap(data)
		require.NoError(t, err)
		require.Equal(t, false, result["executed"])
	})

	t.Run("Withdraw", func(t *testing.T) {
		from := createAddress("withdrawer")
		assetID := []byte("KLV")
		amount := []byte("2000")

		data := [][]byte{
			{byte(ptx.Withdraw), 18},
			from,
			assetID,
			amount,
		}

		result, err := cp.receiptToMap(data)
		require.NoError(t, err)
		require.Equal(t, ptx.Withdraw, result["type"])
		require.Equal(t, "klv1withdrawer", result["from"]) // 10 chars max after klv1
		require.Equal(t, "KLV", result["assetId"])
		require.Equal(t, int64(2000), result["amount"])
	})

	t.Run("UpdateMetadata", func(t *testing.T) {
		owner := createAddress("owner")
		assetID := []byte("NFT-COL")
		nftNonce := []byte("123")

		data := [][]byte{
			{byte(ptx.UpdateMetadata), 19},
			owner,
			assetID,
			nftNonce,
		}

		result, err := cp.receiptToMap(data)
		require.NoError(t, err)
		require.Equal(t, ptx.UpdateMetadata, result["type"])
		require.Equal(t, "klv1owner", result["owner"])
		require.Equal(t, "NFT-COL", result["assetId"])
		require.Equal(t, "123", result["nftNonce"])
	})

	t.Run("UpdateKDAPool", func(t *testing.T) {
		poolID := []byte("pool-123")
		data := [][]byte{
			{byte(ptx.UpdateKDAPool), 20},
			poolID,
		}

		result, err := cp.receiptToMap(data)
		require.NoError(t, err)
		require.Equal(t, ptx.UpdateKDAPool, result["type"])
		require.Equal(t, "pool-123", result["poolId"])
	})

	t.Run("UpdateITO_WithAddress", func(t *testing.T) {
		address := createAddress("ito-address")
		assetID := []byte("ITO-TOKEN")
		amount := []byte("10000")

		data := [][]byte{
			{byte(ptx.UpdateITO), 21},
			address,
			assetID,
			amount,
		}

		result, err := cp.receiptToMap(data)
		require.NoError(t, err)
		require.Equal(t, ptx.UpdateITO, result["type"])
		require.Equal(t, "klv1ito-addres", result["address"]) // 10 chars max after klv1, "ito-address" -> "ito-addres"
		require.Equal(t, "ITO-TOKEN", result["assetId"])
		require.Equal(t, int64(10000), result["amount"])
	})

	t.Run("UpdateITO_WithoutAddress", func(t *testing.T) {
		assetID := []byte("ITO-TOKEN")

		data := [][]byte{
			{byte(ptx.UpdateITO), 21},
			assetID,
		}

		result, err := cp.receiptToMap(data)
		require.NoError(t, err)
		require.Equal(t, ptx.UpdateITO, result["type"])
		require.Equal(t, "ITO-TOKEN", result["assetId"])
		require.NotContains(t, result, "address")
	})

	t.Run("Deposit", func(t *testing.T) {
		from := createAddress("depositor")
		depositType := []byte("staking")
		amount := []byte("15000")
		assetID := []byte("KLV")
		currencyID := []byte("USD")

		data := [][]byte{
			{byte(ptx.Deposit), 22},
			from,
			depositType,
			amount,
			assetID,
			currencyID,
		}

		result, err := cp.receiptToMap(data)
		require.NoError(t, err)
		require.Equal(t, ptx.Deposit, result["type"])
		require.Equal(t, "klv1depositor", result["from"])
		require.Equal(t, "staking", result["depositType"])
		require.Equal(t, int64(15000), result["amount"])
		require.Equal(t, "KLV", result["assetId"])
		require.Equal(t, "USD", result["currencyId"])
	})

	t.Run("CancelOrder", func(t *testing.T) {
		marketplaceID := []byte{1, 2, 3}
		orderID := []byte{4, 5, 6}

		data := [][]byte{
			{byte(ptx.CancelOrder), 23},
			marketplaceID,
			orderID,
		}

		result, err := cp.receiptToMap(data)
		require.NoError(t, err)
		require.Equal(t, ptx.CancelOrder, result["type"])
		require.Equal(t, hex.EncodeToString(marketplaceID), result["marketplaceId"])
		require.Equal(t, hex.EncodeToString(orderID), result["orderId"])
	})

	t.Run("SCTrigger", func(t *testing.T) {
		triggerType := []byte("deploy")
		from := createAddress("deployer")
		contract := createAddress("contract")

		data := [][]byte{
			{byte(ptx.SCTrigger), 24},
			triggerType,
			from,
			contract,
		}

		result, err := cp.receiptToMap(data)
		require.NoError(t, err)
		require.Equal(t, ptx.SCTrigger, result["type"])
		require.Equal(t, "deploy", result["triggerType"])
		require.Equal(t, "klv1deployer", result["from"])
		require.Equal(t, "klv1contract", result["contract"])
	})

	t.Run("SetAccountName", func(t *testing.T) {
		name := []byte("my-account")
		address := createAddress("account")

		data := [][]byte{
			{byte(ptx.SetAccountName), 25},
			name,
			address,
		}

		result, err := cp.receiptToMap(data)
		require.NoError(t, err)
		require.Equal(t, ptx.SetAccountName, result["type"])
		require.Equal(t, "my-account", result["name"])
		require.Equal(t, "klv1account", result["address"])
	})

	t.Run("SetAccountName_InvalidLength", func(t *testing.T) {
		// SetAccountName requires exactly 3 elements
		data := [][]byte{
			{byte(ptx.SetAccountName), 25},
			[]byte("name"),
		}

		_, err := cp.receiptToMap(data)
		require.Error(t, err)
	})

	t.Run("UpdateAccountPermission", func(t *testing.T) {
		address := createAddress("account")

		data := [][]byte{
			{byte(ptx.UpdateAccountPermission), 26},
			address,
		}

		result, err := cp.receiptToMap(data)
		require.NoError(t, err)
		require.Equal(t, ptx.UpdateAccountPermission, result["type"])
		require.Equal(t, "klv1account", result["address"])
	})
}
