package kdaFPR

import (
	"fmt"
	"log"
	"time"

	"github.com/klever-io/klever-go/cmd/tests/utils"

	"github.com/klever-io/klever-go/cmd/tests/common"
	"github.com/klever-io/klever-go/cmd/tests/common/account"
	"github.com/klever-io/klever-go/indexer/data"
)

// CreateKdaFprs Should create a KDA FPR Token, send it to 2 addresses, freeze the token, deposit an amount and distribute it with the correct percent
func CreateKdaFprs(args common.TestArgs) {
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

	// create the FPR asset
	createHash, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "kda", "create", "0",
		"--name", "Test", "--ticker", "TST", "--precision", "6", "--initialSupply", "20000000",
		"--maxSupply", "120000000", "--canFreeze", "--canPause", "--canMint", "--canBurn",
		"--canChangeOwner", "--canAddRoles", "--interestType", "1")

	amount := 100
	depositAmount := 1000

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

	// Send the created asset to the accounts
	sendAsset1, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "account", "send", account1.Address, fmt.Sprint(amount),
		"--kda", assetId)
	if err != nil {
		log.Fatalln(err)
	}

	sendAsset2, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "account", "send", account2.Address, fmt.Sprint(amount),
		"--kda", assetId)
	if err != nil {
		log.Fatalln(err)
	}

	statuses, _, err := common.CheckTransactions(0, sendAsset1, sendAsset2)
	if err != nil {
		log.Fatalln(err)
	}

	if statuses[0] != "Ok" || statuses[1] != "Ok" {
		log.Fatalf("send FPR transactions is not ok. Tx1: %s, Tx2: %s\n", statuses[0], statuses[1])
	}

	// Freeze Asset Transaction
	freezeAsset1, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", account1.KeyFile), "account", "freeze", fmt.Sprint(amount/2),
		"--kda", assetId)
	if err != nil {
		log.Fatalln(err)
	}

	freezeAsset2, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", account2.KeyFile), "account", "freeze", fmt.Sprint(amount),
		"--kda", assetId)
	if err != nil {
		log.Fatalln(err)
	}

	statusesFreeze, _, err := common.CheckTransactions(0, freezeAsset1, freezeAsset2)
	if err != nil {
		log.Fatalln(err)
	}

	if statusesFreeze[0] != "Ok" || statusesFreeze[1] != "Ok" {
		log.Fatalf("Freeze FPR transactions is not ok. Tx1: %s, Tx2: %s\n", statusesFreeze[0], statusesFreeze[1])
	}

	if err := account3.Sync(); err != nil {
		log.Fatalln(err)
	}

	initialBalanceAccount3 := account3.Data.Balance

	// Deposit to the holders
	depositHash, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "kda", "deposit", fmt.Sprint(depositAmount),
		"--kdaID", assetId, "--currencyID", "KLV", "--depositType", "0")

	if err != nil {
		log.Fatalln(err)
	}

	depositStatus, _, err := common.CheckTransaction(depositHash, 0)
	if err != nil {
		log.Fatalln(err)
	}

	if depositStatus != "Ok" {
		log.Fatalln("tx is not success.")
	}

	// Sync the test accounts balances
	if err := account1.Sync(); err != nil {
		log.Fatalln(err)
	}

	if err := account2.Sync(); err != nil {
		log.Fatalln(err)
	}

	// Set the initial balance for comparison
	initialBalanceAccount1 := account1.Data.Balance
	initialBalanceAccount2 := account2.Data.Balance

	// wait til the next epoch to claim
	duration, err := common.GetRemainingTimeToEpoch(1)
	if err != nil {
		log.Fatalln(err)
	}

	fmt.Println("---------------------------")
	fmt.Println("Waiting for the next epoch.")
	fmt.Printf("---------------------------\n\n")

	time.Sleep(duration)

	// verify user allowance
	allowanceUser1, err := account1.GetAllowance(assetId)
	if err != nil {
		log.Fatalln(err)
	}

	allowanceUser2, err := account2.GetAllowance(assetId)
	if err != nil {
		log.Fatalln(err)
	}

	fmt.Println("Allowance Before Claim ACC1: ", allowanceUser1)
	fmt.Println("Allowance Before Claim ACC2: ", allowanceUser2)
	fmt.Printf("CreateKdaFprs\n\n")

	if klvAllowance := allowanceUser1["KLV"]; klvAllowance != 333333333 {
		log.Fatalln("Allowance account1 is wrong.")
	}

	if klvAllowance := allowanceUser2["KLV"]; klvAllowance != 666666666 {
		log.Fatalln("Allowance account2 is wrong.")
	}

	// Claim allowance
	claimAsset1, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", account1.KeyFile), "account", "claim", "0",
		"--id", assetId)
	if err != nil {
		log.Fatalln(err)
	}

	claimAsset2, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", account2.KeyFile), "account", "claim", "0",
		"--id", assetId)
	if err != nil {
		log.Fatalln(err)
	}

	statusesClaim, _, err := common.CheckTransactions(0, claimAsset1, claimAsset2)
	if err != nil {
		log.Fatalln(err)
	}

	if statusesClaim[0] != "Ok" || statusesClaim[1] != "Ok" {
		log.Fatalf("Claim transactions is not ok. Tx1: %s, Tx2: %s\n", statusesClaim[0], statusesClaim[1])
	}

	// Sync the test accounts balances
	if err := account1.Sync(); err != nil {
		log.Fatalln(err)
	}

	if err := account2.Sync(); err != nil {
		log.Fatalln(err)
	}

	feeClaim := int64(2000000)
	feeDeposit := int64(10000000 + 1000000)

	// verify user allowance after claim
	allowanceUser1, err = account1.GetAllowance(assetId)
	if err != nil {
		log.Fatalln(err)
	}

	allowanceUser2, err = account2.GetAllowance(assetId)
	if err != nil {
		log.Fatalln(err)
	}

	if klvAllowance := allowanceUser1["KLV"]; klvAllowance != 0 {
		log.Fatalln("Allowance account1 should be 0.")
	}

	if klvAllowance := allowanceUser2["KLV"]; klvAllowance != 0 {
		log.Fatalln("Allowance account2 should be 0.")
	}

	// sync sender
	if err := account3.Sync(); err != nil {
		log.Fatalln(err)
	}

	// verify balances
	var diffErr []error

	if account1.Data.Balance != initialBalanceAccount1+333333333-feeClaim {
		diffErr = append(diffErr, fmt.Errorf("account 1 balance diff: initial: %v - expect: %v - actual: %v", initialBalanceAccount2, initialBalanceAccount1+333333333-feeClaim, account1.Data.Balance))
		log.Println(diffErr[len(diffErr)-1])
	}

	if account2.Data.Balance != initialBalanceAccount2+666666666-feeClaim {
		diffErr = append(diffErr, fmt.Errorf("account 2 balance diff: initial: %v - expect: %v - actual: %v", initialBalanceAccount2, initialBalanceAccount2+666666666-feeClaim, account2.Data.Balance))
		log.Println(diffErr[len(diffErr)-1])
	}

	if account3.Data.Balance != initialBalanceAccount3-1000_000_000-feeDeposit {
		diffErr = append(diffErr, fmt.Errorf("account 3 balance diff: initial: %v - expect: %v - actual: %v", initialBalanceAccount3, initialBalanceAccount3-1000_000_000-(feeDeposit), account3.Data.Balance))
		log.Println(diffErr[len(diffErr)-1])
	}

	if len(diffErr) == 0 {
		fmt.Println("----- All tests passed! ----")

		fmt.Printf("Account 1: initial: %v - expect: %v - actual: %v\n", initialBalanceAccount1, initialBalanceAccount1+333333333-feeClaim, account1.Data.Balance)
		fmt.Println("Account 1: Allowance: ", allowanceUser1["KLV"])

		fmt.Printf("Account 2: initial: %v - expect: %v - actual: %v\n", initialBalanceAccount2, initialBalanceAccount2+666666666-feeClaim, account2.Data.Balance)
		fmt.Println("Account 2: Allowance: ", allowanceUser2["KLV"])

		fmt.Printf("Account 3 - Owner: initial: %v - expect: %v - actual: %v\n", initialBalanceAccount3, initialBalanceAccount3-1000_000_000-(feeDeposit), account3.Data.Balance)

		fmt.Println("----------------------------")
	} else {
		log.Fatalln("CreateKdaFprs account diffs")
	}
}

func CreateKdaFprsWithoutFreeze(args common.TestArgs) {
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

	// create the FPR asset
	createHash, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "kda", "create", "0",
		"--name", "Test", "--ticker", "TST", "--precision", "6", "--initialSupply", "20000000",
		"--maxSupply", "120000000", "--canFreeze", "--canPause", "--canMint", "--canBurn",
		"--canChangeOwner", "--canAddRoles", "--interestType", "1")

	amount := 100
	depositAmount := 1000

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

	// Send the created asset to the accounts
	sendAsset1, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "account", "send", account1.Address, fmt.Sprint(amount),
		"--kda", assetId)
	if err != nil {
		log.Fatalln(err)
	}

	sendAsset2, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "account", "send", account2.Address, fmt.Sprint(amount),
		"--kda", assetId)
	if err != nil {
		log.Fatalln(err)
	}

	statuses, _, err := common.CheckTransactions(0, sendAsset1, sendAsset2)
	if err != nil {
		log.Fatalln(err)
	}

	if statuses[0] != "Ok" || statuses[1] != "Ok" {
		log.Fatalf("send FPR transactions is not ok. Tx1: %s, Tx2: %s\n", statuses[0], statuses[1])
	}

	// deposit without any freeze
	beforeFreezeDepositHash, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "kda", "deposit", fmt.Sprint(depositAmount),
		"--kdaID", assetId, "--currencyID", "KLV", "--depositType", "0")

	if err != nil {
		log.Fatalln(err)
	}

	beforeFreezeDepositStatus, _, err := common.CheckTransaction(beforeFreezeDepositHash, 0)
	if err != nil {
		log.Fatalln(err)
	}

	if beforeFreezeDepositStatus != "Ok" {
		log.Fatalln("tx is not success.")
	}

	// wait til the next epoch to claim
	duration, err := common.GetRemainingTimeToEpoch(1)
	if err != nil {
		log.Fatalln(err)
	}

	fmt.Println("---------------------------")
	fmt.Println("Waiting for the next epoch.")
	fmt.Printf("---------------------------\n\n")

	time.Sleep(duration)

	// Freeze Asset Transaction
	freezeAsset1, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", account1.KeyFile), "account", "freeze", fmt.Sprint(amount/2),
		"--kda", assetId)
	if err != nil {
		log.Fatalln(err)
	}

	freezeAsset2, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", account2.KeyFile), "account", "freeze", fmt.Sprint(amount),
		"--kda", assetId)
	if err != nil {
		log.Fatalln(err)
	}

	statusesFreeze, _, err := common.CheckTransactions(0, freezeAsset1, freezeAsset2)
	if err != nil {
		log.Fatalln(err)
	}

	if statusesFreeze[0] != "Ok" || statusesFreeze[1] != "Ok" {
		log.Fatalf("Freeze FPR transactions is not ok. Tx1: %s, Tx2: %s\n", statusesFreeze[0], statusesFreeze[1])
	}

	if err := account3.Sync(); err != nil {
		log.Fatalln(err)
	}

	initialBalanceAccount3 := account3.Data.Balance

	// Deposit to the holders
	depositHash, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "kda", "deposit", fmt.Sprint(depositAmount),
		"--kdaID", assetId, "--currencyID", "KLV", "--depositType", "0")

	if err != nil {
		log.Fatalln(err)
	}

	depositStatus, _, err := common.CheckTransaction(depositHash, 0)
	if err != nil {
		log.Fatalln(err)
	}

	if depositStatus != "Ok" {
		log.Fatalln("tx is not success.")
	}

	// Sync the test accounts balances
	if err := account1.Sync(); err != nil {
		log.Fatalln(err)
	}

	if err := account2.Sync(); err != nil {
		log.Fatalln(err)
	}

	// Set the initial balance for comparison
	initialBalanceAccount1 := account1.Data.Balance
	initialBalanceAccount2 := account2.Data.Balance

	// wait til the next epoch to claim
	nextDuration, err := common.GetRemainingTimeToEpoch(1)
	if err != nil {
		log.Fatalln(err)
	}

	fmt.Println("---------------------------")
	fmt.Println("Waiting for the next epoch.")
	fmt.Printf("---------------------------\n\n")

	time.Sleep(nextDuration)

	// verify user allowance
	allowanceUser1, err := account1.GetAllowance(assetId)
	if err != nil {
		log.Fatalln(err)
	}

	allowanceUser2, err := account2.GetAllowance(assetId)
	if err != nil {
		log.Fatalln(err)
	}

	fmt.Println("Allowance Before Claim ACC1: ", allowanceUser1)
	fmt.Println("Allowance Before Claim ACC2: ", allowanceUser2)
	fmt.Printf("CreateKdaFprsWithoutFreeze\n\n")

	if klvAllowance := allowanceUser1["KLV"]; klvAllowance != 333333333 {
		log.Fatalln("Allowance account1 is wrong.")
	}

	if klvAllowance := allowanceUser2["KLV"]; klvAllowance != 666666666 {
		log.Fatalln("Allowance account2 is wrong.")
	}

	// Claim allowance
	claimAsset1, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", account1.KeyFile), "account", "claim", "0",
		"--id", assetId)
	if err != nil {
		log.Fatalln(err)
	}

	claimAsset2, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", account2.KeyFile), "account", "claim", "0",
		"--id", assetId)
	if err != nil {
		log.Fatalln(err)
	}

	statusesClaim, _, err := common.CheckTransactions(0, claimAsset1, claimAsset2)
	if err != nil {
		log.Fatalln(err)
	}

	if statusesClaim[0] != "Ok" || statusesClaim[1] != "Ok" {
		log.Fatalf("Claim transactions is not ok. Tx1: %s, Tx2: %s\n", statusesClaim[0], statusesClaim[1])
	}

	// Sync the test accounts balances
	if err := account1.Sync(); err != nil {
		log.Fatalln(err)
	}

	if err := account2.Sync(); err != nil {
		log.Fatalln(err)
	}

	feeClaim := int64(2000000)
	feeDeposit := int64(10000000 + 1000000)

	// verify user allowance after claim
	allowanceUser1, err = account1.GetAllowance(assetId)
	if err != nil {
		log.Fatalln(err)
	}

	allowanceUser2, err = account2.GetAllowance(assetId)
	if err != nil {
		log.Fatalln(err)
	}

	if klvAllowance := allowanceUser1["KLV"]; klvAllowance != 0 {
		log.Fatalln("Allowance account1 should be 0.")
	}

	if klvAllowance := allowanceUser2["KLV"]; klvAllowance != 0 {
		log.Fatalln("Allowance account2 should be 0.")
	}

	// sync sender
	if err := account3.Sync(); err != nil {
		log.Fatalln(err)
	}

	// verify balances
	var diffErr []error

	if account1.Data.Balance != initialBalanceAccount1+333333333-feeClaim {
		diffErr = append(diffErr, fmt.Errorf("account 1 balance diff: initial: %v - expect: %v - actual: %v", initialBalanceAccount2, initialBalanceAccount1+333333333-feeClaim, account1.Data.Balance))
		log.Println(diffErr[len(diffErr)-1])
	}

	if account2.Data.Balance != initialBalanceAccount2+666666666-feeClaim {
		diffErr = append(diffErr, fmt.Errorf("account 2 balance diff: initial: %v - expect: %v - actual: %v", initialBalanceAccount2, initialBalanceAccount2+666666666-feeClaim, account2.Data.Balance))
		log.Println(diffErr[len(diffErr)-1])
	}

	if account3.Data.Balance != initialBalanceAccount3-1000_000_000-feeDeposit {
		diffErr = append(diffErr, fmt.Errorf("account 3 balance diff: initial: %v - expect: %v - actual: %v", initialBalanceAccount3, initialBalanceAccount3-1000_000_000-(feeDeposit), account3.Data.Balance))
		log.Println(diffErr[len(diffErr)-1])
	}

	if len(diffErr) == 0 {
		fmt.Println("----- All tests passed! ----")

		fmt.Printf("Account 1: initial: %v - expect: %v - actual: %v\n", initialBalanceAccount1, initialBalanceAccount1+333333333-feeClaim, account1.Data.Balance)
		fmt.Println("Account 1: Allowance: ", allowanceUser1["KLV"])

		fmt.Printf("Account 2: initial: %v - expect: %v - actual: %v\n", initialBalanceAccount2, initialBalanceAccount2+666666666-feeClaim, account2.Data.Balance)
		fmt.Println("Account 2: Allowance: ", allowanceUser2["KLV"])

		fmt.Printf("Account 3 - Owner: initial: %v - expect: %v - actual: %v\n", initialBalanceAccount3, initialBalanceAccount3-1000_000_000-(feeDeposit), account3.Data.Balance)

		fmt.Println("----------------------------")
	} else {
		log.Fatalln("CreateKdaFprsWithoutFreeze account diffs")
	}
}

func CreateKdaFprsBeforeFreeze(args common.TestArgs) {
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

	// create the FPR asset
	createHash, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "kda", "create", "0",
		"--name", "Test", "--ticker", "TST", "--precision", "6", "--initialSupply", "20000000",
		"--maxSupply", "120000000", "--canFreeze", "--canPause", "--canMint", "--canBurn",
		"--canChangeOwner", "--canAddRoles", "--interestType", "1")

	amount := 100
	depositAmount := 1000

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

	// Send the created asset to the accounts
	sendAsset1, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "account", "send", account1.Address, fmt.Sprint(amount),
		"--kda", assetId)
	if err != nil {
		log.Fatalln(err)
	}

	sendAsset2, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "account", "send", account2.Address, fmt.Sprint(amount),
		"--kda", assetId)
	if err != nil {
		log.Fatalln(err)
	}

	statuses, _, err := common.CheckTransactions(0, sendAsset1, sendAsset2)
	if err != nil {
		log.Fatalln(err)
	}

	if statuses[0] != "Ok" || statuses[1] != "Ok" {
		log.Fatalf("send FPR transactions is not ok. Tx1: %s, Tx2: %s\n", statuses[0], statuses[1])
	}

	// Create deposit before any freezing
	beforeFreezeDepositHash, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "kda", "deposit", fmt.Sprint(depositAmount),
		"--kdaID", assetId, "--currencyID", "KLV", "--depositType", "0")

	if err != nil {
		log.Fatalln(err)
	}

	beforeFreezeDepositStatus, _, err := common.CheckTransaction(beforeFreezeDepositHash, 0)
	if err != nil {
		log.Fatalln(err)
	}

	if beforeFreezeDepositStatus != "Ok" {
		log.Fatalln("tx is not success.")
	}

	// Freeze Asset Transaction
	freezeAsset1, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", account1.KeyFile), "account", "freeze", fmt.Sprint(amount/2),
		"--kda", assetId)
	if err != nil {
		log.Fatalln(err)
	}

	freezeAsset2, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", account2.KeyFile), "account", "freeze", fmt.Sprint(amount),
		"--kda", assetId)
	if err != nil {
		log.Fatalln(err)
	}

	statusesFreeze, _, err := common.CheckTransactions(0, freezeAsset1, freezeAsset2)
	if err != nil {
		log.Fatalln(err)
	}

	if statusesFreeze[0] != "Ok" || statusesFreeze[1] != "Ok" {
		log.Fatalf("Freeze FPR transactions is not ok. Tx1: %s, Tx2: %s\n", statusesFreeze[0], statusesFreeze[1])
	}

	if err := account3.Sync(); err != nil {
		log.Fatalln(err)
	}

	initialBalanceAccount3 := account3.Data.Balance

	// Deposit to the holders
	depositHash, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "kda", "deposit", fmt.Sprint(depositAmount),
		"--kdaID", assetId, "--currencyID", "KLV", "--depositType", "0")

	if err != nil {
		log.Fatalln(err)
	}

	depositStatus, _, err := common.CheckTransaction(depositHash, 0)
	if err != nil {
		log.Fatalln(err)
	}

	if depositStatus != "Ok" {
		log.Fatalln("tx is not success.")
	}

	// Sync the test accounts balances
	if err := account1.Sync(); err != nil {
		log.Fatalln(err)
	}

	if err := account2.Sync(); err != nil {
		log.Fatalln(err)
	}

	// Set the initial balance for comparison
	initialBalanceAccount1 := account1.Data.Balance
	initialBalanceAccount2 := account2.Data.Balance

	// wait til the next epoch to claim
	duration, err := common.GetRemainingTimeToEpoch(1)
	if err != nil {
		log.Fatalln(err)
	}

	fmt.Println("---------------------------")
	fmt.Println("Waiting for the next epoch.")
	fmt.Printf("---------------------------\n\n")

	time.Sleep(duration)

	// verify user allowance
	allowanceUser1, err := account1.GetAllowance(assetId)
	if err != nil {
		log.Fatalln(err)
	}

	allowanceUser2, err := account2.GetAllowance(assetId)
	if err != nil {
		log.Fatalln(err)
	}

	fmt.Println("Allowance Before Claim ACC1: ", allowanceUser1)
	fmt.Println("Allowance Before Claim ACC2: ", allowanceUser2)
	fmt.Printf("CreateKdaFprsBeforeFreeze\n\n")

	if klvAllowance := allowanceUser1["KLV"]; klvAllowance != 666666666 {
		log.Fatalln("Allowance account1 is wrong.")
	}

	// 1000 KLV / 150 * 100 + 1000 KLV = 666666666 + 666666666 = 1333333332
	if klvAllowance := allowanceUser2["KLV"]; klvAllowance != 1333333332 {
		log.Fatalln("Allowance account2 is wrong.")
	}

	// Claim allowance
	claimAsset1, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", account1.KeyFile), "account", "claim", "0",
		"--id", assetId)
	if err != nil {
		log.Fatalln(err)
	}

	claimAsset2, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", account2.KeyFile), "account", "claim", "0",
		"--id", assetId)
	if err != nil {
		log.Fatalln(err)
	}

	statusesClaim, _, err := common.CheckTransactions(0, claimAsset1, claimAsset2)
	if err != nil {
		log.Fatalln(err)
	}

	if statusesClaim[0] != "Ok" || statusesClaim[1] != "Ok" {
		log.Fatalf("Claim transactions is not ok. Tx1: %s, Tx2: %s\n", statusesClaim[0], statusesClaim[1])
	}

	// Sync the test accounts balances
	if err := account1.Sync(); err != nil {
		log.Fatalln(err)
	}

	if err := account2.Sync(); err != nil {
		log.Fatalln(err)
	}

	feeClaim := int64(2000000)
	feeDeposit := int64(10000000 + 1000000)

	// verify user allowance after claim
	allowanceUser1, err = account1.GetAllowance(assetId)
	if err != nil {
		log.Fatalln(err)
	}

	allowanceUser2, err = account2.GetAllowance(assetId)
	if err != nil {
		log.Fatalln(err)
	}

	if klvAllowance := allowanceUser1["KLV"]; klvAllowance != 0 {
		log.Fatalln("Allowance account1 should be 0.")
	}

	if klvAllowance := allowanceUser2["KLV"]; klvAllowance != 0 {
		log.Fatalln("Allowance account2 should be 0.")
	}

	// sync sender
	if err := account3.Sync(); err != nil {
		log.Fatalln(err)
	}

	// verify balances
	var diffErr []error

	if account1.Data.Balance != initialBalanceAccount1+666666666-feeClaim {
		diffErr = append(diffErr, fmt.Errorf("account 1 balance diff: initial: %v - expect: %v - actual: %v", initialBalanceAccount2, initialBalanceAccount1+666666666-feeClaim, account1.Data.Balance))
		log.Println(diffErr[len(diffErr)-1])
	}

	if account2.Data.Balance != initialBalanceAccount2+1333333332-feeClaim {
		diffErr = append(diffErr, fmt.Errorf("account 2 balance diff: initial: %v - expect: %v - actual: %v", initialBalanceAccount2, initialBalanceAccount2+1333333333-feeClaim, account2.Data.Balance))
		log.Println(diffErr[len(diffErr)-1])
	}

	if account3.Data.Balance != initialBalanceAccount3-1000_000_000-feeDeposit {
		diffErr = append(diffErr, fmt.Errorf("account 3 balance diff: initial: %v - expect: %v - actual: %v", initialBalanceAccount3, initialBalanceAccount3-1000_000_000-(feeDeposit), account3.Data.Balance))
		log.Println(diffErr[len(diffErr)-1])
	}

	if len(diffErr) == 0 {
		fmt.Println("----- All tests passed! ----")

		fmt.Printf("Account 1: initial: %v - expect: %v - actual: %v\n", initialBalanceAccount1, initialBalanceAccount1+666666666-feeClaim, account1.Data.Balance)
		fmt.Println("Account 1: Allowance: ", allowanceUser1["KLV"])

		fmt.Printf("Account 2: initial: %v - expect: %v - actual: %v\n", initialBalanceAccount2, initialBalanceAccount2+1333333333-feeClaim, account2.Data.Balance)
		fmt.Println("Account 2: Allowance: ", allowanceUser2["KLV"])

		fmt.Printf("Account 3 - Owner: initial: %v - expect: %v - actual: %v\n", initialBalanceAccount3, initialBalanceAccount3-1000_000_000-(feeDeposit), account3.Data.Balance)

		fmt.Println("----------------------------")
	} else {
		log.Fatalln("CreateKdaFprsBeforeFreeze account diffs")
	}
}

// CreateKdaFprsOnlyOneStaked Should create a KDA FPR Token, send it to 2 addresses, freeze the token for only 1 address, deposit an amount and distribute it with the correct percent
func CreateKdaFprsOnlyOneStaked(args common.TestArgs) {
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

	// create the FPR asset
	createHash, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "kda", "create", "0",
		"--name", "Test", "--ticker", "TST", "--precision", "6", "--initialSupply", "20000000",
		"--maxSupply", "120000000", "--canFreeze", "--canPause", "--canMint", "--canBurn",
		"--canChangeOwner", "--canAddRoles", "--interestType", "1")

	amount := 100
	depositAmount := 1000

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

	// Send the created asset to the accounts
	sendAsset1, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "account", "send", account1.Address, fmt.Sprint(amount),
		"--kda", assetId)
	if err != nil {
		log.Fatalln(err)
	}

	sendAsset2, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "account", "send", account2.Address, fmt.Sprint(amount),
		"--kda", assetId)
	if err != nil {
		log.Fatalln(err)
	}

	statuses, _, err := common.CheckTransactions(0, sendAsset1, sendAsset2)
	if err != nil {
		log.Fatalln(err)
	}

	if statuses[0] != "Ok" || statuses[1] != "Ok" {
		log.Fatalf("send FPR transactions is not ok. Tx1: %s, Tx2: %s\n", statuses[0], statuses[1])
	}

	// Freeze Asset Transaction
	freezeAsset1, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", account1.KeyFile), "account", "freeze", fmt.Sprint(amount/2),
		"--kda", assetId)
	if err != nil {
		log.Fatalln(err)
	}

	statusesFreeze, _, err := common.CheckTransaction(freezeAsset1, 0)
	if err != nil {
		log.Fatalln(err)
	}

	if statusesFreeze != "Ok" {
		log.Fatalf("Freeze FPR transactions is not ok. Tx1: %s\n", statusesFreeze)
	}

	if err := account3.Sync(); err != nil {
		log.Fatalln(err)
	}

	initialBalanceAccount3 := account3.Data.Balance

	// Deposit to the holders
	depositHash, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "kda", "deposit", fmt.Sprint(depositAmount),
		"--kdaID", assetId, "--currencyID", "KLV", "--depositType", "0")

	if err != nil {
		log.Fatalln(err)
	}

	depositStatus, _, err := common.CheckTransaction(depositHash, 0)
	if err != nil {
		log.Fatalln(err)
	}

	if depositStatus != "Ok" {
		log.Fatalln("tx is not success.")
	}

	// Sync the test accounts balances
	if err := account1.Sync(); err != nil {
		log.Fatalln(err)
	}

	if err := account2.Sync(); err != nil {
		log.Fatalln(err)
	}

	// Set the initial balance for comparison
	initialBalanceAccount1 := account1.Data.Balance
	initialBalanceAccount2 := account2.Data.Balance

	// wait til the next epoch to claim
	duration, err := common.GetRemainingTimeToEpoch(1)
	if err != nil {
		log.Fatalln(err)
	}

	fmt.Println("---------------------------")
	fmt.Println("Waiting for the next epoch.")
	fmt.Printf("---------------------------\n\n")

	time.Sleep(duration)

	// verify user allowance
	allowanceUser1, err := account1.GetAllowance(assetId)
	if err != nil {
		log.Fatalln(err)
	}

	allowanceUser2, err := account2.GetAllowance(assetId)
	if err != nil {
		log.Fatalln(err)
	}

	fmt.Println("Allowance Before Claim ACC1: ", allowanceUser1)
	fmt.Println("Allowance Before Claim ACC2: ", allowanceUser2)
	fmt.Printf("CreateKdaFprsOnlyOneStaked\n\n")

	if klvAllowance := allowanceUser1["KLV"]; klvAllowance != 1000_000_000 {
		log.Fatalln("Allowance account1 is wrong.")
	}

	if klvAllowance := allowanceUser2["KLV"]; klvAllowance != 0 {
		log.Fatalln("Allowance account2 is wrong.")
	}

	// Claim allowance
	claimAsset1, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", account1.KeyFile), "account", "claim", "0",
		"--id", assetId)
	if err != nil {
		log.Fatalln(err)
	}

	statusesClaim, _, err := common.CheckTransaction(claimAsset1, 0)
	if err != nil {
		log.Fatalln(err)
	}

	if statusesClaim != "Ok" {
		log.Fatalf("Claim transactions is not ok. Tx1: %s\n", statusesClaim)
	}

	// Sync the test accounts balances
	if err := account1.Sync(); err != nil {
		log.Fatalln(err)
	}

	if err := account2.Sync(); err != nil {
		log.Fatalln(err)
	}

	// sync sender
	if err := account3.Sync(); err != nil {
		log.Fatalln(err)
	}

	feeClaim := int64(2000000)
	feeDeposit := int64(10000000 + 1000000)

	// verify user allowance after claim
	allowanceUser1, err = account1.GetAllowance(assetId)
	if err != nil {
		log.Fatalln(err)
	}

	allowanceUser2, err = account2.GetAllowance(assetId)
	if err != nil {
		log.Fatalln(err)
	}

	if klvAllowance := allowanceUser1["KLV"]; klvAllowance != 0 {
		log.Fatalln("Allowance account1 should be 0.")
	}

	if klvAllowance := allowanceUser2["KLV"]; klvAllowance != 0 {
		log.Fatalln("Allowance account2 should be 0.")
	}

	// verify balances
	var diffErr []error

	if account1.Data.Balance != initialBalanceAccount1+1000_000_000-feeClaim {
		diffErr = append(diffErr, fmt.Errorf("account 1 balance diff: initial: %v - expect: %v - actual: %v", initialBalanceAccount2, initialBalanceAccount1+1000_000_000-feeClaim, account1.Data.Balance))
		log.Println(diffErr[len(diffErr)-1])
	}

	if account2.Data.Balance != initialBalanceAccount2 {
		diffErr = append(diffErr, fmt.Errorf("account 2 balance diff: initial: %v - expect: %v - actual: %v", initialBalanceAccount2, initialBalanceAccount2, account2.Data.Balance))
		log.Println(diffErr[len(diffErr)-1])
	}

	if account3.Data.Balance != initialBalanceAccount3-1000_000_000-(feeDeposit) {
		diffErr = append(diffErr, fmt.Errorf("account 3 balance diff: initial: %v - expect: %v - actual: %v", initialBalanceAccount3, initialBalanceAccount3-1000_000_000-(feeDeposit), account3.Data.Balance))
		log.Println(diffErr[len(diffErr)-1])
	}

	if len(diffErr) == 0 {
		fmt.Println("----- All tests passed! ----")

		fmt.Printf("Account 1: initial: %v - expect: %v - actual: %v\n", initialBalanceAccount1, initialBalanceAccount1+1000_000_000-feeClaim, account1.Data.Balance)
		fmt.Println("Account 1: Allowance: ", allowanceUser1["KLV"])

		fmt.Printf("Account 2: initial: %v - expect: %v - actual: %v\n", initialBalanceAccount2, initialBalanceAccount2, account2.Data.Balance)
		fmt.Println("Account 2: Allowance: ", allowanceUser2["KLV"])

		fmt.Printf("Account 3 - Owner: initial: %v - expect: %v - actual: %v\n", initialBalanceAccount3, initialBalanceAccount3-1000_000_000-(feeDeposit), account3.Data.Balance)

		fmt.Println("----------------------------")
	} else {
		log.Fatalln("CreateKdaFprsOnlyOneStaked account diffs")
	}
}

// CreateKdaFprsMultipleTokenDeposits Should create a KDA FPR Token, send it to 2 addresses, freeze the token, deposit an amount for KLV and another for KFI and distribute it with the correct percent
func CreateKdaFprsMultipleTokenDeposits(args common.TestArgs) {
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

	// create the FPR asset
	createHash, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "kda", "create", "0",
		"--name", "Test", "--ticker", "TST", "--precision", "6", "--initialSupply", "20000000",
		"--maxSupply", "120000000", "--canFreeze", "--canPause", "--canMint", "--canBurn",
		"--canChangeOwner", "--canAddRoles", "--interestType", "1")

	amount := 100
	depositAmount := 1000

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

	// Send the created asset to the accounts
	sendAsset1, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "account", "send", account1.Address, fmt.Sprint(amount),
		"--kda", assetId)
	if err != nil {
		log.Fatalln(err)
	}

	sendAsset2, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "account", "send", account2.Address, fmt.Sprint(amount),
		"--kda", assetId)
	if err != nil {
		log.Fatalln(err)
	}

	statuses, _, err := common.CheckTransactions(0, sendAsset1, sendAsset2)
	if err != nil {
		log.Fatalln(err)
	}

	if statuses[0] != "Ok" || statuses[1] != "Ok" {
		log.Fatalf("send FPR transactions is not ok. Tx1: %s, Tx2: %s\n", statuses[0], statuses[1])
	}

	// Freeze Asset Transaction
	freezeAsset1, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", account1.KeyFile), "account", "freeze", fmt.Sprint(amount/2),
		"--kda", assetId)
	if err != nil {
		log.Fatalln(err)
	}

	freezeAsset2, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", account2.KeyFile), "account", "freeze", fmt.Sprint(amount),
		"--kda", assetId)
	if err != nil {
		log.Fatalln(err)
	}

	statusesFreeze, _, err := common.CheckTransactions(0, freezeAsset1, freezeAsset2)
	if err != nil {
		log.Fatalln(err)
	}

	if statusesFreeze[0] != "Ok" || statusesFreeze[1] != "Ok" {
		log.Fatalf("Freeze FPR transactions is not ok. Tx1: %s, Tx2: %s\n", statusesFreeze[0], statusesFreeze[1])
	}

	if err := account3.Sync(); err != nil {
		log.Fatalln(err)
	}

	initialBalanceAccount3 := account3.Data.Balance
	initialKFIBalanceAccount3 := int64(0)
	kfiData, ok := account3.Data.Assets["KFI"]
	if ok {
		initialKFIBalanceAccount3 = kfiData.Balance
	}

	// Deposit to the holders
	depositHash1, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "kda", "deposit", fmt.Sprint(depositAmount),
		"--kdaID", assetId, "--currencyID", "KLV", "--depositType", "0")

	if err != nil {
		log.Fatalln(err)
	}

	depositHash2, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "kda", "deposit", fmt.Sprint(depositAmount),
		"--kdaID", assetId, "--currencyID", "KFI", "--depositType", "0")

	if err != nil {
		log.Fatalln(err)
	}

	depositStatus, _, err := common.CheckTransactions(0, depositHash1, depositHash2)
	if err != nil {
		log.Fatalln(err)
	}

	if depositStatus[0] != "Ok" || depositStatus[1] != "Ok" {
		log.Fatalf("send FPR transactions is not ok. Tx1: %s, Tx2: %s\n", depositStatus[0], depositStatus[1])
	}

	// Sync the test accounts balances
	if err := account1.Sync(); err != nil {
		log.Fatalln(err)
	}

	if err := account2.Sync(); err != nil {
		log.Fatalln(err)
	}

	// Set the initial balance for comparison
	initialBalanceAccount1 := account1.Data.Balance
	initialBalanceAccount2 := account2.Data.Balance

	kfi1, ok := account1.Data.Assets["KFI"]
	if !ok {
		kfi1 = &data.AccountKDA{}
	}

	kfi2, ok := account2.Data.Assets["KFI"]
	if !ok {
		kfi2 = &data.AccountKDA{}
	}
	initialKFIBalanceAccount1 := kfi1.Balance
	initialKFIBalanceAccount2 := kfi2.Balance

	// wait til the next epoch to claim
	duration, err := common.GetRemainingTimeToEpoch(1)
	if err != nil {
		log.Fatalln(err)
	}

	fmt.Println("---------------------------")
	fmt.Println("Waiting for the next epoch.")
	fmt.Printf("---------------------------\n\n")

	time.Sleep(duration)

	// verify user allowance
	allowanceUser1, err := account1.GetAllowance(assetId)
	if err != nil {
		log.Fatalln(err)
	}

	allowanceUser2, err := account2.GetAllowance(assetId)
	if err != nil {
		log.Fatalln(err)
	}

	fmt.Println("Allowance Before Claim ACC1: ", allowanceUser1)
	fmt.Println("Allowance Before Claim ACC2: ", allowanceUser2)
	fmt.Printf("CreateKdaFprsMultipleTokenDeposits\n\n")

	if klvAllowance := allowanceUser1["KLV"]; klvAllowance != 333333333 {
		log.Fatalln("Allowance account1 is wrong.")
	}

	if klvAllowance := allowanceUser2["KLV"]; klvAllowance != 666666666 {
		log.Fatalln("Allowance account2 is wrong.")
	}

	if klvAllowance := allowanceUser1["KFI"]; klvAllowance != 333333333 {
		log.Fatalln("KFI Allowance account1 is wrong.")
	}

	if klvAllowance := allowanceUser2["KFI"]; klvAllowance != 666666666 {
		log.Fatalln("KFI Allowance account2 is wrong.")
	}

	// Claim allowance
	claimAsset1, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", account1.KeyFile), "account", "claim", "0",
		"--id", assetId)
	if err != nil {
		log.Fatalln(err)
	}

	claimAsset2, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", account2.KeyFile), "account", "claim", "0",
		"--id", assetId)
	if err != nil {
		log.Fatalln(err)
	}

	statusesClaim, _, err := common.CheckTransactions(0, claimAsset1, claimAsset2)
	if err != nil {
		log.Fatalln(err)
	}

	if statusesClaim[0] != "Ok" || statusesClaim[1] != "Ok" {
		log.Fatalf("Claim transactions is not ok. Tx1: %s, Tx2: %s\n", statusesClaim[0], statusesClaim[1])
	}

	// Sync the test accounts balances
	if err := account1.Sync(); err != nil {
		log.Fatalln(err)
	}

	if err := account2.Sync(); err != nil {
		log.Fatalln(err)
	}

	feeClaim := int64(2000000)
	feeDeposit := int64(10000000 + 1000000)

	// verify user allowance after claim
	allowanceUser1, err = account1.GetAllowance(assetId)
	if err != nil {
		log.Fatalln(err)
	}

	allowanceUser2, err = account2.GetAllowance(assetId)
	if err != nil {
		log.Fatalln(err)
	}

	if klvAllowance := allowanceUser1["KLV"]; klvAllowance != 0 {
		log.Fatalln("Allowance account1 should be 0.")
	}

	if klvAllowance := allowanceUser2["KLV"]; klvAllowance != 0 {
		log.Fatalln("Allowance account2 should be 0.")
	}

	if klvAllowance := allowanceUser1["KFI"]; klvAllowance != 0 {
		log.Fatalln("Allowance account1 should be 0.")
	}

	if klvAllowance := allowanceUser2["KFI"]; klvAllowance != 0 {
		log.Fatalln("Allowance account2 should be 0.")
	}

	// sync sender
	if err := account3.Sync(); err != nil {
		log.Fatalln(err)
	}

	// verify balances
	var diffErr []error

	if account1.Data.Balance != initialBalanceAccount1+333333333-feeClaim {
		diffErr = append(diffErr, fmt.Errorf("account 1 balance diff: initial: %v - expect: %v - actual: %v", initialBalanceAccount2, initialBalanceAccount1+333333333-feeClaim, account1.Data.Balance))
		log.Println(diffErr[len(diffErr)-1])
	}

	if account2.Data.Balance != initialBalanceAccount2+666666666-feeClaim {
		diffErr = append(diffErr, fmt.Errorf("account 2 balance diff: initial: %v - expect: %v - actual: %v", initialBalanceAccount2, initialBalanceAccount2+666666666-feeClaim, account2.Data.Balance))
		log.Println(diffErr[len(diffErr)-1])
	}

	if account3.Data.Balance != initialBalanceAccount3-1000_000_000-(feeDeposit*2) {
		diffErr = append(diffErr, fmt.Errorf("account 3 balance diff: initial: %v - expect: %v - actual: %v", initialBalanceAccount3, initialBalanceAccount3-1000_000_000-(feeDeposit*2), account3.Data.Balance))
		log.Println(diffErr[len(diffErr)-1])
	}

	if account1.Data.Assets["KFI"].Balance != initialKFIBalanceAccount1+333333333 {
		diffErr = append(diffErr, fmt.Errorf("account 1 balance KFI diff: initial: %v - expect: %v - actual: %v", initialKFIBalanceAccount1, initialKFIBalanceAccount1+333333333, account1.Data.Assets["KFI"].Balance))
		log.Println(diffErr[len(diffErr)-1])
	}

	if account2.Data.Assets["KFI"].Balance != initialKFIBalanceAccount2+666666666 {
		diffErr = append(diffErr, fmt.Errorf("account 2 balance KFI diff: initial: %v - expect: %v - actual: %v", initialKFIBalanceAccount1, initialKFIBalanceAccount1+666666666, account2.Data.Assets["KFI"].Balance))
		log.Println(diffErr[len(diffErr)-1])
	}

	if account3.Data.Assets["KFI"].Balance != initialKFIBalanceAccount3-1000_000_000 {
		diffErr = append(diffErr, fmt.Errorf("account 3 balance KFI diff: initial: %v - expect: %v - actual: %v", initialKFIBalanceAccount3, initialKFIBalanceAccount3-1000_000_000, account3.Data.Assets["KFI"].Balance))
		log.Println(diffErr[len(diffErr)-1])
	}

	if len(diffErr) == 0 {
		fmt.Println("----- All tests passed! ----")

		fmt.Printf("Account 1: initial: %v - expect: %v - actual: %v\n", initialBalanceAccount1, initialBalanceAccount1+333333333-feeClaim, account1.Data.Balance)
		fmt.Printf("Account 1 KFI: initial: %v - expect: %v - actual: %v\n", initialKFIBalanceAccount1, initialKFIBalanceAccount1+333333333, account1.Data.Assets["KFI"].Balance)
		fmt.Println("Account 1: Allowance: ", allowanceUser1)

		fmt.Printf("Account 2: initial: %v - expect: %v - actual: %v\n", initialBalanceAccount2, initialBalanceAccount2+666666666-feeClaim, account2.Data.Balance)
		fmt.Printf("Account 2 KFI: initial: %v - expect: %v - actual: %v\n", initialKFIBalanceAccount2, initialKFIBalanceAccount2+666666666, account2.Data.Assets["KFI"].Balance)
		fmt.Println("Account 2: Allowance: ", allowanceUser2)

		fmt.Printf("Account 3 - Owner: initial: %v - expect: %v - actual: %v\n", initialBalanceAccount3, initialBalanceAccount3-1000_000_000-(feeDeposit*2), account3.Data.Balance)
		fmt.Printf("Account 3 KFI - Owner: initial: %v - expect: %v - actual: %v\n", initialKFIBalanceAccount2, initialKFIBalanceAccount2-1000_000_000, account3.Data.Assets["KFI"].Balance)

		fmt.Println("----------------------------")
	} else {
		log.Fatalln("CreateKdaFprsMultipleTokenDeposits account diffs")
	}
}

// CreateKdaFprs Should create a KDA FPR Token, send it to 2 addresses, freeze the for the owner and the others, deposit an amount and distribute it with the correct percent
func CreateKdaFprsSelfFreeze(args common.TestArgs) {
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

	// create the FPR asset
	createHash, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "kda", "create", "0",
		"--name", "Test", "--ticker", "TST", "--precision", "6", "--initialSupply", "20000000",
		"--maxSupply", "120000000", "--canFreeze", "--canPause", "--canMint", "--canBurn",
		"--canChangeOwner", "--canAddRoles", "--interestType", "1")

	amount := 100
	depositAmount := 1000

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

	// Send the created asset to the accounts
	sendAsset1, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "account", "send", account1.Address, fmt.Sprint(amount),
		"--kda", assetId)
	if err != nil {
		log.Fatalln(err)
	}

	sendAsset2, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "account", "send", account2.Address, fmt.Sprint(amount),
		"--kda", assetId)
	if err != nil {
		log.Fatalln(err)
	}

	statuses, _, err := common.CheckTransactions(0, sendAsset1, sendAsset2)
	if err != nil {
		log.Fatalln(err)
	}

	if statuses[0] != "Ok" || statuses[1] != "Ok" {
		log.Fatalf("send FPR transactions is not ok. Tx1: %s, Tx2: %s\n", statuses[0], statuses[1])
	}

	// Freeze Asset Transaction
	freezeAsset1, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", account1.KeyFile), "account", "freeze", fmt.Sprint(amount),
		"--kda", assetId)
	if err != nil {
		log.Fatalln(err)
	}

	freezeAsset2, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", account2.KeyFile), "account", "freeze", fmt.Sprint(amount),
		"--kda", assetId)
	if err != nil {
		log.Fatalln(err)
	}

	freezeAsset3, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "account", "freeze", fmt.Sprint(amount),
		"--kda", assetId)
	if err != nil {
		log.Fatalln(err)
	}

	statusesFreeze, _, err := common.CheckTransactions(0, freezeAsset1, freezeAsset2, freezeAsset3)
	if err != nil {
		log.Fatalln(err)
	}

	if statusesFreeze[0] != "Ok" || statusesFreeze[1] != "Ok" || statusesFreeze[2] != "Ok" {
		log.Fatalf("Freeze FPR transactions is not ok. Tx1: %s, Tx2: %s\n", statusesFreeze[0], statusesFreeze[1])
	}

	// sync account before deposit
	if err := account3.Sync(); err != nil {
		log.Fatalln(err)
	}

	initialBalanceAccount3 := account3.Data.Balance

	// Deposit to the holders
	depositHash, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "kda", "deposit", fmt.Sprint(depositAmount),
		"--kdaID", assetId, "--currencyID", "KLV", "--depositType", "0")

	if err != nil {
		log.Fatalln(err)
	}

	depositStatus, _, err := common.CheckTransaction(depositHash, 0)
	if err != nil {
		log.Fatalln(err)
	}

	if depositStatus != "Ok" {
		log.Fatalln("tx is not success.")
	}

	// Sync the test accounts balances
	if err := account1.Sync(); err != nil {
		log.Fatalln(err)
	}

	if err := account2.Sync(); err != nil {
		log.Fatalln(err)
	}

	// Set the initial balance for comparison
	initialBalanceAccount1 := account1.Data.Balance
	initialBalanceAccount2 := account2.Data.Balance

	// wait til the next epoch to claim
	duration, err := common.GetRemainingTimeToEpoch(1)
	if err != nil {
		log.Fatalln(err)
	}

	fmt.Println("---------------------------")
	fmt.Println("Waiting for the next epoch.")
	fmt.Printf("---------------------------\n\n")

	time.Sleep(duration)

	// verify user allowance
	allowanceUser1, err := account1.GetAllowance(assetId)
	if err != nil {
		log.Fatalln(err)
	}

	allowanceUser2, err := account2.GetAllowance(assetId)
	if err != nil {
		log.Fatalln(err)
	}

	allowanceUser3, err := account2.GetAllowance(assetId)
	if err != nil {
		log.Fatalln(err)
	}

	fmt.Println("Allowance Before Claim ACC1: ", allowanceUser1)
	fmt.Println("Allowance Before Claim ACC2: ", allowanceUser2)
	fmt.Println("Allowance Before Claim ACC3 - Owner: ", allowanceUser3)
	fmt.Printf("CreateKdaFprsSelfFreeze\n\n")

	if klvAllowance := allowanceUser1["KLV"]; klvAllowance != 333333333 {
		log.Fatalln("Allowance account1 is wrong.")
	}

	if klvAllowance := allowanceUser2["KLV"]; klvAllowance != 333333333 {
		log.Fatalln("Allowance account2 is wrong.")
	}

	if klvAllowance := allowanceUser3["KLV"]; klvAllowance != 333333333 {
		log.Fatalln("Allowance account3 is wrong.")
	}

	// Claim allowance
	claimAsset1, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", account1.KeyFile), "account", "claim", "0",
		"--id", assetId)
	if err != nil {
		log.Fatalln(err)
	}

	claimAsset2, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", account2.KeyFile), "account", "claim", "0",
		"--id", assetId)
	if err != nil {
		log.Fatalln(err)
	}

	claimAsset3, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "account", "claim", "0",
		"--id", assetId)
	if err != nil {
		log.Fatalln(err)
	}

	statusesClaim, _, err := common.CheckTransactions(0, claimAsset1, claimAsset2, claimAsset3)
	if err != nil {
		log.Fatalln(err)
	}

	if statusesClaim[0] != "Ok" || statusesClaim[1] != "Ok" || statusesClaim[2] != "Ok" {
		log.Fatalf("Claim transactions is not ok. Tx1: %s, Tx2: %s, Tx3: %s\n", statusesClaim[0], statusesClaim[1], statusesClaim[2])
	}

	// Sync the test accounts balances
	if err := account1.Sync(); err != nil {
		log.Fatalln(err)
	}

	if err := account2.Sync(); err != nil {
		log.Fatalln(err)
	}

	if err := account3.Sync(); err != nil {
		log.Fatalln(err)
	}

	feeClaim := int64(2000000)
	feeDeposit := int64(10000000 + 1000000)

	// verify user allowance after claim
	allowanceUser1, err = account1.GetAllowance(assetId)
	if err != nil {
		log.Fatalln(err)
	}

	allowanceUser2, err = account2.GetAllowance(assetId)
	if err != nil {
		log.Fatalln(err)
	}

	allowanceUser3, err = account3.GetAllowance(assetId)
	if err != nil {
		log.Fatalln(err)
	}

	if klvAllowance := allowanceUser1["KLV"]; klvAllowance != 0 {
		log.Fatalln("Allowance account1 should be 0.")
	}

	if klvAllowance := allowanceUser2["KLV"]; klvAllowance != 0 {
		log.Fatalln("Allowance account2 should be 0.")
	}

	if klvAllowance := allowanceUser3["KLV"]; klvAllowance != 0 {
		log.Fatalln("Allowance account3 should be 0.")
	}

	// verify balances
	var diffErr []error

	if account1.Data.Balance != initialBalanceAccount1+333333333-feeClaim {
		diffErr = append(diffErr, fmt.Errorf("account 1 balance diff: initial: %v - expect: %v - actual: %v", initialBalanceAccount2, initialBalanceAccount1+333333333-feeClaim, account1.Data.Balance))
		log.Println(diffErr[len(diffErr)-1])
	}

	if account2.Data.Balance != initialBalanceAccount2+333333333-feeClaim {
		diffErr = append(diffErr, fmt.Errorf("account 2 balance diff: initial: %v - expect: %v - actual: %v", initialBalanceAccount2, initialBalanceAccount2+333333333-feeClaim, account2.Data.Balance))
		log.Println(diffErr[len(diffErr)-1])
	}

	if account3.Data.Balance != initialBalanceAccount3+333333333-feeClaim-1000_000_000-feeDeposit {
		diffErr = append(diffErr, fmt.Errorf("account 3 balance diff: initial: %v - expect: %v - actual: %v", initialBalanceAccount3, initialBalanceAccount3+333333333-feeClaim-1000_000_000-feeDeposit, account3.Data.Balance))
		log.Println(diffErr[len(diffErr)-1])
	}

	if len(diffErr) == 0 {
		fmt.Println("----- All tests passed! ----")

		fmt.Printf("Account 1: initial: %v - expect: %v - actual: %v\n", initialBalanceAccount1, initialBalanceAccount1+333333333-feeClaim, account1.Data.Balance)
		fmt.Println("Account 1: Allowance: ", allowanceUser1["KLV"])

		fmt.Printf("Account 2: initial: %v - expect: %v - actual: %v\n", initialBalanceAccount2, initialBalanceAccount2+333333333-feeClaim, account2.Data.Balance)
		fmt.Println("Account 2: Allowance: ", allowanceUser2["KLV"])

		fmt.Printf("Account 3 - Owner: initial: %v - expect: %v - actual: %v\n", initialBalanceAccount3, initialBalanceAccount3+333333333-feeClaim-1000_000_000, account3.Data.Balance)
		fmt.Println("Account 3 - Owner: Allowance: ", allowanceUser3["KLV"])

		fmt.Println("----------------------------")
	} else {
		log.Fatalln("CreateKdaFprsSelfFreeze account diffs")
	}
}

func RunTests(args common.TestArgs) {
	CreateKdaFprsSelfFreeze(args)
	CreateKdaFprsMultipleTokenDeposits(args)
	CreateKdaFprsOnlyOneStaked(args)
	CreateKdaFprs(args)
	CreateKdaFprsBeforeFreeze(args)
	CreateKdaFprsWithoutFreeze(args)
}
