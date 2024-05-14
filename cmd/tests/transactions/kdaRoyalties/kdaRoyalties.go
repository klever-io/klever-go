package kdaRoyalties

import (
	"errors"
	"fmt"
	"log"

	"github.com/klever-io/klever-go/cmd/tests/utils"

	"github.com/klever-io/klever-go/cmd/tests/common"
	"github.com/klever-io/klever-go/cmd/tests/common/account"
)

func syncAccounts(accs ...*account.Account) {
	for _, a := range accs {
		if err := a.Sync(); err != nil {
			log.Fatalln("cannot sync account")
		}
	}
}

// CreateKdaRoyalties Should create a KDA with splitted royalties, send, sell, buy and validate rewards distribution
func CreateKdaRoyalties(args common.TestArgs) {
	fmt.Println("Running KDA Royalties Test")
	// bootstrap the test accounts
	_, _, addr1, err := utils.LoadKey("wallet-generated-1.pem")
	if err != nil {
		log.Fatalln(err)
	}

	account1, err := account.NewAccount(addr1,
		account.WithSync(), account.WithKeyFile("wallet-generated-1.pem"))
	if err != nil {
		log.Fatalln(err)
	}

	_, _, addr2, err := utils.LoadKey("wallet-generated-2.pem")
	if err != nil {
		log.Fatalln(err)
	}

	account2, err := account.NewAccount(addr2,
		account.WithSync(), account.WithKeyFile("wallet-generated-2.pem"))
	if err != nil {
		log.Fatalln(err)
	}

	_, _, addr3, err := utils.LoadKey("walletKey.pem")
	if err != nil {
		log.Fatalln(err)
	}

	account3, err := account.NewAccount(addr3,
		account.WithSync(), account.WithKeyFile("walletKey.pem"))
	if err != nil {
		log.Fatalln(err)
	}

	// create the royalties NFT
	createHash, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "kda", "create", "1",
		"--name", "TestNFT", "--ticker", "TSTNFT", "--initialSupply", "0", "--maxSupply", "2000",
		"--canFreeze", "--canPause", "--canMint", "--canBurn", "--canChangeOwner", "--canAddRoles",
		"--royaltiesTransferFixed", "10", "--royaltiesMarketFixed", "11", "--royaltiesMarketPercentage", "10",
		"--royaltiesITOFixed", "12", "--royaltiesITOPercentage", "15",
		"--royaltiesAddress", account3.Address,
		"--splitRoyalties", fmt.Sprintf("{\"address\": \"%s\", \"percentTransferFixed\": 20, \"percentMarketFixed\": 27, \"percentMarketPercentage\": 24, \"percentITOFixed\": 19, \"percentITOPercentage\": 30}", addr1),
		"--splitRoyalties", fmt.Sprintf("{\"address\": \"%s\", \"percentTransferFixed\": 30, \"percentMarketFixed\": 17, \"percentMarketPercentage\": 33, \"percentITOFixed\": 17, \"percentITOPercentage\": 32}", addr2))
	if err != nil {
		log.Fatalln(err)
	}

	status, tx, err := common.CheckTransaction(createHash, 0)
	if err != nil {
		log.Fatalln(err)
	}

	if status != "Ok" {
		log.Fatalln("tx is not success.")
	}

	assetId, err := common.GetAssetId(tx)
	if err != nil {
		log.Fatalln(err)
	}

	// mint NFTS to the accounts
	mintAsset1, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "kda", "trigger", "0", "--kdaID", assetId, "--amount", "1", "--receiver", account1.Address)
	if err != nil {
		log.Fatalln(err)
	}

	mintAsset2, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "kda", "trigger", "0", "--kdaID", assetId, "--amount", "1", "--receiver", account2.Address)
	if err != nil {
		log.Fatalln(err)
	}

	statuses, _, err := common.CheckTransactions(0, mintAsset1, mintAsset2)
	if err != nil {
		log.Fatalln(err)
	}

	if statuses[0] != "Ok" || statuses[1] != "Ok" {
		log.Fatalf("mint kda royalties transactions is not ok. Tx1: %s, Tx2: %s\n", statuses[0], statuses[1])
	}

	amount := 10000

	// // Send the created asset to the accounts
	sendAsset1, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "account", "send", account1.Address, fmt.Sprint(amount),
		"--kda", "KLV")
	if err != nil {
		log.Fatalln(err)
	}

	sendAsset2, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "account", "send", account2.Address, fmt.Sprint(amount),
		"--kda", "KLV")
	if err != nil {
		log.Fatalln(err)
	}

	sendStatuses, _, err := common.CheckTransactions(0, sendAsset1, sendAsset2)
	if err != nil {
		log.Fatalln(err)
	}

	if sendStatuses[0] != "Ok" || sendStatuses[1] != "Ok" {
		log.Fatalf("send FPR transactions is not ok. Tx1: %s, Tx2: %s\n", statuses[0], statuses[1])
	}

	syncAccounts(account1, account2, account3)

	initialBalanceAccount1 := account1.Data.Balance
	initialBalanceAccount2 := account2.Data.Balance
	initialBalanceAccount3 := account3.Data.Balance

	sendAssetHash, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", account1.KeyFile), "account", "send", account2.Address, "1",
		"--kda", assetId+"/1")
	if err != nil {
		log.Fatalln(err)
	}

	status, _, err = common.CheckTransaction(sendAssetHash, 0)
	if err != nil {
		log.Fatalln(err)
	}

	if status != "Ok" {
		log.Fatalln("tx is not success.")
	}

	syncAccounts(account1, account2, account3)

	sendFee := int64(1_500_000)

	//Check Balances after Transfer fixed:
	//(initial balanace - fee - royalties fixed + 2 royalties fixed split)
	if initialBalanceAccount1-sendFee-10_000_000+2_000_000 != account1.Data.Balance {
		fmt.Println("Account 1 is not ok: expect - current: ", initialBalanceAccount1-sendFee-10_000_000+2_000_000, account1.Data.Balance)
	}

	//(initial balanace + 3 royalties fixed split)
	if initialBalanceAccount2+3_000_000 != account2.Data.Balance {
		fmt.Println("Account 2 is not ok: expect - current: ", initialBalanceAccount2+3_000_000, account2.Data.Balance)
	}

	//(initial balanace + 5 royalties fixed split)
	if initialBalanceAccount3+5_000_000 != account3.Data.Balance {
		fmt.Println("Account 3 is not ok: expect - current: ", initialBalanceAccount3+5_000_000, account3.Data.Balance)
	}

	// start market tests
	createMarketplace, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "kapps", "create-marketplace", "--name", "the-mkpt", "--percentage", "0", "--referral", addr3)
	if err != nil {
		log.Fatalln(err)
	}

	status, tx, err = common.CheckTransaction(createMarketplace, 0)
	if err != nil {
		log.Fatalln(err)
	}

	if status != "Ok" {
		log.Fatalln("tx is not success.")
	}

	mktId, err := common.GetMarketplaceId(tx)
	if err != nil {
		log.Fatalln("cannot get marketplace Id: ", err)
	}

	syncAccounts(account1, account2, account3)

	initialBalanceAccount1 = account1.Data.Balance
	initialBalanceAccount2 = account2.Data.Balance
	initialBalanceAccount3 = account3.Data.Balance

	sellHash, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", account2.KeyFile), "kapps", "sell", "--kda", assetId+"/1",
		"--currency", "KLV", "--market", mktId, "--price", "100")
	if err != nil {
		log.Fatalln(err)
	}

	status, tx, err = common.CheckTransaction(sellHash, 0)
	if err != nil {
		log.Fatalln(err)
	}

	if status != "Ok" {
		log.Fatalln("tx is not success.")
	}

	orderId, err := common.GetOrderId(tx)
	if err != nil {
		log.Fatalln("cannot get order Id: ", err)
	}

	syncAccounts(account1, account2, account3)

	// should not change
	if initialBalanceAccount1 != account1.Data.Balance {
		fmt.Println("Account 1 is not ok: expect - current: ", initialBalanceAccount1, account1.Data.Balance)
	}

	//(initial balance - sell fee - marketFixed)
	if initialBalanceAccount2-11_000_000-11_000_000 != account2.Data.Balance {
		fmt.Println("Account 2 is not ok: expect - current: ", initialBalanceAccount2-11_000_000-11_000_000, account2.Data.Balance)
	}

	// should not change
	if initialBalanceAccount3 != account3.Data.Balance {
		fmt.Println("Account 3 is not ok: expect - current: ", initialBalanceAccount3, account3.Data.Balance)
	}

	initialBalanceAccount1 = account1.Data.Balance
	initialBalanceAccount2 = account2.Data.Balance
	initialBalanceAccount3 = account3.Data.Balance

	buyHash, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", account1.KeyFile), "kapps", "buy", "--id", orderId,
		"--kda", "KLV", "--type", "1", "--amount", "100")
	if err != nil {
		log.Fatalln(err)
	}

	status, _, err = common.CheckTransaction(buyHash, 0)
	if err != nil {
		log.Fatalln(err)
	}

	if status != "Ok" {
		log.Fatalln("tx is not success.")
	}

	syncAccounts(account1, account2, account3)

	buyAmount := int64(100_000_000)
	marketPercentage := int64(float64(buyAmount) * 0.10)
	marketFixed := int64(float64(buyAmount) * 0.11)

	// (initial balance - buy fee - buy amount) + market percentage split + market fixed split)
	if initialBalanceAccount1-2_000_000-buyAmount+int64(float64(marketPercentage)*0.24)+int64(float64(marketFixed)*0.27) != account1.Data.Balance {
		fmt.Println("Account 1 is not ok: expect - current: ", initialBalanceAccount1-2_000_000-buyAmount+int64(float64(marketPercentage)*0.24)+int64(float64(marketFixed)*0.27), account1.Data.Balance)
	}

	// (initial balance - market percentage + buy amount  + market percentage split + market fixed split)
	if initialBalanceAccount2+buyAmount-marketPercentage+int64(float64(marketPercentage)*0.33)+int64(float64(marketFixed)*0.17) != account2.Data.Balance {
		fmt.Println("Account 2 is not ok: expect - current: ", initialBalanceAccount2+buyAmount-marketPercentage+int64(float64(marketPercentage)*0.33)+int64(float64(marketFixed)*0.17), account2.Data.Balance)
	}

	// (initial balance + market percentage split + market fixed split)
	if initialBalanceAccount3+int64(float64(marketPercentage)*0.43)+int64(float64(marketFixed)*0.56) != account3.Data.Balance {
		fmt.Println("Account 3 is not ok: expect - current: ", initialBalanceAccount3+int64(float64(marketPercentage)*0.43)+int64(float64(marketFixed)*0.56), account3.Data.Balance)
	}

	// start ito tests
	configITO, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "kapps", "config-ito", "--kda", assetId,
		"--maxAmount", "100", "--status", "1", "--packs", "{\"kda\": \"KLV\", \"packs\": [{\"amount\": 1, \"price\": 100}, {\"amount\": 10, \"price\": 500}]}", "--receiver", account3.Address)
	if err != nil {
		log.Fatalln(err)
	}

	status, _, err = common.CheckTransaction(configITO, 0)
	if err != nil {
		log.Fatalln(err)
	}

	if status != "Ok" {
		log.Fatalln("tx is not success.")
	}

	syncAccounts(account1, account2, account3)
	initialBalanceAccount1 = account1.Data.Balance
	initialBalanceAccount2 = account2.Data.Balance
	initialBalanceAccount3 = account3.Data.Balance

	buyITOHash, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", account2.KeyFile), "kapps", "buy", "--id", assetId,
		"--kda", "KLV", "--amount", "1")
	if err != nil {
		log.Fatalln(err)
	}

	status, _, err = common.CheckTransaction(buyITOHash, 0)
	if err != nil {
		log.Fatalln(err)
	}

	if status != "Ok" {
		log.Fatalln("tx is not success.")
	}

	syncAccounts(account1, account2, account3)

	buyITOAmount := int64(100_000_000)
	itoPercentage := int64(float64(buyITOAmount) * 0.15)
	itoFixed := int64(float64(buyITOAmount) * 0.12)

	// (initial balance - buy fee - buy amount) + market percentage split + market fixed split)
	if initialBalanceAccount1+int64(float64(itoPercentage)*0.30)+int64(float64(itoFixed)*0.19) != account1.Data.Balance {
		fmt.Println("Account 1 is not ok: expect - current: ", initialBalanceAccount1-+int64(float64(itoPercentage)*0.30)+int64(float64(itoFixed)*0.19), account1.Data.Balance)
	}

	// (initial balance - market percentage + buy amount  + market percentage split + market fixed split)
	if initialBalanceAccount2-2_000_000-buyITOAmount-itoFixed+int64(float64(itoPercentage)*0.32)+int64(float64(itoFixed)*0.17) != account2.Data.Balance {
		fmt.Println("Account 2 is not ok: expect - current: ", initialBalanceAccount2-2_000_000-buyITOAmount-itoFixed+int64(float64(itoPercentage)*0.32)+int64(float64(itoFixed)*0.17), account2.Data.Balance)
	}

	// (initial balance + market percentage split + market fixed split)
	if initialBalanceAccount3+buyITOAmount-itoPercentage+int64(float64(itoPercentage)*0.38)+int64(float64(itoFixed)*0.64) != account3.Data.Balance {
		fmt.Println("Account 3 is not ok: expect - current: ", initialBalanceAccount3+buyITOAmount-itoPercentage+int64(float64(itoPercentage)*0.38)+int64(float64(itoFixed)*0.64), account3.Data.Balance)
	}
}

// CreateKdaRoyalties Should create a KDA with spited royalties, send, sell, buy and validate rewards distribution
func CreateKdaRoyaltiesFungible(args common.TestArgs) {
	fmt.Println("Running KDA Royalties Fungible Test")
	// bootstrap the test accounts
	_, _, addr1, err := utils.LoadKey("wallet-generated-1.pem")
	if err != nil {
		log.Fatalln(err)
	}

	account1, err := account.NewAccount(addr1,
		account.WithSync(), account.WithKeyFile("wallet-generated-1.pem"))
	if err != nil {
		log.Fatalln(err)
	}

	_, _, addr2, err := utils.LoadKey("wallet-generated-2.pem")
	if err != nil {
		log.Fatalln(err)
	}

	account2, err := account.NewAccount(addr2,
		account.WithSync(), account.WithKeyFile("wallet-generated-2.pem"))
	if err != nil {
		log.Fatalln(err)
	}

	_, _, addr3, err := utils.LoadKey("walletKey.pem")
	if err != nil {
		log.Fatalln(err)
	}

	account3, err := account.NewAccount(addr3,
		account.WithSync(), account.WithKeyFile("walletKey.pem"))
	if err != nil {
		log.Fatalln(err)
	}

	// create the royalties NFT
	createHash, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "kda", "create", "0",
		"--name", "TestNFT", "--ticker", "TSTNFT", "--initialSupply", "1000", "--maxSupply", "10000",
		"--canFreeze", "--canPause", "--canMint", "--canBurn", "--canChangeOwner", "--canAddRoles", "--precision", "6",
		"--royaltiesTransferPercentage", "{\"amount\": 100, \"percentage\": 50}",
		"--royaltiesTransferPercentage", "{\"amount\": 500, \"percentage\": 30}",
		"--royaltiesTransferFixed", "10",
		"--royaltiesITOFixed", "12", "--royaltiesITOPercentage", "15",
		"--royaltiesAddress", account3.Address,
		"--splitRoyalties", fmt.Sprintf("{\"address\": \"%s\", \"percentTransferFixed\": 20, \"percentTransferPercentage\": 27, \"percentITOFixed\": 19, \"percentITOPercentage\": 30}", addr1),
		"--splitRoyalties", fmt.Sprintf("{\"address\": \"%s\", \"percentTransferFixed\": 30, \"percentTransferPercentage\": 17, \"percentITOFixed\": 17, \"percentITOPercentage\": 32}", addr2))
	if err != nil {
		log.Fatalln(err)
	}

	status, tx, err := common.CheckTransaction(createHash, 0)
	if err != nil {
		log.Fatalln(err)
	}

	if status != "Ok" {
		log.Fatalln("tx is not success.")
	}

	assetId, err := common.GetAssetId(tx)
	if err != nil {
		log.Fatalln(err)
	}

	// mint NFTS to the accounts
	mintAsset1, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "kda", "trigger", "0", "--kdaID", assetId, "--amount", "2000", "--receiver", account1.Address)
	if err != nil {
		log.Fatalln(err)
	}

	mintAsset2, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "kda", "trigger", "0", "--kdaID", assetId, "--amount", "2000", "--receiver", account2.Address)
	if err != nil {
		log.Fatalln(err)
	}

	statuses, _, err := common.CheckTransactions(0, mintAsset1, mintAsset2)
	if err != nil {
		log.Fatalln(err)
	}

	if statuses[0] != "Ok" || statuses[1] != "Ok" {
		log.Fatalf("mint kda royalties transactions is not ok. Tx1: %s, Tx2: %s\n", statuses[0], statuses[1])
	}

	amount := 10000

	// // Send the created asset to the accounts
	sendAsset1, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "account", "send", account1.Address, fmt.Sprint(amount),
		"--kda", "KLV")
	if err != nil {
		log.Fatalln(err)
	}

	sendAsset2, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "account", "send", account2.Address, fmt.Sprint(amount),
		"--kda", "KLV")
	if err != nil {
		log.Fatalln(err)
	}

	sendStatuses, _, err := common.CheckTransactions(0, sendAsset1, sendAsset2)
	if err != nil {
		log.Fatalln(err)
	}

	if sendStatuses[0] != "Ok" || sendStatuses[1] != "Ok" {
		log.Fatalf("send FPR transactions is not ok. Tx1: %s, Tx2: %s\n", statuses[0], statuses[1])
	}

	syncAccounts(account1, account2, account3)

	initialBalanceAccount1 := account1.Data.Balance
	tokenInitialBalanceAccount1 := account1.Data.Assets[assetId].Balance
	initialBalanceAccount2 := account2.Data.Balance
	tokenInitialBalanceAccount2 := account2.Data.Assets[assetId].Balance
	initialBalanceAccount3 := account3.Data.Balance
	tokenInitialBalanceAccount3 := account3.Data.Assets[assetId].Balance

	sendAssetHash1, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", account1.KeyFile), "account", "send", account2.Address, "50",
		"--kda", assetId)
	if err != nil {
		log.Fatalln(err)
	}

	sendAssetHash2, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", account1.KeyFile), "account", "send", account2.Address, "400",
		"--kda", assetId)
	if err != nil {
		log.Fatalln(err)
	}

	sendAssetHash3, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", account1.KeyFile), "account", "send", account2.Address, "600",
		"--kda", assetId)
	if err != nil {
		log.Fatalln(err)
	}

	tokenSendStatus, txs, err := common.CheckTransactions(0, sendAssetHash1, sendAssetHash2, sendAssetHash3)
	if err != nil {
		log.Fatalln(err)
	}

	if tokenSendStatus[0] != "Ok" || tokenSendStatus[1] != "Ok" || tokenSendStatus[2] != "Ok" {
		log.Fatalf("send FPR transactions is not ok. Tx1: %s, Tx2: %s, Tx3: %s\n", statuses[0], statuses[1], statuses[2])
	}

	syncAccounts(account1, account2, account3)

	// verify kdaRoyalties on transactions
	tx1, ok := txs[0].Contracts[0].Parameter.(map[string]interface{})
	if !ok {
		log.Fatalln("cannot parse transaction contract.")
	}

	paidKdaRoyalts1, ok := tx1["kdaRoyalties"].(float64)
	if !ok {
		log.Fatalln("cannot parse transaction kdaRoyalties.")
	}

	tx2, ok := txs[1].Contracts[0].Parameter.(map[string]interface{})
	if !ok {
		log.Fatalln("cannot parse transaction contract.")
	}

	paidKdaRoyalts2, ok := tx2["kdaRoyalties"].(float64)
	if !ok {
		log.Fatalln("cannot parse transaction kdaRoyalties.")
	}

	tx3, ok := txs[2].Contracts[0].Parameter.(map[string]interface{})
	if !ok {
		log.Fatalln("cannot parse transaction contract.")
	}

	paidKdaRoyalts3, ok := tx3["kdaRoyalties"].(float64)
	if !ok {
		log.Fatalln("cannot parse transaction kdaRoyalties.")
	}

	sendFee := int64(1_500_000) * 3

	fixedAmount := int64(10_000_000) * 3

	transferAmount1 := int64(50_000_000)
	transferAmount2 := int64(400_000_000)
	transferAmount3 := int64(600_000_000)

	expectPercentage1 := int64(float64(transferAmount1) * 0.5)
	expectPercentage2 := int64(float64(100_000_000)*0.5) + int64(float64(transferAmount2-100_000_000)*0.3)
	expectPercentage3 := int64(float64(100_000_000)*0.5) + int64(float64(transferAmount3-100_000_000)*0.3)

	totalPercentage := expectPercentage1 + expectPercentage2 + expectPercentage3
	totalTrasnfer := transferAmount1 + transferAmount2 + transferAmount3

	var difErrs []error
	// verify kdaRoyalties
	if paidKdaRoyalts1 != float64(expectPercentage1) {
		fmt.Println("KdaRoyalties transaction 1, does not match: expect - current:", paidKdaRoyalts1, float64(expectPercentage1))
		difErrs = append(difErrs, errors.New("kdaRoyats transaction 1"))
	}

	if paidKdaRoyalts2 != float64(expectPercentage2) {
		fmt.Println("KdaRoyalties transaction 2, does not match: expect - current:", paidKdaRoyalts1, float64(expectPercentage1))
		difErrs = append(difErrs, errors.New("kdaRoyats transaction 2"))
	}

	if paidKdaRoyalts3 != float64(expectPercentage3) {
		fmt.Println("KdaRoyalties transaction 3, does not match: expect - current:", paidKdaRoyalts1, float64(expectPercentage1))
		difErrs = append(difErrs, errors.New("kdaRoyats transaction 3"))
	}

	//Check Balances after Transfer fixed:
	//(initial balanace - fee - royalties fixed + %20 royalties fixed split)
	if initialBalanceAccount1-sendFee-fixedAmount+int64(float64(fixedAmount)*0.2) != account1.Data.Balance {
		fmt.Println("Account 1 is not ok: expect - current: ", initialBalanceAccount1-sendFee-fixedAmount+int64(float64(fixedAmount)*0.2), account1.Data.Balance)
		difErrs = append(difErrs, errors.New("account diff 1"))
	}

	//(initial balanace - total transferido - total percentaual + 27% royalties percentage split)
	if tokenInitialBalanceAccount1-totalTrasnfer-totalPercentage+int64(float64(totalPercentage)*0.27) != account1.Data.Assets[assetId].Balance {
		fmt.Println("Account 1 is not ok TOKEN: expect - current: ", tokenInitialBalanceAccount1-totalTrasnfer-totalPercentage+int64(float64(totalPercentage)*0.27), account1.Data.Assets[assetId].Balance)
		difErrs = append(difErrs, errors.New("account diff token 1"))
	}

	//(initial balanace + 30% royalties fixed split)
	if initialBalanceAccount2+int64(float64(fixedAmount)*0.3) != account2.Data.Balance {
		fmt.Println("Account 2 is not ok: expect - current: ", initialBalanceAccount2+int64(float64(fixedAmount)*0.3), account2.Data.Balance)
		difErrs = append(difErrs, errors.New("account diff 2"))
	}

	//(initial balanace + total transferido + 17% royalties percentage split)
	if tokenInitialBalanceAccount2+totalTrasnfer+int64(float64(totalPercentage)*0.17) != account2.Data.Assets[assetId].Balance {
		fmt.Println("Account 2 is not ok TOKEN: expect - current: ", tokenInitialBalanceAccount2+totalTrasnfer+int64(float64(totalPercentage)*0.17), account2.Data.Assets[assetId].Balance)
		difErrs = append(difErrs, errors.New("account diff token 2"))
	}

	//(initial balanace + 50% royalties fixed split)
	if initialBalanceAccount3+int64(float64(fixedAmount)*0.5) != account3.Data.Balance {
		fmt.Println("Account 3 is not ok: expect - current: ", initialBalanceAccount3+int64(float64(fixedAmount)*0.5), account3.Data.Balance)
		difErrs = append(difErrs, errors.New("account diff 2"))
	}

	//(initial balanace + 56% royalties percentage split)
	if tokenInitialBalanceAccount3+int64(float64(totalPercentage)*0.56) != account3.Data.Assets[assetId].Balance {
		fmt.Println("Account 3 is not ok TOKEN: expect - current: ", tokenInitialBalanceAccount3+int64(float64(totalPercentage)*0.56), account3.Data.Assets[assetId].Balance)
		difErrs = append(difErrs, errors.New("account diff token 3"))
	}

	if len(difErrs) > 0 {
		log.Fatalln("some errors occurred", difErrs)
	}

	// start ito tests
	configITO, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "kapps", "config-ito", "--kda", assetId,
		"--maxAmount", "100", "--status", "1", "--packs", "{\"kda\": \"KLV\", \"packs\": [{\"amount\": 1, \"price\": 100}, {\"amount\": 10, \"price\": 500}]}", "--receiver", account3.Address)
	if err != nil {
		log.Fatalln(err)
	}

	status, _, err = common.CheckTransaction(configITO, 0)
	if err != nil {
		log.Fatalln(err)
	}

	if status != "Ok" {
		log.Fatalln("tx is not success.")
	}

	syncAccounts(account1, account2, account3)
	initialBalanceAccount1 = account1.Data.Balance
	initialBalanceAccount2 = account2.Data.Balance
	initialBalanceAccount3 = account3.Data.Balance

	buyITOHash, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", account2.KeyFile), "kapps", "buy", "--id", assetId,
		"--kda", "KLV", "--amount", "1")
	if err != nil {
		log.Fatalln(err)
	}

	status, _, err = common.CheckTransaction(buyITOHash, 0)
	if err != nil {
		log.Fatalln(err)
	}

	if status != "Ok" {
		log.Fatalln("tx is not success.")
	}

	syncAccounts(account1, account2, account3)

	buyITOAmount := int64(100_000_000)
	itoPercentage := int64(float64(buyITOAmount) * 0.15)
	itoFixed := int64(float64(buyITOAmount) * 0.12)

	// (initial balance - buy fee - buy amount) + market percentage split + market fixed split)
	if initialBalanceAccount1+int64(float64(itoPercentage)*0.30)+int64(float64(itoFixed)*0.19) != account1.Data.Balance {
		fmt.Println("Account 1 is not ok: expect - current: ", initialBalanceAccount1-+int64(float64(itoPercentage)*0.30)+int64(float64(itoFixed)*0.19), account1.Data.Balance)
		difErrs = append(difErrs, errors.New("account diff on ITO 1"))
	}

	// (initial balance - market percentage + buy amount  + market percentage split + market fixed split)
	if initialBalanceAccount2-2_000_000-buyITOAmount-itoFixed+int64(float64(itoPercentage)*0.32)+int64(float64(itoFixed)*0.17) != account2.Data.Balance {
		fmt.Println("Account 2 is not ok: expect - current: ", initialBalanceAccount2-2_000_000-buyITOAmount-itoFixed+int64(float64(itoPercentage)*0.32)+int64(float64(itoFixed)*0.17), account2.Data.Balance)
		difErrs = append(difErrs, errors.New("account diff on ITO 2"))
	}

	// (initial balance + market percentage split + market fixed split)
	if initialBalanceAccount3+buyITOAmount-itoPercentage+int64(float64(itoPercentage)*0.38)+int64(float64(itoFixed)*0.64) != account3.Data.Balance {
		fmt.Println("Account 3 is not ok: expect - current: ", initialBalanceAccount3+buyITOAmount-itoPercentage+int64(float64(itoPercentage)*0.38)+int64(float64(itoFixed)*0.64), account3.Data.Balance)
		difErrs = append(difErrs, errors.New("account diff on ITO 3"))
	}

	if len(difErrs) > 0 {
		log.Fatalln("some errors occurred", difErrs)
	}
}

func RunTests(args common.TestArgs) {
	CreateKdaRoyaltiesFungible(args)
	CreateKdaRoyalties(args)
}
