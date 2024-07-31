package vmjsonintegrationtest

import (
	"testing"
)

func TestRustAllocFeatures(t *testing.T) {
	if testing.Short() {
		t.Skip("not a short test")
	}

	ScenariosTest(t).
		Folder("features/alloc-features/scenarios").
		Run().
		CheckNoError()
}

func TestRustBasicFeaturesLatest(t *testing.T) {
	if testing.Short() {
		t.Skip("not a short test")
	}

	ScenariosTest(t).
		Folder("features/basic-features/scenarios").
		Exclude("features/basic-features/scenarios/storage_mapper_fungible_token.scen.json").
		Run().
		CheckNoError()
}

func TestRustBasicFeaturesNoSmallIntApi(t *testing.T) {
	if testing.Short() {
		t.Skip("not a short test")
	}

	ScenariosTest(t).
		Folder("features/basic-features-no-small-int-api/scenarios").
		Run().
		CheckNoError()
}

func TestRustBigFloatFeatures(t *testing.T) {
	if testing.Short() {
		t.Skip("not a short test")
	}

	ScenariosTest(t).
		Folder("features/big-float-features/scenarios").
		Run().
		CheckNoError()
}

func TestRustManagedMapFeatures(t *testing.T) {
	if testing.Short() {
		t.Skip("not a short test")
	}

	ScenariosTest(t).
		Folder("features/managed-map-features/scenarios").
		Run().
		CheckNoError()
}

func TestRustPayableFeaturesLatest(t *testing.T) {
	if testing.Short() {
		t.Skip("not a short test")
	}

	ScenariosTest(t).
		Folder("features/payable-features/scenarios").
		Run().
		CheckNoError()
}

func TestRustFormattedMessageFeatures(t *testing.T) {
	ScenariosTest(t).
		Folder("features/formatted-message-features/scenarios").
		Run().
		CheckNoError()
}

func TestTimelocks(t *testing.T) {
	ScenariosTest(t).
		Folder("timelocks").
		Run().
		CheckNoError()
}
