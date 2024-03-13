package scenjsonwrite

import (
	scenjsonmodel "github.com/klever-io/klever-go/kvm/scenarioexec/model"
	"github.com/klever-io/klever-go/kvm/scenarioexec/orderedjson"
)

// TestToJSONString converts a test object to its JSON representation.
func TestToJSONString(testTopLevel []*scenjsonmodel.Test) string {
	jobj := TestToOrderedJSON(testTopLevel)
	return orderedjson.JSONString(jobj) + "\n"
}

// TestToOrderedJSON converts a test object to an ordered JSON object.
func TestToOrderedJSON(testTopLevel []*scenjsonmodel.Test) orderedjson.OJsonObject {
	result := orderedjson.NewMap()
	for _, test := range testTopLevel {
		result.Put(test.TestName, testToOJ(test))
	}

	return result
}

func testToOJ(test *scenjsonmodel.Test) orderedjson.OJsonObject {
	testOJ := orderedjson.NewMap()

	if !test.CheckGas {
		ojFalse := orderedjson.OJsonBool(false)
		testOJ.Put("checkGas", &ojFalse)
	}

	testOJ.Put("pre", AccountsToOJ(test.Pre))

	var blockList []orderedjson.OJsonObject
	for _, block := range test.Blocks {
		blockList = append(blockList, blockToOJ(block))
	}
	blocksOJ := orderedjson.OJsonList(blockList)
	testOJ.Put("blocks", &blocksOJ)
	testOJ.Put("network", stringToOJ(test.Network))
	testOJ.Put("blockHashes", valueListToOJ(test.BlockHashes))
	testOJ.Put("postState", checkAccountsToOJ(test.PostState))
	return testOJ
}

func transactionToTestOJ(tx *scenjsonmodel.Transaction) orderedjson.OJsonObject {
	transactionOJ := orderedjson.NewMap()
	transactionOJ.Put("nonce", uint64ToOJ(tx.Nonce))
	transactionOJ.Put("function", stringToOJ(tx.Function))
	transactionOJ.Put("gasLimit", uint64ToOJ(tx.GasLimit))
	transactionOJ.Put("value", bigIntToOJ(tx.KLVValue))
	transactionOJ.Put("to", bytesFromStringToOJ(tx.To))

	var argList []orderedjson.OJsonObject
	for _, arg := range tx.Arguments {
		argList = append(argList, bytesFromTreeToOJ(arg))
	}
	argOJ := orderedjson.OJsonList(argList)
	transactionOJ.Put("arguments", &argOJ)

	if len(tx.Code.Original) > 0 {
		transactionOJ.Put("contractCode", bytesFromStringToOJ(tx.Code))
	}
	transactionOJ.Put("gasPrice", uint64ToOJ(tx.GasPrice))
	transactionOJ.Put("from", bytesFromStringToOJ(tx.From))

	return transactionOJ
}

func blockToOJ(block *scenjsonmodel.Block) orderedjson.OJsonObject {
	blockOJ := orderedjson.NewMap()

	var resultList []orderedjson.OJsonObject
	for _, blr := range block.Results {
		resultList = append(resultList, resultToOJ(blr))
	}
	resultsOJ := orderedjson.OJsonList(resultList)
	blockOJ.Put("results", &resultsOJ)

	var txList []orderedjson.OJsonObject
	for _, tx := range block.Transactions {
		txList = append(txList, transactionToTestOJ(tx))
	}
	txsOJ := orderedjson.OJsonList(txList)
	blockOJ.Put("transactions", &txsOJ)

	blockHeaderOJ := orderedjson.NewMap()
	blockHeaderOJ.Put("gasLimit", bigIntToOJ(block.BlockHeader.GasLimit))
	blockHeaderOJ.Put("number", bigIntToOJ(block.BlockHeader.Number))
	blockHeaderOJ.Put("difficulty", bigIntToOJ(block.BlockHeader.Difficulty))
	blockHeaderOJ.Put("timestamp", uint64ToOJ(block.BlockHeader.Timestamp))
	blockHeaderOJ.Put("coinbase", bigIntToOJ(block.BlockHeader.Beneficiary))
	blockOJ.Put("blockHeader", blockHeaderOJ)

	return blockOJ
}
