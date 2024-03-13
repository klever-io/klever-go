package common

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"
)

func ExecAndGetHash(name string, args ...string) (string, error) {
	time.Sleep(2000 * time.Millisecond)

	watcher := make(chan string)

	var err error

	go func() {
		err = ExecAndPrint(watcher, false, name, args...)
	}()

	for {
		s, ok := <-watcher
		if !ok {
			if err != nil {
				return "", err
			}

			return "", fmt.Errorf("end of file, nothing found")
		}

		// sleep for logs
		time.Sleep(100 * time.Millisecond)

		hash := GetTransactionHash(s)
		if hash != "" {
			return hash, nil
		}
	}
}

func Exec(name string, args ...string) error {
	time.Sleep(2000 * time.Millisecond)
	cmd := exec.Command(name, args...)

	err := cmd.Start()
	if err != nil {
		return err
	}
	return nil
}

func printEnd() {
	fmt.Println()
	fmt.Println(" ----------------------  End  ---------------------- ")
	fmt.Println()
}

func ExecAndPrint(output chan<- string, silence bool, name string, args ...string) error {
	defer func() {
		if output != nil {
			close(output)
		}
	}()

	cmd := exec.Command(name, args...)

	fmt.Println(" ----------------  Running command  ---------------- ")
	fmt.Println()

	fmt.Println(cmd.String())
	fmt.Println()

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		printEnd()
		return err
	}

	err = cmd.Start()
	if err != nil {
		printEnd()

		return err
	}

	scanner := bufio.NewScanner(stdout)

	for scanner.Scan() {
		if output != nil {
			output <- scanner.Text()
		}
		if !silence {
			fmt.Println(scanner.Text())
		}
	}

	if err := scanner.Err(); err != nil {
		printEnd()

		return err
	}

	printEnd()

	return nil
}

func GetTransactionHash(text string) string {
	if strings.Contains(text, "successful") {
		v := strings.Split(text, ParseColor("txHash")+" = ")

		if len(v) == 2 {
			return strings.TrimRight(v[1], " ")
		}
	}

	return ""
}

func ParseColor(text string) string {
	return "\u001B[0;32m" + text + "\u001B[0m"
}

var Args *TestArgs

type TestArgs struct {
	NodeUrl         string
	ProxyUrl        string
	KeyFile         string
	Deployed        bool
	DeployedAddress string
	DelegateAddress string
}

func Bootstrap() TestArgs {
	var node string
	var proxy string
	var deployed bool
	var deployedAddress string
	var keyFile string
	var delegateAddress string
	var retry uint

	flag.StringVar(&node, "node", "https://node.testnet.klever.finance", "node url")
	flag.StringVar(&proxy, "proxy", "https://api.testnet.klever.finance", "proxy url")
	flag.StringVar(&keyFile, "key-file", "./walletKey.pem", "key file")
	flag.UintVar(&retry, "retry", 25, "retry")
	flag.BoolVar(&deployed, "deployed", false, "contract already deployed")
	flag.StringVar(&deployedAddress, "deployed-address", "", "deployed contract address")
	flag.StringVar(&delegateAddress, "delegate-address", "", "some validator address")

	flag.Parse()

	if err := os.Setenv("KLEVER_NODE", node); err != nil {
		log.Fatalln(err)
	}

	if err := os.Setenv("KLEVER_PROXY", proxy); err != nil {
		log.Fatalln(err)
	}

	if err := os.Setenv("RETRY", fmt.Sprintf("%d", retry)); err != nil {
		log.Fatalln(err)
	}

	args := TestArgs{
		NodeUrl:         node,
		ProxyUrl:        proxy,
		KeyFile:         keyFile,
		Deployed:        deployed,
		DeployedAddress: deployedAddress,
		DelegateAddress: delegateAddress,
	}

	Args = &args

	return args
}

func GenerateAccounts() {
	keys := os.Getenv("E2E_PRIVATE_KEYS")
	if keys == "" {
		log.Fatalln("cannot get test private keys")
	}

	for i, pk := range strings.Split(keys, ",") {
		filename := fmt.Sprintf("wallet-generated-%d.pem", i)
		if i == 0 {
			filename = "walletKey.pem"
		}

		if _, err := os.Stat(filename); !os.IsNotExist(err) {
			fmt.Println("File already exists: ", filename)
			continue
		}

		err := ExecAndPrint(nil, true, "go", "run", "./cmd/operator/", "account", "import-sk", pk, "--path", filename)
		if err != nil {
			log.Fatalln(err)
		}
	}
}
