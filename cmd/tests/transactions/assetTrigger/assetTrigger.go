package assetTrigger

import (
	"fmt"
	"log"

	"github.com/klever-io/klever-go/cmd/tests/utils"

	"github.com/klever-io/klever-go/cmd/tests/common"
	"github.com/klever-io/klever-go/cmd/tests/common/account"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/kapps"
)

func AssetTrigger(args common.TestArgs) error {
	amount := 100000

	_, _, addr1, err := utils.LoadKey("wallet-generated-1.pem")
	if err != nil {
		return err
	}
	// add account
	account, err := account.NewAccount(addr1,
		account.WithKeyFile("wallet-generated-1.pem"))
	if err != nil {
		return err
	}

	// send the klv to the account
	sendAsset, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "account", "send", account.Address, fmt.Sprint(amount))
	if err != nil {
		return err
	}

	status, _, err := common.CheckTransaction(sendAsset, 0)
	if err != nil {
		return err
	}

	if status != "Ok" {
		return fmt.Errorf("tx is not success")
	}

	// create asset
	createAssetHash, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", account.KeyFile), "kda", "create", "1",
		"--name", "Test", "--ticker", "TST", "--maxSupply", "10000", "--canFreeze", "--canPause", "--canMint", "--canBurn", "--canChangeOwner", "--canAddRoles",
		"--royaltiesAddress=klv1g5khe6ec5yhwqd773gghc55q0kvpanevgqzj8h5r5sj2eeke2heqnt5dfp", `--royaltiesITOPercentage=5`,
		fmt.Sprintf("--splitRoyalties={\"address\": \"%s\", \"percentITOPercentage\": 10}", addr1),
		"--splitRoyalties={\"address\": \"klv1fpwjz234gy8aaae3gx0e8q9f52vymzzn3z5q0s5h60pvktzx0n0qwvtux5\", \"percentITOPercentage\": 20}")

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

	// mint one
	assetTriggerHash, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", account.KeyFile), "kda", "trigger", "0",
		fmt.Sprintf("--kdaID=%s", assetId), "--amount=1", fmt.Sprintf("--receiver=%s", account.Address))
	if err != nil {
		return err
	}

	status, _, err = common.CheckTransaction(assetTriggerHash, 0)
	if err != nil {
		return err
	}

	if status != "Ok" {
		return fmt.Errorf("tx is not success")
	}

	// trigger: stop royalties change
	// change royalties
	assetTriggerHash, err = common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", account.KeyFile), "kda", "trigger", "14",
		fmt.Sprintf("--kdaID=%s", assetId), fmt.Sprintf(`--splitRoyalties={"address": "%s", "percentITOPercentage": 5}`, addr1),
		`--splitRoyalties={"address": "klv1fpwjz234gy8aaae3gx0e8q9f52vymzzn3z5q0s5h60pvktzx0n0qwvtux5", "percentITOPercentage": 15}`)

	if err != nil {
		return err
	}

	status, _, err = common.CheckTransaction(assetTriggerHash, 0)
	if err != nil {
		return err
	}

	if status != "Ok" {
		return fmt.Errorf("tx is not success")
	}

	// stop royalties change
	assetTriggerHash, err = common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", account.KeyFile), "kda", "trigger", "16",
		fmt.Sprintf("--kdaID=%s", assetId))

	if err != nil {
		return err
	}

	status, _, err = common.CheckTransaction(assetTriggerHash, 0)
	if err != nil {
		return err
	}

	if status != "Ok" {
		return fmt.Errorf("tx is not success")
	}

	// change royalties after stop royalties change trigger
	assetTriggerHash, err = common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", account.KeyFile), "kda", "trigger", "14",
		fmt.Sprintf("--kdaID=%s", assetId), `--royaltiesITOPercentage=1`)

	if err != nil {
		return err
	}

	_, tx, err := common.CheckTransaction(assetTriggerHash, 0)
	if err != nil {
		return err
	}

	if tx.Status != "fail" && tx.ResultCode != transaction.Transaction_RoyaltiesChangeStopped.String() {
		return fmt.Errorf("tx updating royalties after stopped updates succeded")
	}

	// trigger: stop metadata change
	// change metadata
	assetTriggerHash, err = common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", account.KeyFile), "kda", "trigger", "8",
		fmt.Sprintf("--kdaID=%s", assetId), "--message=metadataUpdated", "--mime=testMime", "--nftID=1", fmt.Sprintf("--receiver=%s", account.Address))

	if err != nil {
		return err
	}

	status, _, err = common.CheckTransaction(assetTriggerHash, 0)
	if err != nil {
		return err
	}

	if status != "Ok" {
		return fmt.Errorf("tx is not success")
	}

	// stop metadata change
	assetTriggerHash, err = common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", account.KeyFile), "kda", "trigger", "17",
		fmt.Sprintf("--kdaID=%s", assetId))

	if err != nil {
		return err
	}

	status, _, err = common.CheckTransaction(assetTriggerHash, 0)
	if err != nil {
		return err
	}

	if status != "Ok" {
		return fmt.Errorf("tx is not success")
	}

	// change metadata after stop metadata change trigger
	assetTriggerHash, err = common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", account.KeyFile), "kda", "trigger", "8",
		fmt.Sprintf("--kdaID=%s", assetId), "--message=metadataUpdated2", "--mime=testMime2", "--nftID=1", fmt.Sprintf("--receiver=%s", account.Address))

	if err != nil {
		return err
	}

	_, tx, err = common.CheckTransaction(assetTriggerHash, 0)
	if err != nil {
		return err
	}

	if tx.Status != "fail" && tx.ResultCode != transaction.Transaction_NFTMetadataChangeStopped.String() {
		return fmt.Errorf("tx updating metadata after stopped updates succeded")
	}

	// try to stop royalties updates after it already stopped
	assetTriggerHash, err = common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", account.KeyFile), "kda", "trigger", "16",
		fmt.Sprintf("--kdaID=%s", assetId))

	if err != nil {
		return err
	}

	_, tx, err = common.CheckTransaction(assetTriggerHash, 0)
	if err != nil {
		return err
	}

	if tx.Status != "fail" && tx.ResultCode != transaction.Transaction_RoyaltiesChangeStopped.String() {
		return fmt.Errorf("tx stopping royalties updates after stopped updates succeded")
	}

	// try to stop metadata updates after it already stopped
	assetTriggerHash, err = common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", account.KeyFile), "kda", "trigger", "17",
		fmt.Sprintf("--kdaID=%s", assetId))

	if err != nil {
		return err
	}

	_, tx, err = common.CheckTransaction(assetTriggerHash, 0)
	if err != nil {
		return err
	}

	if tx.Status != "fail" && tx.ResultCode != transaction.Transaction_NFTMetadataChangeStopped.String() {
		return fmt.Errorf("tx stopping metadata updates after stopped updates succeded")
	}

	return nil
}

func CreateNftAndMint(args common.TestArgs) (string, error) {
	// create the KDA NFT Asset
	createHash, err := common.ExecAndGetHash("go", "run", "./cmd/operator/", fmt.Sprintf("--key-file=%s", args.KeyFile), "kda", "create", "1",
		"--name", "Test", "--ticker", "TST", "--maxSupply", "10000", "--canMint")
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

	mintHash, err := common.ExecAndGetHash("go", "run", "./cmd/operator", fmt.Sprintf("--key-file=%s", args.KeyFile), "kda", "trigger", "0", "--kdaID", assetId, "--amount", "1", "--receiver", "klv1vq9f7xtazuk9y3n46ukthgf2l30ev2s0qxvs6dfp4f2e76sfu3xshpmjsr")
	if err != nil {
		return "", err
	}

	_, _, err = common.CheckTransaction(mintHash, 0)
	if err != nil {
		return "", err
	}

	return assetId, nil
}

func assetTriggerMetadata(args common.TestArgs) error {
	assetId, err := CreateNftAndMint(args)
	if err != nil {
		return err
	}

	metadata := "TestUpdateMetadata"

	triggerHash, err := common.ExecAndGetHash("go", "run", "./cmd/operator", fmt.Sprintf("--key-file=%s", args.KeyFile), fmt.Sprintf("--message=%s", metadata), "kda", "trigger", "8", "--kdaID", assetId, "--nftID", "1", "--receiver", "klv1vq9f7xtazuk9y3n46ukthgf2l30ev2s0qxvs6dfp4f2e76sfu3xshpmjsr")
	if err != nil {
		return err
	}

	status, _, err := common.CheckTransaction(triggerHash, 0)
	if err != nil {
		return err
	}

	if status != "Ok" {
		return fmt.Errorf("tx is not success")
	}

	nftId := assetId + kapps.Sp + "1"

	nft, err := common.GetNFT(nftId, "klv1vq9f7xtazuk9y3n46ukthgf2l30ev2s0qxvs6dfp4f2e76sfu3xshpmjsr")
	if err != nil {
		return err
	}

	if string(nft.UserKDA.GetMetadata()) != metadata {
		return fmt.Errorf("metadata does not match")
	}

	return nil

}

func RunTests(args common.TestArgs) {
	err := assetTriggerMetadata(args)
	if err != nil {
		log.Fatalln(err)
	}
	err = AssetTrigger(args)
	if err != nil {
		log.Fatalln(err)
	}
}
