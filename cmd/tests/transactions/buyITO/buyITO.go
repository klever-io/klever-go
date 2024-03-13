package buyITO

import (
	"fmt"
	"log"
	"time"

	"github.com/klever-io/klever-go/cmd/tests/utils"

	"github.com/klever-io/klever-go/cmd/tests/common"
	"github.com/klever-io/klever-go/cmd/tests/common/account"
	"github.com/klever-io/klever-go/data/transaction"
	// "github.com/klever-io/klever-go/data/transaction"
)

func BuyITOShouldError(args common.TestArgs) {
	amount := 1000

	_, _, testWalletAddress, err := utils.LoadKey(args.KeyFile)
	if err != nil {
		log.Fatalln(err)
	}

	_, _, addr1, err := utils.LoadKey("wallet-generated-1.pem")
	if err != nil {
		log.Fatalln(err)
	}

	// add buyers account
	account1, err := account.NewAccount(addr1,
		account.WithKeyFile("wallet-generated-1.pem"))
	if err != nil {
		log.Fatalln(err)
	}

	_, _, addr2, err := utils.LoadKey("wallet-generated-2.pem")
	if err != nil {
		log.Fatalln(err)
	}

	account2, err := account.NewAccount(addr2,
		account.WithKeyFile("wallet-generated-2.pem"))
	if err != nil {
		log.Fatalln(err)
	}

	// send the klv to the accounts
	sendAsset1, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "account", "send", account1.Address, fmt.Sprint(amount))
	if err != nil {
		log.Fatalln(err)
	}

	sendAsset2, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "account", "send", account2.Address, fmt.Sprint(amount))
	if err != nil {
		log.Fatalln(err)
	}

	statuses, _, err := common.CheckTransactions(0, sendAsset1, sendAsset2)
	if err != nil {
		log.Fatalln(err)
	}

	if statuses[0] != "Ok" || statuses[1] != "Ok" {
		log.Fatalf("send KLV transactions is not ok. Tx1: %s, Tx2: %s\n", statuses[0], statuses[1])
	}

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
	// config ito
	startTime := time.Now().Add(15 * time.Second).UnixMilli()
	endTime := time.Now().AddDate(0, 0, 3).UnixMilli()

	whitelistStartTime := time.Now().Add(30 * time.Second).UnixMilli()
	whitelistEndTime := time.Now().AddDate(0, 0, 3).UnixMilli()

	configITOHash, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "kapps", "config-ito",
		fmt.Sprintf("--kda=%s", assetId), "--maxAmount=1", fmt.Sprintf("--receiver=%s", testWalletAddress), "--status=1",
		`--packs={"kda": "KLV", "packs": [{"amount":1, "price":20},{"amount":10, "price":150}]}`, fmt.Sprintf("--startTime=%d", startTime),
		fmt.Sprintf("--endTime=%d", endTime), fmt.Sprintf("--whitelist=%s=20", addr2),
		"--whitelistStatus=1", fmt.Sprintf("--whitelistStartTime=%d", whitelistStartTime), fmt.Sprintf("--whitelistEndTime=%d", whitelistEndTime), fmt.Sprintf("--limitPerAddress=%d", 1))
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

	// buyITO
	// error: buy ITO out of time
	buyITOHash, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", account2.KeyFile), "kapps", "buy", fmt.Sprintf("--kda=%s", "KLV"), fmt.Sprintf("--id=%s", assetId), "--amount=1", "--type=0")
	if err != nil {
		log.Fatalln(err)
	}

	_, tx, err := common.CheckTransaction(buyITOHash, 0)
	if err != nil {
		log.Fatalln(err)
	}

	if tx.Status != "fail" && tx.ResultCode != transaction.Transaction_AccountError.String() {
		log.Fatalln("buy ITO tx with inactive ITO succeded!")
	}

	time.Sleep(time.Until(time.UnixMilli(startTime)))

	// error: buy with wrong ITO asset
	_, err = common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", account2.KeyFile), "kapps", "buy", fmt.Sprintf("--kda=%s", "KLV"), fmt.Sprintf("--id=%s", "INVALID"), "--amount=1", "--type=0")
	if err != nil {
		log.Fatalln(err)
	}

	_, tx, err = common.CheckTransaction(buyITOHash, 0)
	if err != nil {
		log.Fatalln(err)
	}

	if tx.Status != "fail" && tx.ResultCode != transaction.Transaction_KAPPError.String() {
		log.Fatalln("buy ITO tx with wrong asset succeded!")
	}

	// trigger ITO to pause
	triggerITOUpdateStatusHash, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "kapps", "trigger-ito", "1", fmt.Sprintf("--kda=%s", assetId), "--status=2")
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

	// error: buy when ITO is paused
	buyITOHash, err = common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", account2.KeyFile), "kapps", "buy", fmt.Sprintf("--kda=%s", "KLV"), fmt.Sprintf("--id=%s", assetId), "--amount=1", "--type=0")
	if err != nil {
		log.Fatalln(err)
	}

	_, tx, err = common.CheckTransaction(buyITOHash, 0)
	if err != nil {
		log.Fatalln(err)
	}

	if tx.Status != "fail" && tx.ResultCode != transaction.Transaction_ITONotActive.String() {
		log.Fatalln("buy ITO tx with paused ITO succeded!")
	}

	// trigger ITO to make it active again
	triggerITOUpdateStatusHash, err = common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "kapps", "trigger-ito", "1", fmt.Sprintf("--kda=%s", assetId), "--status=1")
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

	// error: buy ITO with an address out of the whitelist
	buyITOHash, err = common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", account1.KeyFile), "kapps", "buy", fmt.Sprintf("--kda=%s", "KLV"), fmt.Sprintf("--id=%s", assetId), "--amount=1", "--type=0")
	if err != nil {
		log.Fatalln(err)
	}

	_, tx, err = common.CheckTransaction(buyITOHash, 0)
	if err != nil {
		log.Fatalln(err)
	}

	if tx.Status != "fail" && tx.ResultCode != transaction.Transaction_ITONotActive.String() {
		log.Fatalln("buy ITO tx with paused ITO succeded!")
	}

	// error: buy when asset minted amount is higher then max ITO amount

	// buy ITO to reach limit
	buyITOHash, err = common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", account1.KeyFile), "kapps", "buy", fmt.Sprintf("--kda=%s", "KLV"), fmt.Sprintf("--id=%s", assetId), "--amount=1", "--type=0")
	if err != nil {
		log.Fatalln(err)
	}

	_, tx, err = common.CheckTransaction(buyITOHash, 0)
	if err != nil {
		log.Fatalln(err)
	}

	if tx.Status != "fail" && tx.ResultCode != transaction.Transaction_ParameterInvalid.String() {
		log.Fatalln("buy ITO tx with minted amount higher then max ITO amount succeded!")
	}

	// buy ITO with limit reached
	buyITOHash, err = common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", account1.KeyFile), "kapps", "buy", fmt.Sprintf("--kda=%s", "KLV"), fmt.Sprintf("--id=%s", assetId), "--amount=1", "--type=0")
	if err != nil {
		log.Fatalln(err)
	}

	_, tx, err = common.CheckTransaction(buyITOHash, 0)
	if err != nil {
		log.Fatalln(err)
	}

	if tx.Status != "fail" && tx.ResultCode != transaction.Transaction_ITONotActive.String() {
		log.Println("2")
		log.Println("buy ITO tx with paused ITO succeded!")
	}
}

func RunTests(args common.TestArgs) {
	BuyITOShouldError(args)
}
