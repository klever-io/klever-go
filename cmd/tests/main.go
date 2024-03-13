package main

import (
	"fmt"
	"time"

	"github.com/klever-io/klever-go/cmd/tests/common"
	"github.com/klever-io/klever-go/cmd/tests/transactions/assetTrigger"
	"github.com/klever-io/klever-go/cmd/tests/transactions/buyITO"
	"github.com/klever-io/klever-go/cmd/tests/transactions/configITO"
	"github.com/klever-io/klever-go/cmd/tests/transactions/kdaFPR"
	"github.com/klever-io/klever-go/cmd/tests/transactions/kdaFeePool"
	"github.com/klever-io/klever-go/cmd/tests/transactions/kdaRoyalties"
	"github.com/klever-io/klever-go/cmd/tests/transactions/triggerITO"
	"github.com/klever-io/klever-go/cmd/tests/transactions/vmKapps"
)

func main() {
	common.GenerateAccounts()
	args := common.Bootstrap()

	fmt.Println("------- RUNNING FPR TEST ------- ")
	kdaFPR.RunTests(args)
	time.Sleep(time.Second * 4)

	fmt.Println("------- RUNNING KDA FEE TEST ------- ")
	kdaFeePool.RunTests(args)
	time.Sleep(time.Second * 4)

	fmt.Println("------- RUNNING ASSET TRIGGER TEST ------- ")
	assetTrigger.RunTests(args)
	time.Sleep(time.Second * 4)

	fmt.Println("------- RUNNING KDA ROYALTIES TEST ------- ")
	kdaRoyalties.RunTests(args)
	time.Sleep(time.Second * 4)

	fmt.Println("------- RUNNING CONFIG ITO TEST ------- ")
	configITO.RunTests(args)
	time.Sleep(time.Second * 4)

	fmt.Println("------- RUNNING BUY ITO TEST ------- ")
	buyITO.RunTests(args)
	time.Sleep(time.Second * 4)

	fmt.Println("------- RUNNING TRIGGER ITO TEST ------- ")
	triggerITO.RunTests(args)
	time.Sleep(time.Second * 4)

	fmt.Println("------- RUNNING KAPPS CUSTOM CONTRACT TEST ------- ")
	vmKapps.RunTests(args)
}
