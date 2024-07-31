package vmjsonintegrationtest

import (
	"testing"
)

func TestRustAdder(t *testing.T) {
	ScenariosTest(t).
		Folder("adder/scenarios").
		Run().
		CheckNoError()
}

func TestRustFactorial(t *testing.T) {
	ScenariosTest(t).
		Folder("factorial/scenarios").
		Run().
		CheckNoError()
}

func TestRustErc20(t *testing.T) {
	ScenariosTest(t).
		Folder("erc20-rust/scenarios").
		Run().
		CheckNoError()
}

func TestCErc20(t *testing.T) {
	ScenariosTest(t).
		Folder("erc20-c").
		Run().
		CheckNoError()
}

func TestDigitalCash(t *testing.T) {
	ScenariosTest(t).
		Folder("digital-cash").
		Run().
		CheckNoError()
}

func TestCrowdfundingKda(t *testing.T) {
	ScenariosTest(t).
		Folder("crowdfunding-kda").
		Run().
		CheckNoError()
}

func TestWKlvSwap(t *testing.T) {
	ScenariosTest(t).
		Folder("wklv-swap").
		Run().
		CheckNoError()
}

func TestRustAttestation(t *testing.T) {
	if testing.Short() {
		t.Skip("not a short test")
	}

	ScenariosTest(t).
		Folder("attestation-rust").
		Run().
		CheckNoError()
}

func TestCAttestation(t *testing.T) {
	if testing.Short() {
		t.Skip("not a short test")
	}

	ScenariosTest(t).
		Folder("attestation-c").
		Run().
		CheckNoError()
}
