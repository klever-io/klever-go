package contexts

import (
	"strings"
	"testing"

	commonMock "github.com/klever-io/klever-go/common/mock"
	mock "github.com/klever-io/klever-go/kvm/mock/context"
	"github.com/stretchr/testify/require"
)

func TestInstanceTracker_InitState(t *testing.T) {
	iTracker, err := NewInstanceTracker(commonMock.NewForkControllerStub())
	require.Nil(t, err)
	require.Equal(t, 0, iTracker.numRunningInstances)

	for i := 0; i < 5; i++ {
		_ = iTracker.SetNewInstance(mock.NewInstanceMock(nil), Bytecode)
	}

	require.Equal(t, 5, iTracker.numRunningInstances)
	require.Len(t, iTracker.instances, 5)

	iTracker.codeSize = 12
	iTracker.InitState()

	require.Nil(t, iTracker.instance)
	require.Len(t, iTracker.codeHash, 0)
	require.Len(t, iTracker.instances, 0)
	require.Zero(t, iTracker.codeSize)

	// InitState() must not reset numRunningInstances
	require.Equal(t, 5, iTracker.numRunningInstances)
}

func TestInstanceTracker_GetWarmInstance(t *testing.T) {
	iTracker, err := NewInstanceTracker(commonMock.NewForkControllerStub())
	require.Nil(t, err)

	testData := []string{"warm1", "bytecode1", "bytecode2", "warm2"}

	for _, codeHash := range testData {
		_ = iTracker.SetNewInstance(mock.NewInstanceMock([]byte(codeHash)), Bytecode)
		iTracker.codeHash = []byte(codeHash)
		if strings.Contains(codeHash, "warm") {
			iTracker.SaveAsWarmInstance()
		}
	}

	require.Equal(t, 4, iTracker.numRunningInstances)
	require.Len(t, iTracker.instances, 4)

	for _, codeHash := range testData {
		instance, ok := iTracker.GetWarmInstance([]byte(codeHash))

		if strings.Contains(codeHash, "warm") {
			require.NotNil(t, instance)
			require.True(t, ok)
			continue
		}

		require.Nil(t, instance)
		require.False(t, ok)
	}

}

func TestInstanceTracker_UseWarmInstance(t *testing.T) {
	iTracker, err := NewInstanceTracker(commonMock.NewForkControllerStub())
	require.Nil(t, err)

	testData := []string{"warm1", "bytecode1", "warm2", "bytecode2"}

	for _, codeHash := range testData {
		_ = iTracker.SetNewInstance(mock.NewInstanceMock([]byte(codeHash)), Bytecode)
		iTracker.codeHash = []byte(codeHash)

		if strings.Contains(codeHash, "warm") {
			iTracker.SaveAsWarmInstance()
		}
	}

	require.Equal(t, []byte("bytecode2"), iTracker.CodeHash())

	for _, codeHash := range testData {
		ok, _ := iTracker.UseWarmInstance([]byte(codeHash), false)

		if strings.Contains(codeHash, "warm") {
			require.True(t, ok)
			continue
		}

		require.False(t, ok)
	}
}

func TestInstanceTracker_IsCodeHashOnStack_Ok(t *testing.T) {
	iTracker, err := NewInstanceTracker(commonMock.NewForkControllerStub())
	require.Nil(t, err)

	testData := []string{"alpha", "beta", "alpha", "active"}

	for i, codeHash := range testData {
		_ = iTracker.SetNewInstance(mock.NewInstanceMock([]byte(codeHash)), Bytecode)
		iTracker.codeHash = []byte(codeHash)
		if i < 2 || codeHash == "active" {
			iTracker.SaveAsWarmInstance()
		}
		if codeHash != "active" {
			iTracker.PushState()
		}
	}
	require.Len(t, iTracker.codeHashStack, 3)
	require.Len(t, iTracker.instanceStack, 3)

	warm, cold := iTracker.NumRunningInstances()
	require.Equal(t, 3, warm)
	require.Equal(t, 1, cold)

	iTracker.PopSetActiveState()
	require.Equal(t, []byte("alpha"), iTracker.CodeHash())
	require.True(t, iTracker.IsCodeHashOnTheStack(iTracker.codeHash))

	iTracker.PopSetActiveState()
	require.Equal(t, []byte("beta"), iTracker.CodeHash())
	require.False(t, iTracker.IsCodeHashOnTheStack(iTracker.codeHash))
}

// stack: alpha<-alpha(cold)<-alpha(cold)<-alpha(cold)
func TestInstanceTracker_PopSetActiveSelfScenario(t *testing.T) {
	iTracker, err := NewInstanceTracker(commonMock.NewForkControllerStub())
	require.Nil(t, err)

	testData := []string{"alpha", "alpha", "alpha", "alpha", "active"}

	for i, codeHash := range testData {
		_ = iTracker.SetNewInstance(mock.NewInstanceMock([]byte(codeHash)), Bytecode)
		iTracker.codeHash = []byte(codeHash)
		if i == 0 || codeHash == "active" {
			iTracker.SaveAsWarmInstance()
		}
		if codeHash != "active" {
			iTracker.PushState()
		}
	}
	require.Len(t, iTracker.codeHashStack, 4)
	require.Len(t, iTracker.instanceStack, 4)

	warm, cold := iTracker.NumRunningInstances()
	require.Equal(t, 2, warm)
	require.Equal(t, 3, cold)

	checkColdInstancesAfterEmptyingStack(t, iTracker)

	iTracker.ClearWarmInstanceCache()
	checkInstances(t, iTracker)
}

// stack: alpha<-beta<-alpha(cold)<-beta(cold)
func TestInstanceTracker_PopSetActiveSimpleScenario(t *testing.T) {
	iTracker, err := NewInstanceTracker(commonMock.NewForkControllerStub())
	require.Nil(t, err)

	testData := []string{"alpha", "beta", "alpha", "beta", "active"}

	for i, codeHash := range testData {
		_ = iTracker.SetNewInstance(mock.NewInstanceMock([]byte(codeHash)), Bytecode)
		iTracker.codeHash = []byte(codeHash)
		if i < 2 || codeHash == "active" {
			iTracker.SaveAsWarmInstance()
		}
		if codeHash != "active" {
			iTracker.PushState()
		}
	}
	require.Len(t, iTracker.codeHashStack, 4)
	require.Len(t, iTracker.instanceStack, 4)

	warm, cold := iTracker.NumRunningInstances()
	require.Equal(t, 3, warm)
	require.Equal(t, 2, cold)

	emptyInstanceStack(iTracker)

	warm, cold = iTracker.NumRunningInstances()
	require.Equal(t, 3, warm)
	require.Equal(t, 0, cold)

	require.Equal(t, 3, iTracker.numRunningInstances)
	iTracker.InitState()
	require.Equal(t, 3, iTracker.numRunningInstances)

	iTracker.ClearWarmInstanceCache()
	require.Equal(t, 0, iTracker.numRunningInstances)
	checkInstances(t, iTracker)
}

// stack: alpha<-beta<-gamma<-beta(cold)<-gamma(cold)<-delta<-alpha(cold)
func TestInstanceTracker_PopSetActiveComplexScenario(t *testing.T) {
	iTracker, err := NewInstanceTracker(commonMock.NewForkControllerStub())
	require.Nil(t, err)

	testData := []string{"alpha", "beta", "gamma", "beta", "gamma", "delta", "alpha", "active"}

	for i, codeHash := range testData {
		_ = iTracker.SetNewInstance(mock.NewInstanceMock([]byte(codeHash)), Bytecode)
		iTracker.codeHash = []byte(codeHash)
		if i < 3 || codeHash == "delta" || codeHash == "active" {
			iTracker.SaveAsWarmInstance()
		}
		if codeHash != "active" {
			iTracker.PushState()
		}
	}
	require.Len(t, iTracker.codeHashStack, 7)
	require.Len(t, iTracker.instanceStack, 7)

	warm, cold := iTracker.NumRunningInstances()
	require.Equal(t, 5, warm)
	require.Equal(t, 3, cold)

	checkColdInstancesAfterEmptyingStack(t, iTracker)

	iTracker.ClearWarmInstanceCache()
	checkInstances(t, iTracker)
}

func TestInstanceTracker_PopSetActiveWarmOnlyScenario(t *testing.T) {
	iTracker, err := NewInstanceTracker(commonMock.NewForkControllerStub())
	require.Nil(t, err)

	testData := []string{"alpha", "beta", "gamma", "delta", "active"}

	for _, codeHash := range testData {
		_ = iTracker.SetNewInstance(mock.NewInstanceMock([]byte(codeHash)), Bytecode)
		iTracker.codeHash = []byte(codeHash)
		iTracker.SaveAsWarmInstance()

		if codeHash != "active" {
			iTracker.PushState()
		}
	}
	require.Len(t, iTracker.codeHashStack, 4)
	require.Len(t, iTracker.instanceStack, 4)

	warm, cold := iTracker.NumRunningInstances()
	require.Equal(t, 5, warm)
	require.Equal(t, 0, cold)

	checkColdInstancesAfterEmptyingStack(t, iTracker)

	iTracker.ClearWarmInstanceCache()
	checkInstances(t, iTracker)
}

func TestInstanceTracker_ForceCleanInstanceWithBypass(t *testing.T) {
	iTracker, err := NewInstanceTracker(commonMock.NewForkControllerStub())
	require.Nil(t, err)

	testData := []string{"warm1", "bytecode1"}

	for _, codeHash := range testData {
		_ = iTracker.SetNewInstance(mock.NewInstanceMock([]byte(codeHash)), Bytecode)
		iTracker.codeHash = []byte(codeHash)

		if strings.Contains(codeHash, "warm") {
			iTracker.SaveAsWarmInstance()
		}
	}

	warm, cold := iTracker.NumRunningInstances()
	require.Equal(t, 1, warm)
	require.Equal(t, 1, cold)

	iTracker.ForceCleanInstance(true)
	require.Nil(t, iTracker.instance)

	_, _ = iTracker.UseWarmInstance([]byte("warm1"), false)
	require.NotNil(t, iTracker.instance)

	iTracker.ForceCleanInstance(true)
	require.Nil(t, iTracker.instance)

	require.Equal(t, 0, iTracker.numRunningInstances)
	require.Nil(t, iTracker.CheckInstances())
}

func TestInstanceTracker_DoubleForceClean(t *testing.T) {
	iTracker, err := NewInstanceTracker(commonMock.NewForkControllerStub())
	require.Nil(t, err)

	_ = iTracker.SetNewInstance(mock.NewInstanceMock(nil), Bytecode)
	require.NotNil(t, iTracker.instance)
	require.Equal(t, 1, iTracker.numRunningInstances)

	iTracker.ForceCleanInstance(true)
	require.Equal(t, 0, iTracker.numRunningInstances)
	require.Nil(t, iTracker.CheckInstances())

	iTracker.ForceCleanInstance(true)
	require.Equal(t, 0, iTracker.numRunningInstances)
	require.Nil(t, iTracker.CheckInstances())
}

func TestInstanceTracker_UnsetInstance_AlreadyNil_Ok(t *testing.T) {
	iTracker, err := NewInstanceTracker(commonMock.NewForkControllerStub())
	require.Nil(t, err)

	iTracker.instance = &mock.InstanceMock{}
	iTracker.UnsetInstance()
	require.Nil(t, iTracker.instance)
}

func checkColdInstancesAfterEmptyingStack(t *testing.T, iTracker *instanceTracker) {
	emptyInstanceStack(iTracker)
	_, cold := iTracker.NumRunningInstances()
	require.Equal(t, 0, cold)
}

func emptyInstanceStack(iTracker *instanceTracker) {
	n := len(iTracker.instanceStack)
	for i := 0; i < n; i++ {
		iTracker.PopSetActiveState()
	}
}

// fillTracker tracks as many instances as a transaction is allowed to hold, so that
// the next new instance is rejected.
func fillTracker(t *testing.T, iTracker *instanceTracker) {
	for i := 0; i < maxTrackedInstances; i++ {
		require.NoError(t, iTracker.SetNewInstance(mock.NewInstanceMock(nil), Bytecode))
	}
}

func checkInstances(t *testing.T, iTracker *instanceTracker) {
	require.Equal(t, 0, iTracker.numRunningInstances)
	require.Len(t, iTracker.instanceStack, 0)
	require.Len(t, iTracker.codeHashStack, 0)
	require.Nil(t, iTracker.CheckInstances())
}

func TestInstanceTracker_SetNewInstanceCapacity(t *testing.T) {
	iTracker, err := NewInstanceTracker(commonMock.NewForkControllerStub())
	require.Nil(t, err)

	// The number of accepted instances must not drift: the Nth distinct instance
	// is rejected once N reaches warmCacheSize-1, so warmCacheSize-2 are accepted.
	accepted := warmCacheSize - 2
	require.Equal(t, accepted, maxTrackedInstances)
	fillTracker(t, iTracker)
	require.Len(t, iTracker.instances, accepted)

	// Re-tracking an already tracked instance does not grow the map, so it must
	// still be accepted at capacity.
	require.NoError(t, iTracker.SetNewInstance(iTracker.instance, Bytecode))
	require.Len(t, iTracker.instances, accepted)

	// A warm instance that is not tracked yet does grow the map (the map is reset
	// per transaction while the warm cache outlives it), so it must be rejected
	// like any other: the limit cannot be branched on the cache level.
	require.ErrorIs(t, iTracker.SetNewInstance(mock.NewInstanceMock(nil), Warm), errTooManyInstances)
	require.Len(t, iTracker.instances, accepted)

	// A rejected instance must leave no trace: the caller still owns it and is
	// responsible for cleaning it.
	numRunningBefore := iTracker.numRunningInstances
	activeBefore := iTracker.instance
	rejected := mock.NewInstanceMock(nil)

	require.ErrorIs(t, iTracker.SetNewInstance(rejected, Bytecode), errTooManyInstances)

	require.Len(t, iTracker.instances, accepted)
	require.NotContains(t, iTracker.instances, rejected.ID())
	require.Equal(t, numRunningBefore, iTracker.numRunningInstances)
	require.Same(t, activeBefore, iTracker.instance)
}

// fixedIDInstance forges the ID of a real instance, reproducing what the native
// allocator does when it hands a freed instance's address to a new one.
type fixedIDInstance struct {
	*mock.InstanceMock
	id string
}

func (instance *fixedIDInstance) ID() string { return instance.id }

func TestInstanceTracker_SetNewInstanceRejectsRecycledID(t *testing.T) {
	iTracker, err := NewInstanceTracker(commonMock.NewForkControllerStub())
	require.Nil(t, err)

	accepted := warmCacheSize - 2
	fillTracker(t, iTracker)

	// A new instance reusing a tracked instance's ID is not the tracked instance:
	// it must face the capacity check like any other, instead of silently
	// overwriting the entry and slipping past it.
	recycled := &fixedIDInstance{InstanceMock: mock.NewInstanceMock(nil), id: iTracker.instance.ID()}
	numRunningBefore := iTracker.numRunningInstances
	activeBefore := iTracker.instance

	require.ErrorIs(t, iTracker.SetNewInstance(recycled, Bytecode), errTooManyInstances)

	require.Len(t, iTracker.instances, accepted)
	require.Equal(t, accepted, iTracker.numTrackedInstances)
	require.Same(t, activeBefore, iTracker.instance)
	require.Equal(t, numRunningBefore, iTracker.numRunningInstances)
	require.NotSame(t, recycled, iTracker.instances[recycled.ID()])
}

func TestInstanceTracker_RecycledIDDoesNotUndercount(t *testing.T) {
	iTracker, err := NewInstanceTracker(commonMock.NewForkControllerStub())
	require.Nil(t, err)

	// Below capacity, an instance reusing a freed ID is accepted but must still be
	// counted: the count is what bounds warm cache churn, and overwriting a stale
	// map entry leaves len(instances) flat.
	first := mock.NewInstanceMock(nil)
	require.NoError(t, iTracker.SetNewInstance(first, Bytecode))
	require.NoError(t, iTracker.SetNewInstance(&fixedIDInstance{InstanceMock: mock.NewInstanceMock(nil), id: first.ID()}, Bytecode))

	require.Len(t, iTracker.instances, 1)
	require.Equal(t, 2, iTracker.numTrackedInstances)

	iTracker.InitState()
	require.Equal(t, 0, iTracker.numTrackedInstances)
}

func TestInstanceTracker_CleanedInstancesFreeCapacity(t *testing.T) {
	iTracker, err := NewInstanceTracker(commonMock.NewForkControllerStub())
	require.Nil(t, err)

	// A contract calling itself in a loop: its codeHash is on the stack, so every
	// iteration takes the cold path and the instance is cleaned when popped. The
	// transaction holds two instances at a time, however many it churns through.
	require.NoError(t, iTracker.SetNewInstance(mock.NewInstanceMock([]byte("alpha")), Bytecode))
	iTracker.codeHash = []byte("alpha")
	iTracker.PushState()

	for i := 0; i < 3*warmCacheSize; i++ {
		require.NoError(t, iTracker.SetNewInstance(mock.NewInstanceMock([]byte("alpha")), Bytecode))
		iTracker.codeHash = []byte("alpha")
		require.Equal(t, 2, iTracker.numTrackedInstances)

		iTracker.PopSetActiveState()
		iTracker.PushState()
		require.Equal(t, 1, iTracker.numTrackedInstances)
	}
}

func TestInstanceTracker_CleaningUntrackedInstanceKeepsCountAtZero(t *testing.T) {
	iTracker, err := NewInstanceTracker(commonMock.NewForkControllerStub())
	require.Nil(t, err)

	instance := mock.NewInstanceMock([]byte("alpha"))
	require.NoError(t, iTracker.SetNewInstance(instance, Bytecode))
	iTracker.codeHash = []byte("alpha")
	iTracker.SaveAsWarmInstance()

	// The warm cache outlives the transaction, the tracked instances map does not,
	// so evicting this instance in the next transaction must not spend budget that
	// was never handed out.
	iTracker.InitState()
	iTracker.ClearWarmInstanceCache()

	require.True(t, instance.IsAlreadyCleaned())
	require.Equal(t, 0, iTracker.numTrackedInstances)
}

func TestInstanceTracker_SetNewInstanceCapacityBeforeFixAuditChangesV4(t *testing.T) {
	forkController := commonMock.NewForkControllerStub()
	forkController.FixAuditChangesV4Value = false

	iTracker, err := NewInstanceTracker(forkController)
	require.Nil(t, err)

	fillTracker(t, iTracker)

	// Before the fork, the bound is the size of a map keyed on the native pointer:
	// an instance inheriting a freed ID overwrites the stale entry without growing
	// it, and so slips past the limit.
	recycled := &fixedIDInstance{InstanceMock: mock.NewInstanceMock(nil), id: iTracker.instance.ID()}
	require.NoError(t, iTracker.SetNewInstance(recycled, Bytecode))
	require.Len(t, iTracker.instances, maxTrackedInstances)

	// And cleaning instances does not hand any budget back either.
	iTracker.codeHash = []byte("alpha")
	iTracker.ForceCleanInstance(true)
	require.ErrorIs(t, iTracker.SetNewInstance(mock.NewInstanceMock(nil), Bytecode), errTooManyInstances)
}
