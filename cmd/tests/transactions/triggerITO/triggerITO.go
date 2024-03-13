package triggerITO

import (
	"fmt"
	"log"
	"time"

	"github.com/klever-io/klever-go/cmd/tests/utils"

	"github.com/klever-io/klever-go/cmd/tests/common"
	"github.com/klever-io/klever-go/cmd/tests/common/account"
	"github.com/klever-io/klever-go/data/transaction"
)

func TriggerITOShouldError(args common.TestArgs) {
	_, _, testWalletAddress, err := utils.LoadKey(args.KeyFile)
	if err != nil {
		log.Fatalln(err)
	}

	// create the FPR asset
	createAssetHash, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "kda", "create", "1",
		"--name", "Test", "--ticker", "TST", "--maxSupply", "10000", "--canFreeze", "--canPause", "--canMint", "--canBurn",
		"--canChangeOwner", "--canAddRoles")

	if err != nil {
		log.Fatalln(err)
	}

	status, createAssetTx, err := common.CheckTransaction(createAssetHash, 0)
	if err != nil {
		log.Fatalln(err)
	}

	if status != "Ok" {
		log.Fatalln("tx is not success.")
	}

	assetId, err := common.GetAssetId(createAssetTx)
	if err != nil {
		log.Fatalln(err)
	}

	// config ito
	startTime := time.Now().Add(time.Minute).Unix()
	endTime := time.Now().AddDate(0, 0, 3).Unix()

	whitelistStartTime := time.Now().Add(time.Minute).Unix()
	whitelistEndTime := time.Now().AddDate(0, 0, 3).Unix()

	configITOHash, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "kapps", "config-ito", fmt.Sprintf("--kda=%s", assetId), "--maxAmount=1000", fmt.Sprintf("--receiver=%s", testWalletAddress), "--status=1", `--packs={"kda": "KLV", "packs": [{"amount":1, "price":20},{"amount":10, "price":150}]}`, fmt.Sprintf("--startTime=%d", startTime), fmt.Sprintf("--endTime=%d", endTime), "--whitelist=klv1rsm4vx8pdg2cy74zz9gwff0avl7vwunzy9tv7utn8eey0chtdaasectfw4=10,klv1fpwjz234gy8aaae3gx0e8q9f52vymzzn3z5q0s5h60pvktzx0n0qwvtux5=20", "--whitelistStatus=1", fmt.Sprintf("--whitelistStartTime=%d", whitelistStartTime), fmt.Sprintf("--whitelistEndTime=%d", whitelistEndTime), fmt.Sprintf("--limitPerAddress=%d", 5))
	if err != nil {
		log.Fatalln(err)
	}

	status, _, err = common.CheckTransaction(configITOHash, 0)
	if err != nil {
		log.Fatalln(err)
	}

	if status != "Ok" {
		log.Fatalln("tx is not success.")
	}

	// triggerITO

	// trigger: set ito prices
	// error: invalid amount
	_, err = common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "kapps", "trigger-ito", "0", fmt.Sprintf("--kda=%s", assetId), `--packs={"kda": "KLV", "packs": [{"amount":-1, "price":5},{"amount":10, "price":250}]}`)
	if err == nil {
		log.Fatalln(err)
	}

	// error: invalid price
	_, err = common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "kapps", "trigger-ito", "0", fmt.Sprintf("--kda=%s", assetId),
		`--packs={"kda": "KLV", "packs": [{"amount":1, "price":-5},{"amount":10, "price":250}]}`)
	if err == nil {
		log.Fatalln(err)
	}

	// error: invalid pack asset
	triggerITOInvalidHash, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "kapps", "trigger-ito", "0", fmt.Sprintf("--kda=%s", assetId),
		`--packs={"kda": "INVALID", "packs": [{"amount":1, "price":5},{"amount":10, "price":250}]}`)
	if err != nil {
		log.Fatalln(err)
	}

	_, tx, err := common.CheckTransaction(triggerITOInvalidHash, 0)
	if err != nil {
		log.Fatalln(err)
	}

	if tx.Status != "fail" && tx.ResultCode != transaction.Transaction_KAPPError.String() {
		log.Fatalln("tx with invalid pack asset succeded!")
	}

	// error: user without permission
	// fill account with KLV
	//"klv10vcm82mw30v48rr272s3m6llcs0p09tpp76hztsgnja276nzq4cqw38hgz"
	_, _, addr1, err := utils.LoadKey("wallet-generated-1.pem")
	if err != nil {
		log.Fatalln(err)
	}
	account1, err := account.NewAccount(addr1,
		account.WithKeyFile("wallet-generated-1.pem"))
	if err != nil {
		log.Fatalln(err)
	}

	sendKLVToAccount1Hash, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "account", "send", account1.Address, fmt.Sprint(1000))
	if err != nil {
		log.Fatalln(err)
	}

	status, _, err = common.CheckTransaction(sendKLVToAccount1Hash, 0)
	if err != nil {
		log.Fatalln(err)
	}

	if status != "Ok" {
		log.Fatalln("tx is not success.")
	}

	triggerITOInvalidHash, err = common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", account1.KeyFile), "kapps", "trigger-ito", "0", fmt.Sprintf("--kda=%s", assetId),
		`--packs={"kda": "KLV", "packs": [{"amount":1, "price":5},{"amount":10, "price":250}]}`)
	if err != nil {
		log.Fatalln(err)
	}

	_, tx, err = common.CheckTransaction(triggerITOInvalidHash, 0)
	if err != nil {
		log.Fatalln(err)
	}

	if tx.Status != "fail" && tx.ResultCode != transaction.Transaction_AccountError.String() {
		log.Fatalln("tx with user without permission succeded!")
	}

	// trigger: update ITO status
	// error: invalid status
	_, err = common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "kapps", "trigger-ito", "1",
		fmt.Sprintf("--kda=%s", assetId), "--status=22")
	if err == nil {
		log.Fatalln(err)
	}

	// trigger: update receiver address
	// error: invalid address
	invalidAddress := "INVALID"
	_, err = common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "kapps", "trigger-ito", "2",
		fmt.Sprintf("--kda=%s", assetId), fmt.Sprintf("--receiver=%s", invalidAddress))
	if err == nil {
		log.Fatalln(err)
	}

	// trigger: update max amount
	// err: invalid max amount
	_, err = common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "kapps", "trigger-ito", "3",
		fmt.Sprintf("--kda=%s", assetId), "--maxAmount=-40")
	if err == nil {
		log.Fatalln(err)
	}

	// trigger: update default limit address
	// err: invalid limit per address
	_, err = common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "kapps", "trigger-ito", "4",
		fmt.Sprintf("--kda=%s", assetId), fmt.Sprintf("--limitPerAddress=%d", -2))
	if err == nil {
		log.Fatalln(err)
	}

	// trigger: update ITO times
	// err: invalid ito times
	newStartTime := time.Now().AddDate(0, 0, 3).Unix()
	newEndTime := time.Now().AddDate(0, 0, 2).Unix()

	_, err = common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "kapps", "trigger-ito", "5",
		fmt.Sprintf("--kda=%s", assetId), fmt.Sprintf("--startTime=%d", newStartTime), fmt.Sprintf("--endTime=%d", newEndTime))
	if err == nil {
		log.Fatalln(err)
	}

	// trigger: update whitelist status
	// err: invalid status
	_, err = common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "kapps", "trigger-ito", "6",
		fmt.Sprintf("--kda=%s", assetId), "--whitelistStatus=22")
	if err == nil {
		log.Fatalln(err)
	}

	// trigger: add to whitelist
	// err: invalid address
	invalidWhitelistAddress := "INVALID"
	_, err = common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "kapps", "trigger-ito", "7",
		fmt.Sprintf("--kda=%s", assetId), fmt.Sprintf("--whitelist=%s=10", invalidWhitelistAddress))
	if err == nil {
		log.Fatalln(err)
	}

	// trigger: remove from whitelist
	// err: invalid address
	_, err = common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "kapps", "trigger-ito", "8",
		fmt.Sprintf("--kda=%s", assetId), fmt.Sprintf("--whitelist=%s=10", invalidWhitelistAddress))
	if err == nil {
		log.Fatalln(err)
	}

	// trigger: update whitelist times
	newWhitelistStartTime := time.Now().AddDate(0, 0, 3).Unix()
	newWhitelistEndTime := time.Now().AddDate(0, 0, 2).Unix()

	_, err = common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "kapps", "trigger-ito", "9",
		fmt.Sprintf("--kda=%s", assetId), fmt.Sprintf("--whitelistStartTime=%d", newWhitelistStartTime), fmt.Sprintf("--whitelistEndTime=%d", newWhitelistEndTime))
	if err == nil {
		log.Fatalln(err)
	}

	fmt.Println("----- All tests passed! ----")
}

func TriggerITOShouldWork(args common.TestArgs) {
	_, _, testWalletAddress, err := utils.LoadKey(args.KeyFile)
	if err != nil {
		log.Fatalln(err)
	}

	// create the FPR asset
	createAssetHash, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "kda", "create", "1",
		"--name", "Test", "--ticker", "TST", "--maxSupply", "10000", "--canFreeze", "--canPause", "--canMint", "--canBurn",
		"--canChangeOwner", "--canAddRoles")

	if err != nil {
		log.Fatalln(err)
	}

	status, createAssetTx, err := common.CheckTransaction(createAssetHash, 0)
	if err != nil {
		log.Fatalln(err)
	}

	if status != "Ok" {
		log.Fatalln("tx is not success.")
	}

	assetId, err := common.GetAssetId(createAssetTx)
	if err != nil {
		log.Fatalln(err)
	}

	// config ito
	startTime := time.Now().Add(time.Minute).Unix()
	endTime := time.Now().AddDate(0, 0, 3).Unix()

	whitelistStartTime := time.Now().Add(time.Minute).Unix()
	whitelistEndTime := time.Now().AddDate(0, 0, 3).Unix()

	configITOHash, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "kapps", "config-ito", fmt.Sprintf("--kda=%s", assetId), "--maxAmount=1000",
		fmt.Sprintf("--receiver=%s", testWalletAddress), "--status=1", `--packs={"kda": "KLV", "packs": [{"amount":1, "price":20},{"amount":10, "price":150}]}`, fmt.Sprintf("--startTime=%d", startTime),
		fmt.Sprintf("--endTime=%d", endTime), "--whitelist=klv1rsm4vx8pdg2cy74zz9gwff0avl7vwunzy9tv7utn8eey0chtdaasectfw4=10,klv1fpwjz234gy8aaae3gx0e8q9f52vymzzn3z5q0s5h60pvktzx0n0qwvtux5=20",
		"--whitelistStatus=1", fmt.Sprintf("--whitelistStartTime=%d", whitelistStartTime), fmt.Sprintf("--whitelistEndTime=%d", whitelistEndTime), fmt.Sprintf("--limitPerAddress=%d", 5))
	if err != nil {
		log.Fatalln(err)
	}

	status, _, err = common.CheckTransaction(configITOHash, 0)
	if err != nil {
		log.Fatalln(err)
	}

	if status != "Ok" {
		log.Fatalln("tx is not success.")
	}

	// triggerITO

	// trigger: set ito prices
	triggerITOSetPriceHash, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "kapps", "trigger-ito", "0", fmt.Sprintf("--kda=%s", assetId),
		`--packs={"kda": "KLV", "packs": [{"amount":1, "price":5},{"amount":10, "price":250}]}`)
	if err != nil {
		log.Fatalln(err)
	}

	status, _, err = common.CheckTransaction(triggerITOSetPriceHash, 0)
	if err != nil {
		log.Fatalln(err)
	}

	if status != "Ok" {
		log.Fatalln("tx is not success.")
	}

	// trigger: update ITO status
	triggerITOUpdateStatusHash, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "kapps", "trigger-ito", "1", fmt.Sprintf("--kda=%s", assetId), "--status=1")
	if err != nil {
		log.Fatalln(err)
	}

	status, _, err = common.CheckTransaction(triggerITOUpdateStatusHash, 0)
	if err != nil {
		log.Fatalln(err)
	}

	if status != "Ok" {
		log.Fatalln("tx is not success.")
	}

	// trigger: update receiver address
	newReceiverAddress := "klv1ec7vltrwy7sqxv8sted4xmmjuczazsc40qh3ncvendyx9qhevxlseywjtm"
	triggerITOUpdateReceiverHash, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "kapps", "trigger-ito", "2", fmt.Sprintf("--kda=%s", assetId), fmt.Sprintf("--receiver=%s", newReceiverAddress))
	if err != nil {
		log.Fatalln(err)
	}

	status, _, err = common.CheckTransaction(triggerITOUpdateReceiverHash, 0)
	if err != nil {
		log.Fatalln(err)
	}

	if status != "Ok" {
		log.Fatalln("tx is not success.")
	}

	// trigger: update max amount
	triggerITOUpdateMaxAmountHash, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "kapps", "trigger-ito", "3", fmt.Sprintf("--kda=%s", assetId), "--maxAmount=40")
	if err != nil {
		log.Fatalln(err)
	}

	status, _, err = common.CheckTransaction(triggerITOUpdateMaxAmountHash, 0)
	if err != nil {
		log.Fatalln(err)
	}

	if status != "Ok" {
		log.Fatalln("tx is not success.")
	}

	// trigger: update default limit address
	triggerITOUpdateDefaultLimitHash, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "kapps", "trigger-ito", "4", fmt.Sprintf("--kda=%s", assetId), fmt.Sprintf("--limitPerAddress=%d", 2))
	if err != nil {
		log.Fatalln(err)
	}

	status, _, err = common.CheckTransaction(triggerITOUpdateDefaultLimitHash, 0)
	if err != nil {
		log.Fatalln(err)
	}

	if status != "Ok" {
		log.Fatalln("tx is not success.")
	}

	// trigger: update ITO times
	newStartTime := time.Now().AddDate(0, 0, 2).Unix()
	newEndTime := time.Now().AddDate(0, 0, 3).Unix()

	triggerITOUpdateTimesHash, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "kapps", "trigger-ito", "5", fmt.Sprintf("--kda=%s", assetId),
		fmt.Sprintf("--startTime=%d", newStartTime), fmt.Sprintf("--endTime=%d", newEndTime))

	if err != nil {
		log.Fatalln(err)
	}

	status, _, err = common.CheckTransaction(triggerITOUpdateTimesHash, 0)
	if err != nil {
		log.Fatalln(err)
	}

	if status != "Ok" {
		log.Fatalln("tx is not success.")
	}

	// trigger: update whitelist status
	triggerITOUpdateWhitelistStatusHash, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "kapps", "trigger-ito", "6", fmt.Sprintf("--kda=%s", assetId), "--whitelistStatus=1")
	if err != nil {
		log.Fatalln(err)
	}

	status, _, err = common.CheckTransaction(triggerITOUpdateWhitelistStatusHash, 0)
	if err != nil {
		log.Fatalln(err)
	}

	if status != "Ok" {
		log.Fatalln("tx is not success.")
	}

	// trigger: add to whitelist
	newWhitelistAddress := "klv17e8zzgn73h6ehe3c6q9vlt77kuxk5euddmhymy5uhv2rhv0dc0nqlfp0ap"

	triggerITOAddToWhitelistHash, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "kapps", "trigger-ito", "7", fmt.Sprintf("--kda=%s", assetId),
		fmt.Sprintf("--whitelist=%s=10", newWhitelistAddress))

	if err != nil {
		log.Fatalln(err)
	}

	status, _, err = common.CheckTransaction(triggerITOAddToWhitelistHash, 0)
	if err != nil {
		log.Fatalln(err)
	}

	if status != "Ok" {
		log.Fatalln("tx is not success.")
	}

	// trigger: remove from whitelist
	removedWhitelistAddress := "klv1rsm4vx8pdg2cy74zz9gwff0avl7vwunzy9tv7utn8eey0chtdaasectfw4"

	triggerITORemoveFromWhitelistHash, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "kapps", "trigger-ito", "8", fmt.Sprintf("--kda=%s", assetId),
		fmt.Sprintf("--whitelist=%s=10", removedWhitelistAddress))

	if err != nil {
		log.Fatalln(err)
	}

	status, _, err = common.CheckTransaction(triggerITORemoveFromWhitelistHash, 0)
	if err != nil {
		log.Fatalln(err)
	}

	if status != "Ok" {
		log.Fatalln("tx is not success.")
	}

	// trigger: update whitelist times
	newWhitelistStartTime := time.Now().AddDate(0, 0, 2).Unix()
	newWhitelistEndTime := time.Now().AddDate(0, 0, 3).Unix()

	triggerITOUpdateWhitelistTimesHash, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "kapps", "trigger-ito", "9", fmt.Sprintf("--kda=%s", assetId),
		fmt.Sprintf("--whitelistStartTime=%d", newWhitelistStartTime), fmt.Sprintf("--whitelistEndTime=%d", newWhitelistEndTime))

	if err != nil {
		log.Fatalln(err)
	}

	status, _, err = common.CheckTransaction(triggerITOUpdateWhitelistTimesHash, 0)
	if err != nil {
		log.Fatalln(err)
	}

	if status != "Ok" {
		log.Fatalln("tx is not success.")
	}

	//checks

	// get ito
	ito, err := common.GetITO(assetId)
	if err != nil {
		log.Fatalln(err)
	}

	// check: set ITO price
	if ito.PackData[0].Packs[0].Price != int64(5)*1000000 {
		log.Fatalln("new pack data is incorrect!")
	}

	// check: update status
	if !ito.IsActive {
		log.Fatalln("ito status is incorrect!")
	}

	// check: update receiver address
	if ito.ReceiverAddress != newReceiverAddress {
		log.Fatalln("new receiver address is incorrect!")
	}

	// check: update max amount
	if ito.MaxAmount != int64(40) {
		log.Fatalln("new max amount is incorrect!")
	}

	// check: update default limit address
	if ito.DefaultLimitPerAddress != int64(2) {
		log.Fatalln("new default limit address is incorrect!")
	}

	// check: update times
	if ito.StartTime != newStartTime || ito.EndTime != newEndTime {
		log.Fatalln("new ito times is incorrect!")
	}

	// check: add to whitelist
	addressFound := false
	for _, w := range ito.WhitelistInfo {
		if w.Address == newWhitelistAddress {
			addressFound = true
		}
	}
	if !addressFound {
		log.Fatalln("whitelist addresses is incorrect!")
	}

	// check: remove from whitelist
	addressFound = false
	for _, w := range ito.WhitelistInfo {
		if w.Address == removedWhitelistAddress {
			addressFound = true
		}
	}
	if addressFound {
		log.Fatalln("whitelist addresses is incorrect!")
	}

	// check: update whitelist time
	if ito.WhitelistStartTime != newWhitelistStartTime || ito.WhitelistEndTime != newWhitelistEndTime {
		log.Fatalln("new ito whitelist times is incorrect!")
	}

	// check: update whitelist status
	if !ito.IsWhitelistActive {
		log.Fatalln("ito whitelist status is incorrect!")
	}
}

func TriggerITOWithoutConfigShouldError(args common.TestArgs) {
	// create asset
	createAssetHash, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "kda", "create", "1",
		"--name", "Test", "--ticker", "TST", "--maxSupply", "10000", "--canFreeze", "--canPause", "--canMint", "--canBurn",
		"--canChangeOwner", "--canAddRoles")

	if err != nil {
		log.Fatalln(err)
	}

	status, createAssetTx, err := common.CheckTransaction(createAssetHash, 0)
	if err != nil {
		log.Fatalln(err)
	}

	if status != "Ok" {
		log.Fatalln("tx is not success.")
	}

	assetId, err := common.GetAssetId(createAssetTx)
	if err != nil {
		log.Fatalln(err)
	}

	triggerITOUpdateStatusHash, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "kapps", "trigger-ito", "1", fmt.Sprintf("--kda=%s", assetId), "--status=2")
	if err != nil {
		log.Fatalln(err)
	}

	status, _, err = common.CheckTransaction(triggerITOUpdateStatusHash, 0)
	if err != nil {
		log.Fatalln(err)
	}

	if status == "Ok" {
		log.Fatalln("tx is success but should not be.")
	}
}

func RunTests(args common.TestArgs) {
	TriggerITOWithoutConfigShouldError(args)
	TriggerITOShouldWork(args)
	TriggerITOShouldError(args)
}
