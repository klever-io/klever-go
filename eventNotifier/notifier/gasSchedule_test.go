package notifier_test

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core/process/smartContract/hooks/counters"
	"github.com/klever-io/klever-go/eventNotifier/notifier"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

const nodeGasSchedulePath = "../../config/node/gasScheduleV1.yaml"

type gasScheduleHandlerStub struct {
	received []map[string]map[string]uint64
}

func (h *gasScheduleHandlerStub) GasScheduleChange(gasSchedule map[string]map[string]uint64) {
	h.received = append(h.received, gasSchedule)
}

func (h *gasScheduleHandlerStub) IsInterfaceNil() bool {
	return h == nil
}

// writeGasSchedule writes the node's gas schedule to dir/name, with the given mutation applied
func writeGasSchedule(t *testing.T, dir string, name string, mutate func(map[string]map[string]uint64)) string {
	t.Helper()

	gasSchedule, err := config.LoadGasScheduleConfig(nodeGasSchedulePath)
	require.NoError(t, err)
	if mutate != nil {
		mutate(gasSchedule)
	}

	data, err := yaml.Marshal(gasSchedule)
	require.NoError(t, err)

	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, data, 0o600))

	return path
}

func dropI32Add(gasSchedule map[string]map[string]uint64) {
	delete(gasSchedule["WASMOpcodeCost"], "I32Add")
}

func bumpI32Add(gasSchedule map[string]map[string]uint64) {
	gasSchedule["WASMOpcodeCost"]["I32Add"]++
}

func newGasScheduleNotifierArgs(files ...string) notifier.ArgsNewGasScheduleNotifier {
	byEpochs := make([]config.GasScheduleByEpochs, 0, len(files))
	for i, file := range files {
		byEpochs = append(byEpochs, config.GasScheduleByEpochs{StartEpoch: uint32(i * 10), FileName: file})
	}

	return notifier.ArgsNewGasScheduleNotifier{
		GasScheduleConfig:  config.GasScheduleConfig{GasScheduleByEpochs: byEpochs},
		EpochNotifier:      notifier.NewGenericEpochNotifier(),
		WasmVMChangeLocker: &sync.RWMutex{},
	}
}

func TestNewGasScheduleNotifier_UnloadableFutureScheduleRefusesToStart(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
	}{
		{name: "missing file"},
		{name: "scalar section", content: "BuiltInCost: 1\n"},
		{name: "empty section", content: "BuiltInCost:\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			first := writeGasSchedule(t, dir, "v1.yaml", nil)
			second := filepath.Join(dir, "v2.yaml")
			if tt.content != "" {
				require.NoError(t, os.WriteFile(second, []byte(tt.content), 0o600))
			}

			var gsn interface{}
			var err error
			require.NotPanics(t, func() {
				gsn, err = notifier.NewGasScheduleNotifier(newGasScheduleNotifierArgs(first, second))
			})
			require.Error(t, err)
			require.Contains(t, err.Error(), "epoch 10")
			require.Contains(t, err.Error(), second)
			require.Nil(t, gsn)
		})
	}
}

func TestNewGasScheduleNotifier_ShippedSchedulesAreAccepted(t *testing.T) {
	t.Parallel()

	for _, file := range []string{nodeGasSchedulePath, "../../integrationTest/config/gasScheduleV1.yaml"} {
		_, err := notifier.NewGasScheduleNotifier(newGasScheduleNotifierArgs(file))
		require.NoError(t, err, file)
	}
}

func TestNewGasScheduleNotifier_ScheduleAHandlerCannotApplyRefusesToStart(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		mutate  func(map[string]map[string]uint64)
		reason  string
		wrapped error
	}{
		{name: "missing vm cost", mutate: dropI32Add, reason: "wasm vm"},
		{
			name:   "missing built-in cost",
			mutate: func(gs map[string]map[string]uint64) { delete(gs["BuiltInCost"], "Transfer") },
			reason: "built-in functions",
		},
		{
			name:   "missing built-in cost section",
			mutate: func(gs map[string]map[string]uint64) { delete(gs, "BuiltInCost") },
			reason: "built-in functions",
		},
		{
			name:   "zero per-byte storage cost",
			mutate: func(gs map[string]map[string]uint64) { gs["BaseOperationCost"]["PersistPerByte"] = 0 },
			reason: "PersistPerByte",
		},
		{
			name:    "missing per-transaction maximum",
			mutate:  func(gs map[string]map[string]uint64) { delete(gs["MaxPerTransaction"], "MaxNumberOfTrieReadsPerTx") },
			reason:  "MaxNumberOfTrieReadsPerTx",
			wrapped: counters.ErrInvalidMaximumValue,
		},
		{
			name:    "zero per-transaction maximum",
			mutate:  func(gs map[string]map[string]uint64) { gs["MaxPerTransaction"]["MaxBuiltInCallsPerTx"] = 0 },
			reason:  "MaxBuiltInCallsPerTx",
			wrapped: counters.ErrInvalidMaximumValue,
		},
		{
			name:    "missing per-transaction section",
			mutate:  func(gs map[string]map[string]uint64) { delete(gs, "MaxPerTransaction") },
			reason:  "MaxPerTransaction",
			wrapped: counters.ErrInvalidMaximumValue,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			first := writeGasSchedule(t, dir, "v1.yaml", nil)
			second := writeGasSchedule(t, dir, "v2.yaml", tt.mutate)

			gsn, err := notifier.NewGasScheduleNotifier(newGasScheduleNotifierArgs(first, second))
			require.ErrorIs(t, err, common.ErrInvalidGasSchedule)
			if tt.wrapped != nil {
				require.ErrorIs(t, err, tt.wrapped)
			}
			require.Contains(t, err.Error(), "epoch 10")
			require.Contains(t, err.Error(), tt.reason)
			require.Nil(t, gsn)
		})
	}
}

func TestNewGasScheduleNotifier_IncompleteFirstScheduleRefusesToStart(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	first := writeGasSchedule(t, dir, "v1.yaml", dropI32Add)

	gsn, err := notifier.NewGasScheduleNotifier(newGasScheduleNotifierArgs(first))
	require.ErrorIs(t, err, common.ErrInvalidGasSchedule)
	require.Nil(t, gsn)
}

func TestNewGasScheduleNotifier_DuplicateStartEpochRefusesToStart(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	args := newGasScheduleNotifierArgs(writeGasSchedule(t, dir, "v1.yaml", nil), writeGasSchedule(t, dir, "v2.yaml", nil))
	args.GasScheduleConfig.GasScheduleByEpochs[1].StartEpoch = 0

	gsn, err := notifier.NewGasScheduleNotifier(args)
	require.ErrorIs(t, err, common.ErrDuplicateGasScheduleEpoch)
	require.Nil(t, gsn)
}

func TestNewGasScheduleNotifier_DoesNotReorderTheSharedConfig(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	args := newGasScheduleNotifierArgs(writeGasSchedule(t, dir, "v1.yaml", nil), writeGasSchedule(t, dir, "v2.yaml", nil))
	byEpochs := args.GasScheduleConfig.GasScheduleByEpochs
	byEpochs[0], byEpochs[1] = byEpochs[1], byEpochs[0]

	_, err := notifier.NewGasScheduleNotifier(args)
	require.NoError(t, err)
	require.Equal(t, uint32(10), byEpochs[0].StartEpoch)
}

func TestGasScheduleNotifier_EpochConfirmedAppliesNewSchedule(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	first := writeGasSchedule(t, dir, "v1.yaml", nil)
	second := writeGasSchedule(t, dir, "v2.yaml", bumpI32Add)

	gsn, err := notifier.NewGasScheduleNotifier(newGasScheduleNotifierArgs(first, second))
	require.NoError(t, err)

	handler := &gasScheduleHandlerStub{}
	gsn.RegisterNotifyHandler(handler)
	oldCost := gsn.LatestGasSchedule()["WASMOpcodeCost"]["I32Add"]

	gsn.EpochConfirmed(10)

	require.Len(t, handler.received, 2)
	require.Equal(t, oldCost+1, handler.received[1]["WASMOpcodeCost"]["I32Add"])
	require.Equal(t, oldCost+1, gsn.LatestGasSchedule()["WASMOpcodeCost"]["I32Add"])
}

func TestGasScheduleNotifier_EpochConfirmedAppliesTheScheduleCheckedAtStartUp(t *testing.T) {
	t.Parallel()

	t.Run("file changed on disk after start-up", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		first := writeGasSchedule(t, dir, "v1.yaml", nil)
		second := writeGasSchedule(t, dir, "v2.yaml", bumpI32Add)

		gsn, err := notifier.NewGasScheduleNotifier(newGasScheduleNotifierArgs(first, second))
		require.NoError(t, err)
		handler := &gasScheduleHandlerStub{}
		gsn.RegisterNotifyHandler(handler)
		checked := gsn.LatestGasSchedule()["WASMOpcodeCost"]["I32Add"] + 1

		writeGasSchedule(t, dir, "v2.yaml", dropI32Add)
		gsn.EpochConfirmed(10)

		require.Len(t, handler.received, 2)
		require.Equal(t, checked, handler.received[1]["WASMOpcodeCost"]["I32Add"])
	})

	t.Run("file removed after start-up", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		first := writeGasSchedule(t, dir, "v1.yaml", nil)
		second := writeGasSchedule(t, dir, "v2.yaml", bumpI32Add)

		gsn, err := notifier.NewGasScheduleNotifier(newGasScheduleNotifierArgs(first, second))
		require.NoError(t, err)
		handler := &gasScheduleHandlerStub{}
		gsn.RegisterNotifyHandler(handler)

		require.NoError(t, os.Remove(second))
		gsn.EpochConfirmed(10)

		require.Len(t, handler.received, 2)
		require.Equal(t, handler.received[1], gsn.LatestGasSchedule())
	})
}
