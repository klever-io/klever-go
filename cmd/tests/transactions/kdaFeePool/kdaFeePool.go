package kdaFeePool

import (
	"fmt"
	"log"
	"math"
	"time"

	"github.com/klever-io/klever-go/cmd/tests/common"
	"github.com/klever-io/klever-go/cmd/tests/common/account"
	"github.com/klever-io/klever-go/cmd/tests/utils"
	"github.com/klever-io/klever-go/indexer/data"
)

func SendKDAPayFeesWithKDA(args common.TestArgs, keyfile string, to string, amount string, kdaID string) (*data.Transaction, error) {
	sendHash, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", keyfile), "account", "send", to, amount, fmt.Sprintf("--kda=%s", kdaID), fmt.Sprintf("--kdaFee=%s", kdaID))
	if err != nil {
		return nil, err
	}

	status, tx, err := common.CheckTransaction(sendHash, 0)
	if err != nil {
		return nil, err
	}

	if status != "Ok" {
		return nil, fmt.Errorf("tx is not success")
	}

	return tx, nil
}

func SendPayFeesWithKDA(args common.TestArgs, keyfile string, to string, amount string, kdaFee string) (*data.Transaction, error) {
	sendHash, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", keyfile), "account", "send", to, amount, fmt.Sprintf("--kdaFee=%s", kdaFee))
	if err != nil {
		return nil, err
	}

	status, tx, err := common.CheckTransaction(sendHash, 0)
	if err != nil {
		return nil, err
	}

	if status != "Ok" {
		return nil, fmt.Errorf("tx is not success")
	}

	return tx, nil
}

func CreateWallet() (string, string, error) {
	walletName := fmt.Sprintf("auxAddress-%d.pem", time.Now().Unix())
	err := common.Exec("go", "run", "./cmd/operator/", "account", "create", fmt.Sprintf("--key-file=./%s", walletName))
	if err != nil {
		return "", "", err
	}

	time.Sleep(5 * time.Second)

	_, _, addr, err := utils.LoadKey(walletName)
	if err != nil || len(addr) == 0 {
		err = fmt.Errorf("AddrLen: %d, Error: %v", len(addr), err)
		fmt.Printf("error : %s", err)
		return "", "", err
	}

	return walletName, addr, nil

}

func CreateKdaFeePool(args common.TestArgs, klvRatio string, kdaRation string) (string, error) {
	_, _, addr, err := utils.LoadKey(args.KeyFile)
	if err != nil {
		return "", err
	}

	// create the KDA Fee Pool asset
	createHash, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "kda", "create", "0",
		"--name", "Test", "--ticker", "TST", "--precision", "6", "--initialSupply", "20000000",
		"--maxSupply", "120000000", "--canMint", "--royaltiesAddress", addr)

	if err != nil {
		return "", err
	}

	status, tx, err := common.CheckTransaction(createHash, 0)
	if err != nil {
		return "", err
	}

	if status != "Ok" {
		return "", fmt.Errorf("tx is not success")
	}

	assetId, err := common.GetAssetId(tx)
	if err != nil {
		return "", err
	}

	triggerHash, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "kda", "trigger", "15",
		fmt.Sprintf("--kdaID=%s", assetId), "--updateKdaPool", fmt.Sprintf("active=true,adminAddress=%s,fixedRatioKLV=%s,fixedRatioKDA=%s", addr, klvRatio, kdaRation))
	if err != nil {
		return "", err
	}

	status, _, err = common.CheckTransaction(triggerHash, 0)
	if err != nil {
		return "", err
	}

	if status != "Ok" {
		return "", fmt.Errorf("tx is not success")
	}

	return assetId, nil
}

func UpdateKdaPool(args common.TestArgs, keyfile string, kdaID string, adminAddress string, klvRatio string, kdaRatio string) error {
	// Update the KDA Fee Pool asset
	updateHash, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", keyfile), "kda", "trigger", "15",
		fmt.Sprintf("--kdaID=%s", kdaID), "--updateKdaPool", fmt.Sprintf("active=true,adminAddress=%s,fixedRatioKLV=%s,fixedRatioKDA=%s", adminAddress, klvRatio, kdaRatio))

	if err != nil {
		return err
	}

	status, _, err := common.CheckTransaction(updateHash, 0)
	if err != nil {
		return err
	}

	if status != "Ok" {
		return fmt.Errorf("tx is not success")
	}
	return nil
}

func DepositKdaFeePool(args common.TestArgs, assetID string, currencyID string, amount float64) error {
	depositHash, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "kda", "deposit", fmt.Sprintf("%f", amount/math.Pow10(6)), "--kdaID", assetID, "--currencyID", currencyID, "--depositType", "1")
	if err != nil {
		return err
	}

	status, _, err := common.CheckTransaction(depositHash, 0)
	if err != nil {
		return err
	}

	if status != "Ok" {
		return fmt.Errorf("tx is not success")
	}

	return nil
}

func WithdrawKdaFeePool(args common.TestArgs, assetID string, currencyID string, amount float64) error {
	withdrawHash, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "kda", "withdraw", fmt.Sprintf("%f", amount/math.Pow10(6)), "--kdaID", assetID, "--currencyID", currencyID)
	if err != nil {
		return err
	}

	status, _, err := common.CheckTransaction(withdrawHash, 0)
	if err != nil {
		return err
	}

	if status != "Ok" {
		return fmt.Errorf("tx is not success")
	}

	return nil
}

func configITOButPayedWithKDA(args common.TestArgs, kdaID string) error {
	_, _, testWalletAddress, err := utils.LoadKey(args.KeyFile)
	if err != nil {
		return err
	}
	// create asset
	createAssetHash, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "kda", "create", "1",
		"--name", "Test", "--ticker", "TST", "--maxSupply", "10000", "--canFreeze", "--canPause", "--canMint", "--canBurn",
		"--canChangeOwner", "--canAddRoles", fmt.Sprintf("--kdaFee=%s", kdaID))

	if err != nil {
		return err
	}

	status, createAssetTx, err := common.CheckTransaction(createAssetHash, 0)
	if err != nil {
		return err
	}

	if status != "Ok" {
		return fmt.Errorf("tx is not success")
	}

	assetId, err := common.GetAssetId(createAssetTx)
	if err != nil {
		return err
	}

	// config ito
	startTime := time.Now().Add(time.Minute).Unix()
	endTime := time.Now().AddDate(0, 0, 3).Unix()

	whitelistStartTime := time.Now().Add(time.Minute).Unix()
	whitelistEndTime := time.Now().AddDate(0, 0, 3).Unix()

	configITOHash, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "kapps", "config-ito", fmt.Sprintf("--kda=%s", assetId), "--maxAmount=1000",
		fmt.Sprintf("--receiver=%s", testWalletAddress), "--status=1", `--packs={"kda": "KLV", "packs": [{"amount":1, "price":20},{"amount":10, "price":150}]}`, fmt.Sprintf("--startTime=%d", startTime),
		fmt.Sprintf("--endTime=%d", endTime), "--whitelist=klv1rsm4vx8pdg2cy74zz9gwff0avl7vwunzy9tv7utn8eey0chtdaasectfw4=10,klv1fpwjz234gy8aaae3gx0e8q9f52vymzzn3z5q0s5h60pvktzx0n0qwvtux5=20",
		"--whitelistStatus=1", fmt.Sprintf("--whitelistStartTime=%d", whitelistStartTime), fmt.Sprintf("--whitelistEndTime=%d", whitelistEndTime), fmt.Sprintf("--kdaFee=%s", kdaID))
	if err != nil {
		return err
	}

	status, _, err = common.CheckTransaction(configITOHash, 0)
	if err != nil {
		return err
	}

	if status != "Ok" {
		return fmt.Errorf("tx is not success")
	}

	return nil
}

func kdaFeePool_KLV_DepositAndWithdraw(args common.TestArgs) error {
	fmt.Println("KDA Fee Pool KLV Deposit and Withdraw")
	// Create a asset and activate the feepool of this asset
	assetId, err := CreateKdaFeePool(args, "1000", "100000")
	if err != nil {
		return err
	}

	// Check the balance of the activate fee pool
	// probably return  0 because we don't deposit any asset yet
	pool1, _ := common.GetPool(assetId)

	amountToDeposit := float64(10_000_000)

	// Deposit 10 KLV
	err = DepositKdaFeePool(args, assetId, "KLV", amountToDeposit)
	if err != nil {
		return err
	}

	time.Sleep(4 * time.Second) // wait for the next slot

	pool2, _ := common.GetPool(assetId)
	if err != nil {
		return err
	}

	if pool2.Data.Pool.KLVBalance-pool1.Data.Pool.KLVBalance != amountToDeposit {
		return fmt.Errorf("the deposit amount is different")
	}

	amountToWithdraw := float64(5_000_000)

	err = WithdrawKdaFeePool(args, assetId, "KLV", amountToWithdraw)
	if err != nil {
		return err
	}

	time.Sleep(4 * time.Second) // wait for the next slot

	pool3, _ := common.GetPool(assetId)
	if err != nil {
		return err
	}

	if pool2.Data.Pool.KLVBalance-pool3.Data.Pool.KLVBalance != amountToWithdraw {
		return fmt.Errorf("the withdraw amount is different")
	}

	return nil
}

func createKdaFeePool_KLV_Deposit(args common.TestArgs) (string, error) {
	fmt.Println("KDA Fee Pool KLV Deposit")
	// Create a asset and activate the feepool of this asset
	assetId, err := CreateKdaFeePool(args, "1", "1")
	if err != nil {
		return "", err
	}

	// Check the balance of the activate fee pool
	// probably return  0 because we don't deposit any asset yet
	pool1, _ := common.GetPool(assetId)

	amountToDeposit := float64(100_000_000_000)

	// Deposit 10 KLV
	err = DepositKdaFeePool(args, assetId, "KLV", amountToDeposit)
	if err != nil {
		return "", err
	}

	time.Sleep(4 * time.Second) // wait for the next slot

	pool2, err := common.GetPool(assetId)
	if err != nil {
		return "", err
	}

	if pool2.Data.Pool.KLVBalance-pool1.Data.Pool.KLVBalance != amountToDeposit {
		return "", fmt.Errorf("the deposit amount is different")
	}

	return assetId, nil
}

func kdaFeePool_KDA_DepositAndWithdraw(args common.TestArgs) error {
	fmt.Println("KDA Fee Pool KDA Deposit and Withdraw")
	// Create a asset and activate the feepool of this asset
	assetId, err := CreateKdaFeePool(args, "1000", "100000")
	if err != nil {
		return err
	}

	// Check the balance of the activate fee pool
	// probably return  0 because we don't deposit any asset yet
	pool1, _ := common.GetPool(assetId)

	amountToDeposit := float64(10_000_000)

	// Deposit 10 KLV
	err = DepositKdaFeePool(args, assetId, assetId, amountToDeposit)
	if err != nil {
		return err
	}

	time.Sleep(4 * time.Second) // wait for the next slot

	pool2, err := common.GetPool(assetId)
	if err != nil {
		return err
	}

	if pool2.Data.Pool.KDABalance-pool1.Data.Pool.KLVBalance != amountToDeposit {
		return fmt.Errorf("the deposit amount is different")
	}

	amountToWithdraw := float64(5_000_000)

	err = WithdrawKdaFeePool(args, assetId, assetId, amountToWithdraw)
	if err != nil {
		return err
	}

	time.Sleep(4 * time.Second) // wait for the next slot

	pool3, err := common.GetPool(assetId)
	if err != nil {
		return err
	}

	if pool2.Data.Pool.KDABalance-pool3.Data.Pool.KDABalance != amountToWithdraw {
		return fmt.Errorf("the withdraw amount is different")
	}

	return nil
}

func kdaFeePool_KLV_KDA_Deposit(args common.TestArgs) (string, error) {
	fmt.Println("KDA Fee Pool KLV and KDA Deposit")
	// Create a asset and activate the feepool of this asset
	assetId, err := CreateKdaFeePool(args, "1", "100")
	if err != nil {
		return "", err
	}

	// Check the balance of the activate fee pool
	// probably return  0 because we don't deposit any asset yet
	pool1, _ := common.GetPool(assetId)

	amountKDAToDeposit := float64(10_000_000)

	// Deposit 10 KDA
	err = DepositKdaFeePool(args, assetId, assetId, amountKDAToDeposit)
	if err != nil {
		return "", err
	}

	time.Sleep(4 * time.Second) // wait for the next slot

	pool2, err := common.GetPool(assetId)
	if err != nil {
		return "", err
	}

	if pool2.Data.Pool.KDABalance-pool1.Data.Pool.KDABalance != amountKDAToDeposit {
		return "", fmt.Errorf("the deposit amount is different")
	}

	pool1, _ = common.GetPool(assetId)

	amountKLVToDeposit := float64(10_000_000)

	// Deposit 10 KLV
	err = DepositKdaFeePool(args, assetId, "KLV", amountKLVToDeposit)
	if err != nil {
		return "", err
	}

	time.Sleep(4 * time.Second) // wait for the next slot

	pool2, err = common.GetPool(assetId)
	if err != nil {
		return "", err
	}

	if pool2.Data.Pool.KLVBalance-pool1.Data.Pool.KLVBalance != amountKLVToDeposit {
		return "", fmt.Errorf("the deposit amount is different")
	}

	return assetId, nil
}

func kdaFeePool_Transfer_KDA(args common.TestArgs) error {
	// Create a asset and activate the feepool of this asset
	assetId, err := createKdaFeePool_KLV_Deposit(args)
	if err != nil {
		return err
	}

	_, _, addr, err := utils.LoadKey("wallet-generated-1.pem")
	if err != nil {
		return err
	}

	tx, err := SendKDAPayFeesWithKDA(args, args.KeyFile, addr, "1", assetId)
	if err != nil {
		return err
	}

	time.Sleep(3 * time.Second)

	fees := tx.KAppFee + tx.BandwidthFee

	pool, err := common.GetPool(assetId)
	if err != nil {
		return err
	}

	if float64(fees) != pool.Data.Pool.KDABalance {
		return fmt.Errorf("the value of fees does not match de value of kda on pool")
	}

	return nil
}

func kdaFeePool_Transfer_KLV(args common.TestArgs) error {
	// Create a asset and activate the feepool of this asset
	assetId, err := createKdaFeePool_KLV_Deposit(args)
	if err != nil {
		return err
	}

	_, _, addr, err := utils.LoadKey("wallet-generated-1.pem")
	if err != nil {
		return err
	}

	tx, err := SendPayFeesWithKDA(args, args.KeyFile, addr, "1", assetId)
	if err != nil {
		return err
	}

	fees := tx.KAppFee + tx.BandwidthFee

	time.Sleep(3 * time.Second)

	pool, err := common.GetPool(assetId)
	if err != nil {
		return err
	}

	if float64(fees) != pool.Data.Pool.KDABalance {
		return fmt.Errorf("the value of fees does not match de value of kda on pool")
	}

	return nil
}

func kdaFeePool_PoolOutOfFunds(args common.TestArgs) error {
	_, _, addr1, err := utils.LoadKey("wallet-generated-1.pem")
	if err != nil {
		log.Fatalln(err)
	}

	// add sender
	account1, err := account.NewAccount(addr1,
		account.WithKeyFile("wallet-generated-1.pem"))
	if err != nil {
		log.Fatalln(err)
	}

	// add receiver
	_, _, addr2, err := utils.LoadKey("wallet-generated-2.pem")
	if err != nil {
		log.Fatalln(err)
	}

	// Create a asset and activate the feepool of this asset
	assetId, err := CreateKdaFeePool(args, "1", "1")
	if err != nil {
		return err
	}

	// send klv to account 1
	sendAsset1, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "account", "send", account1.Address, fmt.Sprint(1000))
	if err != nil {
		log.Fatalln(err)
	}

	status, _, err := common.CheckTransactions(0, sendAsset1)
	if err != nil {
		log.Fatalln(err)
	}

	if status[0] != "Ok" {
		log.Fatalf("send KLV transactions is not ok. Tx1: %s\n", status[0])
	}

	_, err = SendPayFeesWithKDA(args, account1.KeyFile, addr2, "1", assetId)
	if err != nil {

		return nil
	}

	return fmt.Errorf("tx need to return error because the pool is outOfFunds")
}

func kdaFeePool_WithNFT(args common.TestArgs) error {
	_, _, addr, err := utils.LoadKey(args.KeyFile)
	if err != nil {
		return err
	}

	createAssetHash, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "kda", "create", "1",
		"--name", "Test", "--ticker", "TST", "--maxSupply", "10000", "--canFreeze", "--canPause", "--canMint", "--canBurn", "--canChangeOwner", "--canAddRoles")
	if err != nil {
		return err
	}

	status, tx, err := common.CheckTransaction(createAssetHash, 0)
	if err != nil {
		return err
	}

	if status != "Ok" {
		return fmt.Errorf("tx is not success")
	}

	assetId, err := common.GetAssetId(tx)
	if err != nil {
		return err
	}

	ratio := "1"
	//try to activate kda pool in a NFT
	triggerHash, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "kda", "trigger", "15",
		fmt.Sprintf("--kdaID=%s", assetId), "--updateKdaPool", fmt.Sprintf("active=true,adminAddress=%s,fixedRatioKLV=%s,fixedRatioKDA=%s", addr, ratio, ratio))
	if err != nil {
		return err
	}

	status, _, err = common.CheckTransaction(triggerHash, 0)
	if err != nil {
		return err
	}

	if status != "Ok" {
		return nil
	}

	return fmt.Errorf("tx need to return error")
}

func kdaFeePool_KappPayWithKDA(args common.TestArgs) error {
	// Create a asset and activate the feepool of this asset
	assetId, err := createKdaFeePool_KLV_Deposit(args)
	if err != nil {
		return err
	}

	err = configITOButPayedWithKDA(args, assetId)
	if err != nil {
		return err
	}

	return nil
}

func kdaFeePool_AdminAddress_ChangePoolData(args common.TestArgs) error {
	// bootstrap a aux accoutn
	_, _, addr1, err := utils.LoadKey("wallet-generated-1.pem")
	if err != nil {
		log.Fatalln(err)
	}

	account1, err := account.NewAccount(addr1,
		account.WithSync(), account.WithKeyFile("wallet-generated-1.pem"))
	if err != nil {
		log.Fatalln(err)
	}

	amount := 10000

	// Send KLV to the aux account
	sendAsset1, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "account", "send", account1.Address, fmt.Sprint(amount),
		"--kda", "KLV")
	if err != nil {
		log.Fatalln(err)
	}

	status, _, err := common.CheckTransaction(sendAsset1, 0)
	if err != nil {
		log.Fatalln(err)
	}

	if status != "Ok" {
		return nil
	}

	if err := account1.Sync(); err != nil {
		return nil
	}
	// Create a asset and activate the feepool of this asset
	assetId, err := CreateKdaFeePool(args, "1", "1")
	if err != nil {
		return err
	}

	// Update Admin Address in kdaPool
	err = UpdateKdaPool(args, args.KeyFile, assetId, account1.Address, "1", "1")
	if err != nil {
		log.Fatalln(err)
	}

	oldPool, err := common.GetPool(assetId)
	if err != nil {
		log.Fatalln(err)
	}

	// After Update, the admin address account can be change kda pool data.
	err = UpdateKdaPool(args, account1.KeyFile, assetId, account1.Address, "1", "10")
	if err != nil {
		log.Fatalln(err)
	}

	newPool, err := common.GetPool(assetId)
	if err != nil {
		log.Fatalln(err)
	}

	if oldPool.Data.Pool.FRatioKLV == newPool.Data.Pool.FRatioKLV && oldPool.Data.Pool.FRatioKDA != newPool.Data.Pool.FRatioKDA {
		return nil
	}

	return fmt.Errorf("pool does not update")

}

func kdaFeePool_AdminAddress_OwnerChangePoolData(args common.TestArgs) error {
	// bootstrap a aux accoutn
	_, _, addr1, err := utils.LoadKey("wallet-generated-1.pem")
	if err != nil {
		log.Fatalln(err)
	}

	account1, err := account.NewAccount(addr1,
		account.WithSync(), account.WithKeyFile("wallet-generated-1.pem"))
	if err != nil {
		log.Fatalln(err)
	}

	amount := 10000

	// Send KLV to the aux account
	sendAsset1, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "account", "send", account1.Address, fmt.Sprint(amount),
		"--kda", "KLV")
	if err != nil {
		log.Fatalln(err)
	}

	status, _, err := common.CheckTransaction(sendAsset1, 0)
	if err != nil {
		log.Fatalln(err)
	}

	if status != "Ok" {
		return nil
	}

	if err := account1.Sync(); err != nil {
		return nil
	}
	// Create a asset and activate the feepool of this asset
	assetId, err := CreateKdaFeePool(args, "1", "1")
	if err != nil {
		return err
	}

	fmt.Println(assetId)

	// Update Admin Address in kdaPool
	err = UpdateKdaPool(args, args.KeyFile, assetId, account1.Address, "1", "1")
	if err != nil {
		log.Fatalln(err)
	}

	oldPool, _ := common.GetPool(assetId)

	// even changed the adminAddress, the owner still can change de values from kdaPool
	err = UpdateKdaPool(args, args.KeyFile, assetId, account1.Address, "1", "10")
	if err != nil {
		log.Fatalln(err)
	}

	newPool, _ := common.GetPool(assetId)

	if oldPool.Data.Pool.FRatioKLV == newPool.Data.Pool.FRatioKLV && oldPool.Data.Pool.FRatioKDA != newPool.Data.Pool.FRatioKDA {
		return nil
	}

	return fmt.Errorf("pool does not update")

}

func kdaFeePool_AdminAddress_WrongAdmin(args common.TestArgs) error {
	// bootstrap a aux accoutn
	_, _, addr1, err := utils.LoadKey("wallet-generated-1.pem")
	if err != nil {
		log.Fatalln(err)
	}

	account1, err := account.NewAccount(addr1,
		account.WithSync(), account.WithKeyFile("wallet-generated-1.pem"))
	if err != nil {
		log.Fatalln(err)
	}

	amount := 10000

	// Send KLV to the aux account
	sendAsset1, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "account", "send", account1.Address, fmt.Sprint(amount),
		"--kda", "KLV")
	if err != nil {
		log.Fatalln(err)
	}

	status, _, err := common.CheckTransaction(sendAsset1, 0)
	if err != nil {
		log.Fatalln(err)
	}

	if status != "Ok" {
		return nil
	}

	if err := account1.Sync(); err != nil {
		return nil
	}
	// Create a asset and activate the feepool of this asset
	assetId, err := CreateKdaFeePool(args, "1", "1")
	if err != nil {
		return err
	}

	err = UpdateKdaPool(args, account1.KeyFile, assetId, account1.Address, "1", "100")
	if err != nil {
		if err.Error() == "tx is not success" {
			return nil // tx need to fail because the keyfile passed is not from admin address or owner.
		}
		return err
	}

	return fmt.Errorf("tx should be wrong because account1 is not the owner of the pool")
}

func kdaFeePool_KLV_NotAvailableBalance(args common.TestArgs) error {
	// Create a asset and activate the feepool of this asset
	assetID, err := kdaFeePool_KLV_KDA_Deposit(args)
	if err != nil {
		return err
	}

	// create a new address

	walletName, address, err := CreateWallet()
	if err != nil {
		return err
	}
	// send 1KLV to activate aux account

	sendKLV, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "account", "send", address, fmt.Sprint(1))
	if err != nil {
		log.Fatalln(err)
	}

	status, _, err := common.CheckTransaction(sendKLV, 0)
	if err != nil {
		log.Fatalln(err)
	}

	if status != "Ok" {
		log.Fatalln("tx is not success.")
	}
	// Send kda

	sendKDA, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "account", "send", fmt.Sprintf("--kda=%s", assetID), address, fmt.Sprint(1000))
	if err != nil {
		log.Fatalln(err)
	}

	status, _, err = common.CheckTransaction(sendKDA, 0)
	if err != nil {
		log.Fatalln(err)
	}

	if status != "Ok" {
		log.Fatalln("tx is not success.")
	}

	// Sync aux

	acc, err := account.NewAccount(address,
		account.WithSync(), account.WithKeyFile(walletName))
	if err != nil {
		log.Fatalln(err)
	}

	// Try to send a tx

	_, _, addr, err := utils.LoadKey(args.KeyFile)
	if err != nil {
		return err
	}

	sendTX, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", acc.KeyFile), "account", "send", fmt.Sprintf("--kda=%s", assetID), addr, fmt.Sprint(10), fmt.Sprintf("--kdaFee=%s", assetID))
	if err != nil {
		log.Fatalln(err)
	}

	status, _, err = common.CheckTransaction(sendTX, 0)
	if err != nil {
		log.Fatalln(err)
	}

	if status != "Ok" {
		log.Fatalln("tx is not success.")
	}

	return nil

}

func RunTests(args common.TestArgs) {
	err := kdaFeePool_Transfer_KDA(args)
	if err != nil {
		log.Fatalln(err)
	}

	err = kdaFeePool_Transfer_KLV(args)
	if err != nil {
		log.Fatalln(err)
	}

	err = kdaFeePool_KDA_DepositAndWithdraw(args)
	if err != nil {
		log.Fatalln(err)
	}

	err = kdaFeePool_KLV_DepositAndWithdraw(args)
	if err != nil {
		log.Fatalln(err)
	}

	_, err = kdaFeePool_KLV_KDA_Deposit(args)
	if err != nil {
		log.Fatalln(err)
	}

	err = kdaFeePool_PoolOutOfFunds(args)
	if err != nil {
		log.Fatalln(err)
	}

	err = kdaFeePool_WithNFT(args)
	if err != nil {
		log.Fatalln(err)
	}

	err = kdaFeePool_KappPayWithKDA(args)
	if err != nil {
		log.Fatalln(err)
	}

	// AdminAddress Fork Tests
	err = kdaFeePool_AdminAddress_ChangePoolData(args)
	if err != nil {
		log.Fatalln(err)
	}

	err = kdaFeePool_AdminAddress_OwnerChangePoolData(args)
	if err != nil {
		log.Fatalln(err)
	}

	err = kdaFeePool_AdminAddress_WrongAdmin(args)
	if err != nil {
		log.Fatalln(err)
	}

	err = kdaFeePool_KLV_NotAvailableBalance(args)
	if err != nil {
		log.Fatal(err)
	}

}
