package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/fatih/color"
	"github.com/gocarina/gocsv"
	"github.com/nwidger/jsoncolor"
)

type txDATA struct {
	Id        string  `csv:"id"` // .csv column headers
	Type      string  `csv:"type"`
	ToAddress string  `csv:"to_address"`
	KDAID     string  `csv:"kda_id"`
	Amount    float64 `csv:"amount"`
	TXHash    string  `csv:"tx_hash"`
}

func csvSend(fileName string) error {
	in, err := os.Open(filepath.Clean(fileName))
	if err != nil {
		return err
	}
	defer in.Close()

	txs := []*txDATA{}

	if err := gocsv.UnmarshalFile(in, &txs); err != nil {
		panic(err)
	}

	tokensToSend := make(map[string]float64)
	for _, tx := range txs {
		// sum values
		tokensToSend[tx.KDAID] += tx.Amount
	}

	// confirm
	fmt.Println("CSV batch send summary")
	fmt.Println("Total transfers: ", g(len(txs)))
	for id, total := range tokensToSend {
		fmt.Println(id, ": ", g(total))
	}

	fmt.Println("Please confirm")
	if askForConfirmation() {
		// dump updated csv
		defer writeCSV(fileName, txs)

		for _, tx := range txs {
			txHash, err := send(signerAddress, tx.ToAddress, tx.Amount, tx.KDAID)
			if err != nil {
				return err
			}
			// increament next nonce
			txNonce++
			tx.TXHash = txHash
			time.Sleep(100 * time.Millisecond)
		}
	} else {
		color.Red("Aborted by user")
	}

	return nil
}

func writeCSV(fileName string, data []*txDATA) {
	out, err := os.Create(filepath.Clean(fileName + ".output.csv"))
	if err != nil {
		color.Red("Error wrting output file: %+v", err)
		// dump
		dumpTXs(data)
		return
	}
	defer out.Close()

	err = gocsv.Marshal(data, out)
	if err != nil {
		color.Red("Error wrting output file: %+v", err)
		// dump
		dumpTXs(data)
	}
}

func dumpTXs(data []*txDATA) {
	// Make a custom formatter with indent set
	// create custom formatter
	f := jsoncolor.NewFormatter()
	f.Indent = "    "

	// marshal v with custom formatter,
	// dst contains colorized output
	dst, err := jsoncolor.MarshalIndentWithFormatter(data, "", "    ", f)
	if err != nil {
		return
	}

	// print colorized output to stdout
	fmt.Println(string(dst))
}

// before calling askForConfirmation. E.g. fmt.Println("WARNING: Are you sure? (yes/no)")
func askForConfirmation() bool {
	var response string
	_, err := fmt.Scanln(&response)
	if err != nil {
		return false
	}
	okayResponses := []string{"y", "Y", "yes", "Yes", "YES"}
	nokayResponses := []string{"n", "N", "no", "No", "NO"}
	if containsString(okayResponses, response) {
		return true
	} else if containsString(nokayResponses, response) {
		return false
	} else {
		fmt.Println("Please type yes or no and then press enter:")
		return askForConfirmation()
	}
}

// posString returns the first index of element in slice.
// If slice does not contain element, returns -1.
func posString(slice []string, element string) int {
	for index, elem := range slice {
		if elem == element {
			return index
		}
	}
	return -1
}

// containsString returns true iff slice contains element
func containsString(slice []string, element string) bool {
	return !(posString(slice, element) == -1)
}
