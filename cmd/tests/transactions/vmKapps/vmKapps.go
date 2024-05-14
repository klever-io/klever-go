package vmKapps

import (
	"encoding/hex"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/klever-io/klever-go/crypto/pubkeyConverter"
	"github.com/klever-io/klever-go/data/transaction"

	"github.com/klever-io/klever-go/cmd/tests/utils"

	"github.com/klever-io/klever-go/cmd/tests/common"
)

var CONTRACT_ADDRESS string
var CONTRACT_ADDRESS_HEX []byte
var ADDRESS_CONVERTER, _ = pubkeyConverter.NewBech32PubkeyConverter(32)

const KLVHex = "4b4c56"

var testWalletAddress string
var testWalletAddressHex []byte
var delegateWalletAddressHex []byte
var testWalletAddress2Hex []byte
var testWalletAddress2 string

func RunTests(args common.TestArgs) {
	StartVars(args)
	TriggerKDATransactions(args)
	TriggerNFTTransactions(args)
	TriggerITOTransactions(args)
	TriggerKLVTransactions(args)
}

func StartVars(args common.TestArgs) {
	var err error

	_, _, testWalletAddress, err = utils.LoadKey(args.KeyFile)
	if err != nil {
		log.Fatalln(err)
	}

	testWalletAddressHex, err = ADDRESS_CONVERTER.Decode(testWalletAddress)
	if err != nil {
		log.Fatalln(err)
	}

	delegateWalletAddressHex, err = ADDRESS_CONVERTER.Decode(args.DelegateAddress)
	if err != nil {
		log.Fatalln(err)
	}

	_, _, testWalletAddress2, err = utils.LoadKey("wallet-generated-2.pem")
	if err != nil {
		log.Fatalln(err)
	}

	testWalletAddress2Hex, err = ADDRESS_CONVERTER.Decode(testWalletAddress2)
	if err != nil {
		log.Fatalln(err)
	}

	if !args.Deployed {
		kappsDeployHash, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile),
			"sc", "create", "--wasm", "../klever-vm-sdk-rs/contracts/examples/kapps/output/kapps.wasm", "--payable", "--payableBySC", "--readable")

		if err != nil {
			log.Fatalln(err)
		}

		status, kappsDeployTx, err := common.CheckTransactionWithResultsOnNode(kappsDeployHash, 10)
		if err != nil {
			log.Fatalln(err)
		}

		if !status {
			log.Fatalf("tx %s is not success.\n", kappsDeployHash)
		}

		CONTRACT_ADDRESS = kappsDeployTx.Logs.Events[0].Address

		fmt.Printf("CONTRACT_ADDRESS: %+v Deploy Kapps Hash: %+v receipts: %d\n\n", CONTRACT_ADDRESS, kappsDeployHash, len(kappsDeployTx.Receipts))
	} else {
		CONTRACT_ADDRESS = args.DeployedAddress
	}

	CONTRACT_ADDRESS_HEX, err = ADDRESS_CONVERTER.Decode(CONTRACT_ADDRESS)
	if err != nil {
		log.Fatalln(err)
	}
}

func TriggerKDATransactions(args common.TestArgs) {
	invokeContractCall("Set Acc Name", args.KeyFile, nil, "set_account_name", "74657374")

	createKDATx := invokeContractCall("Create KDA", args.KeyFile, nil, "kda_create")

	assetBytes := createKDATx.Receipts[len(createKDATx.Receipts)-1].Data[1]
	assetStr := string(assetBytes)
	assetHex := hex.EncodeToString(assetBytes)

	_, asset, err := common.GetAssetOnNode(assetStr, 10)
	if err != nil {
		log.Fatalln(err)
	}

	if asset.CirculatingSupply != 100000 {
		log.Fatalln("initial supply error. ", asset.InitialSupply, " != 100000")
	}

	invokeContractCall("Mint KDA", args.KeyFile, nil, "mint", assetHex, "05")

	_, asset, err = common.GetAssetOnNode(assetStr, 10)
	if err != nil {
		log.Fatalln(err)
	}

	if asset.CirculatingSupply != 100000+5 {
		log.Fatalln("initial supply error. ", asset.CirculatingSupply, " != 100000 + 5")
	}

	invokeContractCall("Burn KDA", args.KeyFile, nil, "burn", assetHex, "00", "04")

	_, asset, err = common.GetAssetOnNode(assetStr, 10)
	if err != nil {
		log.Fatalln(err)
	}

	if asset.CirculatingSupply != 100000+5-4 {
		log.Fatalln("initial supply error. ", asset.CirculatingSupply, " != 100000 + 5 - 4")
	}

	invokeContractCall("Transfer KDA", args.KeyFile, nil, "transfer_kda", hex.EncodeToString(testWalletAddress2Hex), assetHex, "00", "05")

	balancePostSend, err := common.GetBalanceOnNode(testWalletAddress2, assetStr, 10)
	if err != nil {
		log.Fatalln(err)
	}

	if balancePostSend != 5 {
		log.Fatalln("balance after transfer error. ", balancePostSend, " != ", 5)
	}

	invokeContractCall("Wipe KDA", args.KeyFile, nil, "wipe", assetHex, "00", "02", hex.EncodeToString(testWalletAddress2Hex))

	balancePostWipe, err := common.GetBalanceOnNode(testWalletAddress2, assetStr, 10)
	if err != nil {
		log.Fatalln(err)
	}

	if balancePostWipe != 3 {
		log.Fatalln("balance after wipe error. ", balancePostWipe, " != ", 3)
	}

	invokeContractCall("Pause KDA", args.KeyFile, nil, "pause", assetHex)

	_, asset, err = common.GetAssetOnNode(assetStr, 10)
	if err != nil {
		log.Fatalln(err)
	}

	if !asset.Attributes.IsPaused {
		log.Fatalln("asset is not paused.")
	}

	invokeContractCall("Resume KDA", args.KeyFile, nil, "resume", assetHex)

	_, asset, err = common.GetAssetOnNode(assetStr, 10)
	if err != nil {
		log.Fatalln(err)
	}

	if asset.Attributes.IsPaused {
		log.Fatalln("asset is not resumed.")
	}

	invokeContractCall("Add Role KDA", args.KeyFile, nil, "add_role", assetHex, hex.EncodeToString(testWalletAddress2Hex), "00", "01", "00", "00")

	_, asset, err = common.GetAssetOnNode(assetStr, 10)
	if err != nil {
		log.Fatalln(err)
	}

	for _, role := range asset.Roles {
		if hex.EncodeToString(role.Address) != hex.EncodeToString(testWalletAddress2Hex) {
			log.Fatalln("role is not added.")
		}
	}

	invokeContractCall("Remove Role KDA", args.KeyFile, nil, "remove_role", assetHex, hex.EncodeToString(testWalletAddress2Hex))

	_, asset, err = common.GetAssetOnNode(assetStr, 10)
	if err != nil {
		log.Fatalln(err)
	}

	if len(asset.Roles) != 0 {
		log.Fatalln("role is not removed.")
	}

	invokeContractCall("Update Fee Pool", args.KeyFile, nil, "update_fee_pool", assetHex, "01", hex.EncodeToString(CONTRACT_ADDRESS_HEX), "05", "05")

	invokeContractCall("Deposit", args.KeyFile, []Values{{"KLV", 1000000000}}, "deposit", "01", assetHex, KLVHex, "3B9ACA00")

	invokeContractCall("Update Staking", args.KeyFile, nil, "update_staking", assetHex, "00", "05", "01", "01", "01")

	invokeContractCall("Change Owner KDA", args.KeyFile, nil, "change_owner", assetHex, hex.EncodeToString(testWalletAddress2Hex))

	_, asset, err = common.GetAssetOnNode(assetStr, 10)
	if err != nil {
		log.Fatalln(err)
	}

	if hex.EncodeToString(asset.OwnerAddress) != hex.EncodeToString(testWalletAddress2Hex) {
		log.Fatalln("asset is not resumed.")
	}
}

func TriggerKLVTransactions(args common.TestArgs) {

	freeze := invokeContractCall("Freeze", args.KeyFile, []Values{{"KLV", 1000000000}}, "freeze", KLVHex, "3B9ACA00")
	bucketID := hex.EncodeToString(freeze.Receipts[len(freeze.Receipts)-1].Data[1])

	invokeContractCall("Delegate", args.KeyFile, nil, "delegate", hex.EncodeToString(delegateWalletAddressHex), bucketID)

	duration, err := common.GetRemainingTimeToEpoch(1)
	if err != nil {
		log.Fatalln(err)
	}

	fmt.Println("---------------------------")
	fmt.Println("Waiting for the next epoch.")
	fmt.Printf("---------------------------\n\n")

	time.Sleep(duration)

	invokeContractCall("Claim", args.KeyFile, nil, "claim", "01", KLVHex)
	invokeContractCall("Undelegate", args.KeyFile, nil, "undelegate", bucketID)
	invokeContractCall("Unfreeze", args.KeyFile, nil, "unfreeze", KLVHex, bucketID)

	duration, err = common.GetRemainingTimeToEpoch(2)
	if err != nil {
		log.Fatalln(err)
	}

	fmt.Println("-----------------------------")
	fmt.Println("Waiting for the next 2 epochs.")
	fmt.Printf("-----------------------------\n\n")

	time.Sleep(duration)

	invokeContractCall("Withdraw", args.KeyFile, nil, "withdraw", "00", KLVHex, "00", KLVHex)
}

func TriggerNFTTransactions(args common.TestArgs) {
	createKDATx := invokeContractCall("Create NFT", args.KeyFile, nil, "kda_create_nft")

	assetBytes := createKDATx.Receipts[len(createKDATx.Receipts)-1].Data[1]
	assetHex := hex.EncodeToString(assetBytes)

	invokeContractCall("Mint NFT", args.KeyFile, nil, "mint", assetHex, "01")
	invokeContractCall("Update Metadata", args.KeyFile, nil, "update_metadata", assetHex, "01", hex.EncodeToString(CONTRACT_ADDRESS_HEX), "6170706c69636174696f6e2f6a736f6e", "7b2274657374223a2022313233227d")
	invokeContractCall("Stop NFT Mint", args.KeyFile, nil, "stop_nft_mint", assetHex)
	invokeContractCall("Update Logo", args.KeyFile, nil, "update_logo", assetHex, "68747470733a2f2f73746f726167652e676f6f676c65617069732e636f6d2f6465786265742f6c6f676f2e706e67")

	mktId := createMarketplace(args.KeyFile)

	nextMonthDate := time.Now().Add(time.Hour * 24 * 30).Unix()
	parsedNextMonthDate := strings.ToUpper(fmt.Sprintf("%x", nextMonthDate))

	sell := invokeContractCall("Sell NFT", args.KeyFile, nil, "sell", "01", mktId, assetHex, "01", KLVHex, "50", "10", parsedNextMonthDate)
	sellId := hex.EncodeToString(sell.Receipts[len(sell.Receipts)-1].Data[1])

	invokeContractCall("Buy NFT", args.KeyFile, []Values{{"KLV", 80}}, "buy", "01", sellId, KLVHex, "50")
	invokeContractCall("Update URIs", args.KeyFile, nil, "update_uris", assetHex)

	invokeContractCall("Change Royaties Receiver", args.KeyFile, nil, "change_royalties_receiver", assetHex, hex.EncodeToString(testWalletAddress2Hex))
	invokeContractCall("Update Royalties", args.KeyFile, nil, "update_royalties", assetHex, hex.EncodeToString(testWalletAddressHex))
	invokeContractCall("Stop Royalties Change", args.KeyFile, nil, "stop_royalties_change", assetHex)
	invokeContractCall("Stop Metadata Change", args.KeyFile, nil, "stop_metadata_change", assetHex)
}

func TriggerITOTransactions(args common.TestArgs) {
	createKDATx := invokeContractCall("Create NFT", args.KeyFile, nil, "kda_create_nft")

	assetBytes := createKDATx.Receipts[len(createKDATx.Receipts)-1].Data[1]
	assetHex := hex.EncodeToString(assetBytes)

	startDate := strings.ToUpper(fmt.Sprintf("%x", time.Now().Add(time.Hour*24*3).Unix()))
	endDate := strings.ToUpper(fmt.Sprintf("%x", time.Now().Add(time.Hour*24*10).Unix()))

	wlStartDate := strings.ToUpper(fmt.Sprintf("%x", time.Now().Add(time.Hour*24*4).Unix()))
	wlEndDate := strings.ToUpper(fmt.Sprintf("%x", time.Now().Add(time.Hour*24*6).Unix()))

	updateStartDate := strings.ToUpper(fmt.Sprintf("%x", time.Now().Add(time.Hour*24*2).Unix()))
	updateEndDate := strings.ToUpper(fmt.Sprintf("%x", time.Now().Add(time.Hour*24*11).Unix()))
	updateWlStartDate := strings.ToUpper(fmt.Sprintf("%x", time.Now().Add(time.Hour*24*3).Unix()))
	updateWlEndDate := strings.ToUpper(fmt.Sprintf("%x", time.Now().Add(time.Hour*24*7).Unix()))

	invokeContractCall("ITO Config", args.KeyFile, nil, "ito_config", assetHex, hex.EncodeToString(CONTRACT_ADDRESS_HEX), hex.EncodeToString(testWalletAddressHex), startDate, endDate, wlStartDate, wlEndDate)
	invokeContractCall("Set ITO Prices", args.KeyFile, nil, "ito_set_prices", assetHex)
	invokeContractCall("Update ITO Status", args.KeyFile, nil, "ito_update_status", assetHex, "02")
	invokeContractCall("Update ITO Receiver Addr", args.KeyFile, nil, "ito_update_receiver_address", assetHex, hex.EncodeToString(testWalletAddress2Hex))
	invokeContractCall("Update ITO Max Amount", args.KeyFile, nil, "ito_update_max_amount", assetHex, "50")
	invokeContractCall("Update ITO Default Limit", args.KeyFile, nil, "ito_update_default_limit_per_address", assetHex, "50")
	invokeContractCall("Update ITO Times", args.KeyFile, nil, "ito_update_times", assetHex, updateStartDate, updateEndDate)
	invokeContractCall("Update ITO WL Status", args.KeyFile, nil, "ito_update_whitelist_status", assetHex, "02")
	invokeContractCall("Update ITO Add WL", args.KeyFile, nil, "ito_add_to_whitelist", assetHex, hex.EncodeToString(testWalletAddress2Hex))
	invokeContractCall("Update ITO Remove WL", args.KeyFile, nil, "ito_remove_from_whitelist", assetHex, hex.EncodeToString(testWalletAddress2Hex))
	invokeContractCall("Update ITO WL Times", args.KeyFile, nil, "ito_update_whitelist_times", assetHex, updateWlStartDate, updateWlEndDate)
}

type Values struct {
	Asset string
	Value int64
}

func invokeContractCall(name string, keyFile string, values []Values, funcName string, args ...string) *transaction.Transaction {
	strs := []string{funcName}
	strs = append(strs, args...)

	valuesStr := ""
	for _, value := range values {
		valuesStr += fmt.Sprintf("%s=%d", value.Asset, value.Value)
	}

	var err error
	var hash string

	if len(valuesStr) > 0 {
		hash, err = common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", keyFile),
			"sc", "invoke", CONTRACT_ADDRESS, "--message", formatInput(strs...), "--values", valuesStr)
	} else {
		hash, err = common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", keyFile),
			"sc", "invoke", CONTRACT_ADDRESS, "--message", formatInput(strs...))
	}

	if err != nil {
		log.Fatalf("%s(%s) Invoke error:  %+v", name, funcName, err)
	}

	status, tx, err := common.CheckTransactionOnNode(hash, 10)
	if err != nil {
		log.Fatalf("%s(%s) Check error: %+v", name, funcName, err)
	}

	if !status {
		log.Fatalf("%s(%s) Transaction: %s is not success.\n", name, funcName, hash)
	}

	fmt.Printf("%s Hash: %+v receipts: %d\n\n", name, hash, len(tx.Receipts))

	return tx
}

func formatInput(args ...string) string {
	return strings.Join(args, "@")
}

func createMarketplace(keyFile string) string {
	hash, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", keyFile), "kapps",
		"create-marketplace", "--name", "teste", "--referral", testWalletAddress, "--percentage", "0.000001")
	if err != nil {
		log.Fatalln(err)
	}

	status, tx, err := common.CheckTransactionOnNode(hash, 10)
	if err != nil {
		log.Fatalln(err)
	}

	if !status {
		log.Fatalf("Create Marketplace Transaction: %s is not success.\n", hash)
	}

	return hex.EncodeToString(tx.Receipts[len(tx.Receipts)-1].Data[1])
}
