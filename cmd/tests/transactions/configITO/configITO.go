package configITO

import (
	"fmt"
	"log"
	"time"

	"github.com/klever-io/klever-go/cmd/tests/utils"

	"github.com/klever-io/klever-go/cmd/tests/common"
	"github.com/klever-io/klever-go/data/transaction"
)

func ConfigITOShouldWork(args common.TestArgs) {
	_, _, testWalletAddress, err := utils.LoadKey(args.KeyFile)
	if err != nil {
		log.Fatalln(err)
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
	startTime := time.Now().Add(time.Minute).Unix()
	endTime := time.Now().AddDate(0, 0, 3).Unix()

	whitelistStartTime := time.Now().Add(time.Minute).Unix()
	whitelistEndTime := time.Now().AddDate(0, 0, 3).Unix()

	configITOHash, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "kapps", "config-ito", fmt.Sprintf("--kda=%s", assetId), "--maxAmount=1000",
		fmt.Sprintf("--receiver=%s", testWalletAddress), "--status=1", `--packs={"kda": "KLV", "packs": [{"amount":1, "price":20},{"amount":10, "price":150}]}`, fmt.Sprintf("--startTime=%d", startTime),
		fmt.Sprintf("--endTime=%d", endTime), "--whitelist=klv1rsm4vx8pdg2cy74zz9gwff0avl7vwunzy9tv7utn8eey0chtdaasectfw4=10,klv1fpwjz234gy8aaae3gx0e8q9f52vymzzn3z5q0s5h60pvktzx0n0qwvtux5=20",
		"--whitelistStatus=1", fmt.Sprintf("--whitelistStartTime=%d", whitelistStartTime), fmt.Sprintf("--whitelistEndTime=%d", whitelistEndTime))
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
}

func ConfigITOShouldError(args common.TestArgs) {
	_, _, testWalletAddress, err := utils.LoadKey(args.KeyFile)
	if err != nil {
		log.Fatalln(err)
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

	startTime := time.Now().Add(time.Minute).Unix()
	endTime := time.Now().AddDate(0, 0, 3).Unix()

	whitelistStartTime := time.Now().Add(time.Minute).Unix()
	whitelistEndTime := time.Now().AddDate(0, 0, 3).Unix()

	// config ito
	// err: invalid asset
	invalidAsset := "INVALID"
	configITOHash, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "kapps", "config-ito", fmt.Sprintf("--kda=%s", invalidAsset), "--maxAmount=1000",
		fmt.Sprintf("--receiver=%s", testWalletAddress), "--status=1", `--packs={"kda": "KLV", "packs": [{"amount":1, "price":20},{"amount":10, "price":150}]}`, fmt.Sprintf("--startTime=%d", startTime),
		fmt.Sprintf("--endTime=%d", endTime), "--whitelist=klv1rsm4vx8pdg2cy74zz9gwff0avl7vwunzy9tv7utn8eey0chtdaasectfw4=10,klv1fpwjz234gy8aaae3gx0e8q9f52vymzzn3z5q0s5h60pvktzx0n0qwvtux5=20",
		"--whitelistStatus=1", fmt.Sprintf("--whitelistStartTime=%d", whitelistStartTime), fmt.Sprintf("--whitelistEndTime=%d", whitelistEndTime))
	if err != nil {
		log.Fatalln(err)
	}

	_, tx, err := common.CheckTransaction(configITOHash, 0)
	if err != nil {
		log.Fatalln(err)
	}

	if tx.Status != "fail" && tx.ResultCode != transaction.Transaction_KAPPError.String() {
		log.Fatalln("tx with invalid asset succeded!")
	}

	// err: invalid receiver address
	invalidAddress := "INVALID"
	_, err = common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "kapps", "config-ito", fmt.Sprintf("--kda=%s", invalidAsset), "--maxAmount=1000",
		fmt.Sprintf("--receiver=%s", invalidAddress), "--status=1", `--packs={"kda": "KLV", "packs": [{"amount":1, "price":20},{"amount":10, "price":150}]}`, fmt.Sprintf("--startTime=%d", startTime),
		fmt.Sprintf("--endTime=%d", endTime), "--whitelist=klv1rsm4vx8pdg2cy74zz9gwff0avl7vwunzy9tv7utn8eey0chtdaasectfw4=10,klv1fpwjz234gy8aaae3gx0e8q9f52vymzzn3z5q0s5h60pvktzx0n0qwvtux5=20",
		"--whitelistStatus=1", fmt.Sprintf("--whitelistStartTime=%d", whitelistStartTime), fmt.Sprintf("--whitelistEndTime=%d", whitelistEndTime))
	if err == nil {
		log.Fatalln(err)
	}

	// error: invalid status
	_, err = common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "kapps", "config-ito", fmt.Sprintf("--kda=%s", assetId), "--maxAmount=1000",
		fmt.Sprintf("--receiver=%s", testWalletAddress), "--status=22", `--packs={"kda": "KLV", "packs": [{"amount":1, "price":20},{"amount":10, "price":150}]}`, fmt.Sprintf("--startTime=%d", startTime),
		fmt.Sprintf("--endTime=%d", endTime), "--whitelist=klv1rsm4vx8pdg2cy74zz9gwff0avl7vwunzy9tv7utn8eey0chtdaasectfw4=10,klv1fpwjz234gy8aaae3gx0e8q9f52vymzzn3z5q0s5h60pvktzx0n0qwvtux5=20",
		"--whitelistStatus=1", fmt.Sprintf("--whitelistStartTime=%d", whitelistStartTime), fmt.Sprintf("--whitelistEndTime=%d", whitelistEndTime))
	if err == nil {
		log.Fatalln(err)
	}

	// error: invalid max amount
	_, err = common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "kapps", "config-ito", fmt.Sprintf("--kda=%s", assetId), "--maxAmount=-1000",
		fmt.Sprintf("--receiver=%s", testWalletAddress), "--status=1", `--packs={"kda": "KLV", "packs": [{"amount":1, "price":20},{"amount":10, "price":150}]}`, fmt.Sprintf("--startTime=%d", startTime),
		fmt.Sprintf("--endTime=%d", endTime), "--whitelist=klv1rsm4vx8pdg2cy74zz9gwff0avl7vwunzy9tv7utn8eey0chtdaasectfw4=10,klv1fpwjz234gy8aaae3gx0e8q9f52vymzzn3z5q0s5h60pvktzx0n0qwvtux5=20",
		"--whitelistStatus=1", fmt.Sprintf("--whitelistStartTime=%d", whitelistStartTime), fmt.Sprintf("--whitelistEndTime=%d", whitelistEndTime))
	if err == nil {
		log.Fatalln(err)
	}

	// error: invalid pack price
	_, err = common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "kapps", "config-ito", fmt.Sprintf("--kda=%s", assetId), "--maxAmount=1000",
		fmt.Sprintf("--receiver=%s", testWalletAddress), "--status=1", `--packs={"kda": "KLV", "packs": [{"amount":1, "price":-20},{"amount":10, "price":150}]}`, fmt.Sprintf("--startTime=%d", startTime),
		fmt.Sprintf("--endTime=%d", endTime), "--whitelist=klv1rsm4vx8pdg2cy74zz9gwff0avl7vwunzy9tv7utn8eey0chtdaasectfw4=10,klv1fpwjz234gy8aaae3gx0e8q9f52vymzzn3z5q0s5h60pvktzx0n0qwvtux5=20",
		"--whitelistStatus=1", fmt.Sprintf("--whitelistStartTime=%d", whitelistStartTime), fmt.Sprintf("--whitelistEndTime=%d", whitelistEndTime))
	if err == nil {
		log.Fatalln(err)
	}

	// error: invalid pack amount
	_, err = common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "kapps", "config-ito", fmt.Sprintf("--kda=%s", assetId), "--maxAmount=1000",
		fmt.Sprintf("--receiver=%s", testWalletAddress), "--status=1", `--packs={"kda": "KLV", "packs": [{"amount":-1, "price":20},{"amount":10, "price":150}]}`, fmt.Sprintf("--startTime=%d", startTime),
		fmt.Sprintf("--endTime=%d", endTime), "--whitelist=klv1rsm4vx8pdg2cy74zz9gwff0avl7vwunzy9tv7utn8eey0chtdaasectfw4=10,klv1fpwjz234gy8aaae3gx0e8q9f52vymzzn3z5q0s5h60pvktzx0n0qwvtux5=20",
		"--whitelistStatus=1", fmt.Sprintf("--whitelistStartTime=%d", whitelistStartTime), fmt.Sprintf("--whitelistEndTime=%d", whitelistEndTime))
	if err == nil {
		log.Fatalln(err)
	}

	// error: inexistent asset on a pack
	configITOHash1, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "kapps", "config-ito", fmt.Sprintf("--kda=%s", assetId), "--maxAmount=1000",
		fmt.Sprintf("--receiver=%s", testWalletAddress), "--status=1", `--packs={"kda": "INVALID", "packs": [{"amount":1, "price":20},{"amount":10, "price":150}]}`, fmt.Sprintf("--startTime=%d", startTime),
		fmt.Sprintf("--endTime=%d", endTime), "--whitelist=klv1rsm4vx8pdg2cy74zz9gwff0avl7vwunzy9tv7utn8eey0chtdaasectfw4=10,klv1fpwjz234gy8aaae3gx0e8q9f52vymzzn3z5q0s5h60pvktzx0n0qwvtux5=20",
		"--whitelistStatus=1", fmt.Sprintf("--whitelistStartTime=%d", whitelistStartTime), fmt.Sprintf("--whitelistEndTime=%d", whitelistEndTime))
	if err != nil {
		log.Fatalln(err)
	}

	_, tx, err = common.CheckTransaction(configITOHash1, 0)
	if err != nil {
		log.Fatalln(err)
	}

	if tx.Status != "fail" && tx.ResultCode != transaction.Transaction_KAPPError.String() {
		log.Fatalln("tx with invalid asset succeded!")
	}

	// error: empty pack
	_, err = common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "kapps", "config-ito", fmt.Sprintf("--kda=%s", assetId), "--maxAmount=1000",
		fmt.Sprintf("--receiver=%s", testWalletAddress), "--status=1", `--packs={"kda": "", "packs": [{}]}`, fmt.Sprintf("--startTime=%d", startTime),
		fmt.Sprintf("--endTime=%d", endTime), "--whitelist=klv1rsm4vx8pdg2cy74zz9gwff0avl7vwunzy9tv7utn8eey0chtdaasectfw4=10,klv1fpwjz234gy8aaae3gx0e8q9f52vymzzn3z5q0s5h60pvktzx0n0qwvtux5=20",
		"--whitelistStatus=1", fmt.Sprintf("--whitelistStartTime=%d", whitelistStartTime), fmt.Sprintf("--whitelistEndTime=%d", whitelistEndTime))
	if err == nil {
		log.Fatalln(err)
	}

	// error: invalid default limit per address
	_, err = common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "kapps", "config-ito", fmt.Sprintf("--kda=%s", assetId), "--maxAmount=1000",
		fmt.Sprintf("--receiver=%s", testWalletAddress), "--status=1", `--packs={"kda": "KLV", "packs": [{"amount":1, "price":20},{"amount":10, "price":150}]}`, fmt.Sprintf("--startTime=%d", startTime),
		fmt.Sprintf("--endTime=%d", endTime), "--whitelist=klv1rsm4vx8pdg2cy74zz9gwff0avl7vwunzy9tv7utn8eey0chtdaasectfw4=10,klv1fpwjz234gy8aaae3gx0e8q9f52vymzzn3z5q0s5h60pvktzx0n0qwvtux5=20",
		"--whitelistStatus=1", fmt.Sprintf("--whitelistStartTime=%d", whitelistStartTime), fmt.Sprintf("--whitelistEndTime=%d", whitelistEndTime), fmt.Sprintf("--limitPerAddress=%d", -5))
	if err == nil {
		log.Fatalln(err)
	}

	// error: invalid whitelist status
	_, err = common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "kapps", "config-ito", fmt.Sprintf("--kda=%s", assetId), "--maxAmount=1000",
		fmt.Sprintf("--receiver=%s", testWalletAddress), "--status=1", `--packs={"kda": "KLV", "packs": [{"amount":1, "price":20},{"amount":10, "price":150}]}`, fmt.Sprintf("--startTime=%d", startTime),
		fmt.Sprintf("--endTime=%d", endTime), "--whitelist=klv1rsm4vx8pdg2cy74zz9gwff0avl7vwunzy9tv7utn8eey0chtdaasectfw4=10,klv1fpwjz234gy8aaae3gx0e8q9f52vymzzn3z5q0s5h60pvktzx0n0qwvtux5=20",
		"--whitelistStatus=22", fmt.Sprintf("--whitelistStartTime=%d", whitelistStartTime), fmt.Sprintf("--whitelistEndTime=%d", whitelistEndTime), fmt.Sprintf("--limitPerAddress=%d", 5))
	if err == nil {
		log.Fatalln(err)
	}

	// error: invalid whitelist address limit
	_, err = common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "kapps", "config-ito", fmt.Sprintf("--kda=%s", assetId), "--maxAmount=1000",
		fmt.Sprintf("--receiver=%s", testWalletAddress), "--status=1", `--packs={"kda": "KLV", "packs": [{"amount":1, "price":20},{"amount":10, "price":150}]}`, fmt.Sprintf("--startTime=%d", startTime),
		fmt.Sprintf("--endTime=%d", endTime), "--whitelist=klv1rsm4vx8pdg2cy74zz9gwff0avl7vwunzy9tv7utn8eey0chtdaasectfw4=-10,klv1fpwjz234gy8aaae3gx0e8q9f52vymzzn3z5q0s5h60pvktzx0n0qwvtux5=20",
		"--whitelistStatus=1", fmt.Sprintf("--whitelistStartTime=%d", whitelistStartTime), fmt.Sprintf("--whitelistEndTime=%d", whitelistEndTime), fmt.Sprintf("--limitPerAddress=%d", 5))
	if err == nil {
		log.Fatalln(err)
	}

	// error: invalid ito whitelist start and end time
	_, err = common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "kapps", "config-ito", fmt.Sprintf("--kda=%s", assetId), "--maxAmount=1000",
		fmt.Sprintf("--receiver=%s", testWalletAddress), "--status=1", `--packs={"kda": "KLV", "packs": [{"amount":1, "price":20},{"amount":10, "price":150}]}`, fmt.Sprintf("--startTime=%d", startTime),
		fmt.Sprintf("--endTime=%d", endTime), "--whitelist=klv1rsm4vx8pdg2cy74zz9gwff0avl7vwunzy9tv7utn8eey0chtdaasectfw4=10,klv1fpwjz234gy8aaae3gx0e8q9f52vymzzn3z5q0s5h60pvktzx0n0qwvtux5=20",
		"--whitelistStatus=1", fmt.Sprintf("--whitelistStartTime=%d", whitelistEndTime), fmt.Sprintf("--whitelistEndTime=%d", whitelistStartTime), fmt.Sprintf("--limitPerAddress=%d", 5))
	if err == nil {
		log.Fatalln(err)
	}

	// error: invalid ito start and end time
	_, err = common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "kapps", "config-ito", fmt.Sprintf("--kda=%s", assetId), "--maxAmount=1000",
		fmt.Sprintf("--receiver=%s", testWalletAddress), "--status=1", `--packs={"kda": "KLV", "packs": [{"amount":1, "price":20},{"amount":10, "price":150}]}`, fmt.Sprintf("--startTime=%d", endTime),
		fmt.Sprintf("--endTime=%d", startTime), "--whitelist=klv1rsm4vx8pdg2cy74zz9gwff0avl7vwunzy9tv7utn8eey0chtdaasectfw4=10,klv1fpwjz234gy8aaae3gx0e8q9f52vymzzn3z5q0s5h60pvktzx0n0qwvtux5=20",
		"--whitelistStatus=1", fmt.Sprintf("--whitelistStartTime=%d", whitelistStartTime), fmt.Sprintf("--whitelistEndTime=%d", whitelistEndTime), fmt.Sprintf("--limitPerAddress=%d", 5))
	if err == nil {
		log.Fatalln(err)
	}
}

func RunTests(args common.TestArgs) {
	ConfigITOShouldError(args)
	ConfigITOShouldWork(args)
}
