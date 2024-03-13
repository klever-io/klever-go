package testcommon

import (
	"fmt"
	"runtime"
	"strings"

	logger "github.com/klever-io/klever-go-logger"
)

const generateGraphs = false
const graphsFolder = "/home/bogdan/graphs/"

// LogGraph -
var LogGraph = logger.GetOrCreate("vm/graph")
var logAsync = logger.GetOrCreate("vm/async")

// TestReturnDataSuffix -
var TestReturnDataSuffix = "_returnData"

// TestCallbackPrefix -
var TestCallbackPrefix = "callback_"

// TestContextCallbackFunction -
var TestContextCallbackFunction = "contextCallback"

// ErrSyncCallFail -
var ErrSyncCallFail = fmt.Errorf("sync call fail")

// ErrAsyncCallFail -
var ErrAsyncCallFail = fmt.Errorf("async call fail")

// ErrAsyncCallbackFail -
var ErrAsyncCallbackFail = fmt.Errorf("callback fail")

// ErrAsyncRegisterFail -
var ErrAsyncRegisterFail = fmt.Errorf("unable to register async call")

type argIndexesForGraphCall struct {
	edgeTypeIdx          int
	failureIdx           int
	gasUsedIdx           int
	gasUsedByCallbackIdx int
	callbackFailureIdx   int
}

var syncCallArgIndexes = argIndexesForGraphCall{
	edgeTypeIdx: 0,
	failureIdx:  1,
	gasUsedIdx:  2,
}

var asyncCallArgIndexes = argIndexesForGraphCall{
	edgeTypeIdx:          0,
	failureIdx:           1,
	gasUsedIdx:           2,
	gasUsedByCallbackIdx: 3,
	callbackFailureIdx:   4,
}

var callbackCallArgIndexes = argIndexesForGraphCall{
	edgeTypeIdx: 1,
	failureIdx:  2,
	gasUsedIdx:  3,
}

// RuntimeConfigOfCall -
type RuntimeConfigOfCall struct {
	gasUsed           uint64
	gasUsedByCallback uint64
	edgeType          TestCallEdgeType
	willFail          bool
	willFailExpected  bool
	willCallbackFail  bool
}

// CallsFinishData -
type CallsFinishData struct {
	Data []*CallFinishDataItem
}

// CallFinishDataItem -
type CallFinishDataItem struct {
	OriginalCallerAddr  []byte
	ContractAndFunction string
	GasProvided         uint64
	GasRemaining        uint64
	FailError           error
}

// MakeGraphAndImage -
func MakeGraphAndImage(graph *TestCallGraph) *TestCallGraph {
	if generateGraphs {
		GenerateSVGforGraph(graph, graphsFolder, getTestFunctionName())
	}
	return graph
}

func getTestFunctionName() string {
	pc := make([]uintptr, 10)
	runtime.Callers(3, pc)
	f := runtime.FuncForPC(pc[0])
	fullFunctionName := f.Name()
	lastIndexOfDot := strings.LastIndex(fullFunctionName, ".")
	return fullFunctionName[lastIndexOfDot+1:]
}

// CreateGraphTestSyncAndAsync8 -
func CreateGraphTestSyncAndAsync8() *TestCallGraph {
	callGraph := CreateTestCallGraph()
	sc1f1 := callGraph.AddStartNode("sc1", "f1", 5000, 10)

	sc2f3 := callGraph.AddNode("sc2", "f3")
	callGraph.AddAsyncEdge(sc1f1, sc2f3, "cb2", "").
		SetGasLimit(500).
		SetGasUsed(7).
		SetGasUsedByCallback(10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddSyncEdge(sc1f1, sc2f2).
		SetGasLimit(500).
		SetGasUsed(7)

	sc3f4 := callGraph.AddNode("sc3", "f4")

	callGraph.AddAsyncEdge(sc2f2, sc3f4, "cb3", "").
		SetGasLimit(100).
		SetGasUsed(2).
		SetGasUsedByCallback(3)

	sc1cb2 := callGraph.AddNode("sc1", "cb2")
	sc4f5 := callGraph.AddNode("sc4", "f5")
	callGraph.AddSyncEdge(sc1cb2, sc4f5).
		SetGasLimit(4).
		SetGasUsed(2)

	callGraph.AddNode("sc2", "cb3")

	return callGraph
}

// CreateGraphTestAsyncCallsAsync -
func CreateGraphTestAsyncCallsAsync() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 1000, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncEdge(sc1f1, sc2f2, "cb1", "").
		SetGasLimit(500).
		SetGasUsed(7).
		SetGasUsedByCallback(5)

	callGraph.AddNode("sc1", "cb1")

	sc3f3 := callGraph.AddNode("sc3", "f3")
	callGraph.AddAsyncEdge(sc2f2, sc3f3, "cb2", "").
		SetGasLimit(100).
		SetGasUsed(6).
		SetGasUsedByCallback(3)

	callGraph.AddNode("sc2", "cb2")

	return callGraph
}

// CreateGraphTestAsyncCallsAsyncFirstNoCallback -
func CreateGraphTestAsyncCallsAsyncFirstNoCallback() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 1000, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncEdge(sc1f1, sc2f2, "", "").
		SetGasLimit(500).
		SetGasUsed(7)

	callGraph.AddNode("sc1", "cb1")

	sc3f3 := callGraph.AddNode("sc3", "f3")
	callGraph.AddAsyncEdge(sc2f2, sc3f3, "cb2", "").
		SetGasLimit(100).
		SetGasUsed(6).
		SetGasUsedByCallback(3)

	callGraph.AddNode("sc2", "cb2")

	return callGraph
}

// CreateGraphTestAsyncCallsAsyncSecondNoCallback -
func CreateGraphTestAsyncCallsAsyncSecondNoCallback() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 1000, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncEdge(sc1f1, sc2f2, "cb1", "").
		SetGasLimit(500).
		SetGasUsed(7).
		SetGasUsedByCallback(5)

	callGraph.AddNode("sc1", "cb1")

	sc3f3 := callGraph.AddNode("sc3", "f3")
	callGraph.AddAsyncEdge(sc2f2, sc3f3, "", "").
		SetGasLimit(100).
		SetGasUsed(6)

	callGraph.AddNode("sc2", "cb2")

	return callGraph
}

// CreateGraphTestAsyncCallsAsyncFirstFail -
func CreateGraphTestAsyncCallsAsyncFirstFail() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 1000, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncEdge(sc1f1, sc2f2, "cb1", "").
		SetGasLimit(500).
		SetGasUsed(7).
		SetGasUsedByCallback(5).
		SetFail()

	callGraph.AddNode("sc1", "cb1")

	sc3f3 := callGraph.AddNode("sc3", "f3")
	callGraph.AddAsyncEdge(sc2f2, sc3f3, "cb2", "").
		SetGasLimit(100).
		SetGasUsed(6).
		SetGasUsedByCallback(3)

	callGraph.AddNode("sc2", "cb2")

	return callGraph
}

// CreateGraphTestAsyncCallsAsyncSecondFail -
func CreateGraphTestAsyncCallsAsyncSecondFail() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 1000, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncEdge(sc1f1, sc2f2, "cb1", "").
		SetGasLimit(500).
		SetGasUsed(7).
		SetGasUsedByCallback(5)

	callGraph.AddNode("sc1", "cb1")

	sc3f3 := callGraph.AddNode("sc3", "f3")
	callGraph.AddAsyncEdge(sc2f2, sc3f3, "cb2", "").
		SetGasLimit(100).
		SetGasUsed(6).
		SetGasUsedByCallback(3).
		SetFail()

	callGraph.AddNode("sc2", "cb2")

	return callGraph
}

// CreateGraphTestAsyncCallsAsyncFirstCallbackFail -
func CreateGraphTestAsyncCallsAsyncFirstCallbackFail() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 1000, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncEdge(sc1f1, sc2f2, "cb1", "").
		SetGasLimit(500).
		SetGasUsed(7).
		SetGasUsedByCallback(5).
		SetCallbackFail()

	callGraph.AddNode("sc1", "cb1")

	sc3f3 := callGraph.AddNode("sc3", "f3")
	callGraph.AddAsyncEdge(sc2f2, sc3f3, "cb2", "").
		SetGasLimit(100).
		SetGasUsed(6).
		SetGasUsedByCallback(3)

	callGraph.AddNode("sc2", "cb2")

	return callGraph
}

// CreateGraphTestAsyncCallsAsyncSecondCallbackFail -
func CreateGraphTestAsyncCallsAsyncSecondCallbackFail() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 1000, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncEdge(sc1f1, sc2f2, "cb1", "").
		SetGasLimit(500).
		SetGasUsed(7).
		SetGasUsedByCallback(5)

	callGraph.AddNode("sc1", "cb1")

	sc3f3 := callGraph.AddNode("sc3", "f3")
	callGraph.AddAsyncEdge(sc2f2, sc3f3, "cb2", "").
		SetGasLimit(100).
		SetGasUsed(6).
		SetGasUsedByCallback(3).
		SetCallbackFail()

	callGraph.AddNode("sc2", "cb2")

	return callGraph
}

// CreateGraphTestAsyncCallsAsyncBothCallbacksFail -
func CreateGraphTestAsyncCallsAsyncBothCallbacksFail() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 1000, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncEdge(sc1f1, sc2f2, "cb1", "").
		SetGasLimit(500).
		SetGasUsed(7).
		SetGasUsedByCallback(5).
		SetCallbackFail()

	callGraph.AddNode("sc1", "cb1")

	sc3f3 := callGraph.AddNode("sc3", "f3")
	callGraph.AddAsyncEdge(sc2f2, sc3f3, "cb2", "").
		SetGasLimit(100).
		SetGasUsed(6).
		SetGasUsedByCallback(3).
		SetCallbackFail()

	callGraph.AddNode("sc2", "cb2")

	return callGraph
}

// CreateGraphTestAsyncCallsAsyncCrossLocal -
func CreateGraphTestAsyncCallsAsyncCrossLocal() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 1000, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncCrossShardEdge(sc1f1, sc2f2, "cb1", "").
		SetGasLimit(500).
		SetGasUsed(7).
		SetGasUsedByCallback(5)

	callGraph.AddNode("sc1", "cb1")

	sc3f3 := callGraph.AddNode("sc3", "f3")
	callGraph.AddAsyncEdge(sc2f2, sc3f3, "cb2", "").
		SetGasLimit(100).
		SetGasUsed(6).
		SetGasUsedByCallback(3)

	callGraph.AddNode("sc2", "cb2")

	return callGraph
}

// CreateGraphTestAsyncCallsAsyncFirstNoCallbackCrossLocal -
func CreateGraphTestAsyncCallsAsyncFirstNoCallbackCrossLocal() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 1000, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncCrossShardEdge(sc1f1, sc2f2, "", "").
		SetGasLimit(500).
		SetGasUsed(7)

	callGraph.AddNode("sc1", "cb1")

	sc3f3 := callGraph.AddNode("sc3", "f3")
	callGraph.AddAsyncEdge(sc2f2, sc3f3, "cb2", "").
		SetGasLimit(100).
		SetGasUsed(6).
		SetGasUsedByCallback(3)

	callGraph.AddNode("sc2", "cb2")

	return callGraph
}

// CreateGraphTestAsyncCallsAsyncSecondNoCallbackCrossLocal -
func CreateGraphTestAsyncCallsAsyncSecondNoCallbackCrossLocal() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 1000, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncCrossShardEdge(sc1f1, sc2f2, "cb1", "").
		SetGasLimit(500).
		SetGasUsed(7).
		SetGasUsedByCallback(5)

	callGraph.AddNode("sc1", "cb1")

	sc3f3 := callGraph.AddNode("sc3", "f3")
	callGraph.AddAsyncEdge(sc2f2, sc3f3, "", "").
		SetGasLimit(100).
		SetGasUsed(6)

	callGraph.AddNode("sc2", "cb2")

	return callGraph
}

// CreateGraphTestAsyncCallsAsyncFirstFailCrossLocal -
func CreateGraphTestAsyncCallsAsyncFirstFailCrossLocal() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 1000, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncCrossShardEdge(sc1f1, sc2f2, "cb1", "").
		SetGasLimit(500).
		SetGasUsed(7).
		SetGasUsedByCallback(5).
		SetFail()

	callGraph.AddNode("sc1", "cb1")

	sc3f3 := callGraph.AddNode("sc3", "f3")
	callGraph.AddAsyncEdge(sc2f2, sc3f3, "cb2", "").
		SetGasLimit(100).
		SetGasUsed(6).
		SetGasUsedByCallback(3)

	callGraph.AddNode("sc2", "cb2")

	return callGraph
}

// CreateGraphTestAsyncCallsAsyncFirstCallbackFailCrossLocal -
func CreateGraphTestAsyncCallsAsyncFirstCallbackFailCrossLocal() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 1000, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncCrossShardEdge(sc1f1, sc2f2, "cb1", "").
		SetGasLimit(500).
		SetGasUsed(7).
		SetGasUsedByCallback(5).
		SetCallbackFail()

	callGraph.AddNode("sc1", "cb1")

	sc3f3 := callGraph.AddNode("sc3", "f3")
	callGraph.AddAsyncEdge(sc2f2, sc3f3, "cb2", "").
		SetGasLimit(100).
		SetGasUsed(6).
		SetGasUsedByCallback(3)

	callGraph.AddNode("sc2", "cb2")

	return callGraph
}

// CreateGraphTestAsyncCallsAsyncSecondFailCrossLocal -
func CreateGraphTestAsyncCallsAsyncSecondFailCrossLocal() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 1000, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncCrossShardEdge(sc1f1, sc2f2, "cb1", "").
		SetGasLimit(500).
		SetGasUsed(7).
		SetGasUsedByCallback(5)

	callGraph.AddNode("sc1", "cb1")

	sc3f3 := callGraph.AddNode("sc3", "f3")
	callGraph.AddAsyncEdge(sc2f2, sc3f3, "cb2", "").
		SetGasLimit(100).
		SetGasUsed(6).
		SetGasUsedByCallback(3).
		SetFail()

	callGraph.AddNode("sc2", "cb2")

	return callGraph
}

// CreateGraphTestAsyncCallsAsyncSecondCallbackFailCrossLocal -
func CreateGraphTestAsyncCallsAsyncSecondCallbackFailCrossLocal() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 1000, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncCrossShardEdge(sc1f1, sc2f2, "cb1", "").
		SetGasLimit(500).
		SetGasUsed(7).
		SetGasUsedByCallback(5)

	callGraph.AddNode("sc1", "cb1")

	sc3f3 := callGraph.AddNode("sc3", "f3")
	callGraph.AddAsyncEdge(sc2f2, sc3f3, "cb2", "").
		SetGasLimit(100).
		SetGasUsed(6).
		SetGasUsedByCallback(3).
		SetCallbackFail()

	callGraph.AddNode("sc2", "cb2")

	return callGraph
}

// CreateGraphTestAsyncCallsAsyncBothCallbacksFailCrossLocal -
func CreateGraphTestAsyncCallsAsyncBothCallbacksFailCrossLocal() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 1000, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncCrossShardEdge(sc1f1, sc2f2, "cb1", "").
		SetGasLimit(500).
		SetGasUsed(7).
		SetGasUsedByCallback(5).
		SetCallbackFail()

	callGraph.AddNode("sc1", "cb1")

	sc3f3 := callGraph.AddNode("sc3", "f3")
	callGraph.AddAsyncEdge(sc2f2, sc3f3, "cb2", "").
		SetGasLimit(100).
		SetGasUsed(6).
		SetGasUsedByCallback(3).
		SetCallbackFail()

	callGraph.AddNode("sc2", "cb2")

	return callGraph
}

// CreateGraphTestAsyncCallsAsyncLocalCross -
func CreateGraphTestAsyncCallsAsyncLocalCross() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 1000, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncEdge(sc1f1, sc2f2, "cb1", "").
		SetGasLimit(500).
		SetGasUsed(7).
		SetGasUsedByCallback(5)

	callGraph.AddNode("sc1", "cb1")

	sc3f3 := callGraph.AddNode("sc3", "f3")
	callGraph.AddAsyncCrossShardEdge(sc2f2, sc3f3, "cb2", "").
		SetGasLimit(100).
		SetGasUsed(6).
		SetGasUsedByCallback(3)

	callGraph.AddNode("sc2", "cb2")

	return callGraph
}

// CreateGraphTestAsyncCallsAsyncFirstNoCallbackLocalCross -
func CreateGraphTestAsyncCallsAsyncFirstNoCallbackLocalCross() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 1000, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncEdge(sc1f1, sc2f2, "", "").
		SetGasLimit(500).
		SetGasUsed(7)

	callGraph.AddNode("sc1", "cb1")

	sc3f3 := callGraph.AddNode("sc3", "f3")
	callGraph.AddAsyncCrossShardEdge(sc2f2, sc3f3, "cb2", "").
		SetGasLimit(100).
		SetGasUsed(6).
		SetGasUsedByCallback(3)

	callGraph.AddNode("sc2", "cb2")

	return callGraph
}

// CreateGraphTestAsyncCallsAsyncSecondNoCallbackLocalCross -
func CreateGraphTestAsyncCallsAsyncSecondNoCallbackLocalCross() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 1000, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncEdge(sc1f1, sc2f2, "cb1", "").
		SetGasLimit(500).
		SetGasUsed(7).
		SetGasUsedByCallback(5)

	callGraph.AddNode("sc1", "cb1")

	sc3f3 := callGraph.AddNode("sc3", "f3")
	callGraph.AddAsyncCrossShardEdge(sc2f2, sc3f3, "", "").
		SetGasLimit(100).
		SetGasUsed(6)

	callGraph.AddNode("sc2", "cb2")

	return callGraph
}

// CreateGraphTestAsyncCallsAsyncFirstFailLocalCross -
func CreateGraphTestAsyncCallsAsyncFirstFailLocalCross() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 1000, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncEdge(sc1f1, sc2f2, "cb1", "").
		SetGasLimit(500).
		SetGasUsed(7).
		SetGasUsedByCallback(5).
		SetFail()

	callGraph.AddNode("sc1", "cb1")

	sc3f3 := callGraph.AddNode("sc3", "f3")
	callGraph.AddAsyncCrossShardEdge(sc2f2, sc3f3, "cb2", "").
		SetGasLimit(100).
		SetGasUsed(6).
		SetGasUsedByCallback(3)

	callGraph.AddNode("sc2", "cb2")

	return callGraph
}

// CreateGraphTestAsyncCallsAsyncSecondFailLocalCross -
func CreateGraphTestAsyncCallsAsyncSecondFailLocalCross() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 1000, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncEdge(sc1f1, sc2f2, "cb1", "").
		SetGasLimit(500).
		SetGasUsed(7).
		SetGasUsedByCallback(5)

	callGraph.AddNode("sc1", "cb1")

	sc3f3 := callGraph.AddNode("sc3", "f3")
	callGraph.AddAsyncCrossShardEdge(sc2f2, sc3f3, "cb2", "").
		SetGasLimit(100).
		SetGasUsed(6).
		SetGasUsedByCallback(3).
		SetFail()

	callGraph.AddNode("sc2", "cb2")

	return callGraph
}

// CreateGraphTestAsyncCallsAsyncFirstCallbackFailLocalCross -
func CreateGraphTestAsyncCallsAsyncFirstCallbackFailLocalCross() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 1000, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncEdge(sc1f1, sc2f2, "cb1", "").
		SetGasLimit(500).
		SetGasUsed(7).
		SetGasUsedByCallback(5).
		SetCallbackFail()

	callGraph.AddNode("sc1", "cb1")

	sc3f3 := callGraph.AddNode("sc3", "f3")
	callGraph.AddAsyncCrossShardEdge(sc2f2, sc3f3, "cb2", "").
		SetGasLimit(100).
		SetGasUsed(6).
		SetGasUsedByCallback(3)

	callGraph.AddNode("sc2", "cb2")

	return callGraph
}

// CreateGraphTestAsyncCallsAsyncSecondCallbackFailLocalCross -
func CreateGraphTestAsyncCallsAsyncSecondCallbackFailLocalCross() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 1000, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncEdge(sc1f1, sc2f2, "cb1", "").
		SetGasLimit(500).
		SetGasUsed(7).
		SetGasUsedByCallback(5)

	callGraph.AddNode("sc1", "cb1")

	sc3f3 := callGraph.AddNode("sc3", "f3")
	callGraph.AddAsyncCrossShardEdge(sc2f2, sc3f3, "cb2", "").
		SetGasLimit(100).
		SetGasUsed(6).
		SetGasUsedByCallback(3).
		SetCallbackFail()

	callGraph.AddNode("sc2", "cb2")

	return callGraph
}

// CreateGraphTestAsyncCallsAsyncBothCallbacksFailLocalCross -
func CreateGraphTestAsyncCallsAsyncBothCallbacksFailLocalCross() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 1000, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncEdge(sc1f1, sc2f2, "cb1", "").
		SetGasLimit(500).
		SetGasUsed(7).
		SetGasUsedByCallback(5).
		SetCallbackFail()

	callGraph.AddNode("sc1", "cb1")

	sc3f3 := callGraph.AddNode("sc3", "f3")
	callGraph.AddAsyncCrossShardEdge(sc2f2, sc3f3, "cb2", "").
		SetGasLimit(100).
		SetGasUsed(6).
		SetGasUsedByCallback(3).
		SetCallbackFail()

	callGraph.AddNode("sc2", "cb2")

	return callGraph
}

// CreateGraphTestAsyncCallsAsyncCrossShard -
func CreateGraphTestAsyncCallsAsyncCrossShard() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 1000, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncCrossShardEdge(sc1f1, sc2f2, "cb1", "").
		SetGasLimit(500).
		SetGasUsed(7).
		SetGasUsedByCallback(5)

	callGraph.AddNode("sc1", "cb1")

	sc3f3 := callGraph.AddNode("sc3", "f3")
	callGraph.AddAsyncCrossShardEdge(sc2f2, sc3f3, "cb2", "").
		SetGasLimit(100).
		SetGasUsed(6).
		SetGasUsedByCallback(3)

	callGraph.AddNode("sc2", "cb2")

	return callGraph
}

// CreateGraphTestAsyncCallsAsyncFirstNoCallbackCrossShard -
func CreateGraphTestAsyncCallsAsyncFirstNoCallbackCrossShard() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 1000, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncCrossShardEdge(sc1f1, sc2f2, "", "").
		SetGasLimit(500).
		SetGasUsed(7)

	callGraph.AddNode("sc1", "cb1")

	sc3f3 := callGraph.AddNode("sc3", "f3")
	callGraph.AddAsyncCrossShardEdge(sc2f2, sc3f3, "cb2", "").
		SetGasLimit(100).
		SetGasUsed(6).
		SetGasUsedByCallback(3)

	callGraph.AddNode("sc2", "cb2")

	return callGraph
}

// CreateGraphTestAsyncCallsAsyncSecondNoCallbackCrossShard -
func CreateGraphTestAsyncCallsAsyncSecondNoCallbackCrossShard() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 1000, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncCrossShardEdge(sc1f1, sc2f2, "cb1", "").
		SetGasLimit(500).
		SetGasUsed(7).
		SetGasUsedByCallback(5)

	callGraph.AddNode("sc1", "cb1")

	sc3f3 := callGraph.AddNode("sc3", "f3")
	callGraph.AddAsyncCrossShardEdge(sc2f2, sc3f3, "", "").
		SetGasLimit(100).
		SetGasUsed(6)

	callGraph.AddNode("sc2", "cb2")

	return callGraph
}

// CreateGraphTestAsyncCallsAsyncFirstFailCrossShard -
func CreateGraphTestAsyncCallsAsyncFirstFailCrossShard() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 1000, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncCrossShardEdge(sc1f1, sc2f2, "cb1", "").
		SetGasLimit(500).
		SetGasUsed(7).
		SetGasUsedByCallback(5).
		SetFail()

	callGraph.AddNode("sc1", "cb1")

	sc3f3 := callGraph.AddNode("sc3", "f3")
	callGraph.AddAsyncCrossShardEdge(sc2f2, sc3f3, "cb2", "").
		SetGasLimit(100).
		SetGasUsed(6).
		SetGasUsedByCallback(3)

	callGraph.AddNode("sc2", "cb2")

	return callGraph
}

// CreateGraphTestAsyncCallsAsyncSecondFailCrossShard -
func CreateGraphTestAsyncCallsAsyncSecondFailCrossShard() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 1000, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncCrossShardEdge(sc1f1, sc2f2, "cb1", "").
		SetGasLimit(500).
		SetGasUsed(7).
		SetGasUsedByCallback(5)

	callGraph.AddNode("sc1", "cb1")

	sc3f3 := callGraph.AddNode("sc3", "f3")
	callGraph.AddAsyncCrossShardEdge(sc2f2, sc3f3, "cb2", "").
		SetGasLimit(100).
		SetGasUsed(6).
		SetGasUsedByCallback(3).
		SetFail()

	callGraph.AddNode("sc2", "cb2")

	return callGraph
}

// CreateGraphTestAsyncCallsAsyncFirstCallbackFailCrossShard -
func CreateGraphTestAsyncCallsAsyncFirstCallbackFailCrossShard() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 1000, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncCrossShardEdge(sc1f1, sc2f2, "cb1", "").
		SetGasLimit(500).
		SetGasUsed(7).
		SetGasUsedByCallback(5).
		SetCallbackFail()

	callGraph.AddNode("sc1", "cb1")

	sc3f3 := callGraph.AddNode("sc3", "f3")
	callGraph.AddAsyncCrossShardEdge(sc2f2, sc3f3, "cb2", "").
		SetGasLimit(100).
		SetGasUsed(6).
		SetGasUsedByCallback(3)

	callGraph.AddNode("sc2", "cb2")

	return callGraph
}

// CreateGraphTestAsyncCallsAsyncSecondCallbackFailCrossShard -
func CreateGraphTestAsyncCallsAsyncSecondCallbackFailCrossShard() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 1000, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncCrossShardEdge(sc1f1, sc2f2, "cb1", "").
		SetGasLimit(500).
		SetGasUsed(7).
		SetGasUsedByCallback(5)

	callGraph.AddNode("sc1", "cb1")

	sc3f3 := callGraph.AddNode("sc3", "f3")
	callGraph.AddAsyncCrossShardEdge(sc2f2, sc3f3, "cb2", "").
		SetGasLimit(100).
		SetGasUsed(6).
		SetGasUsedByCallback(3).
		SetCallbackFail()

	callGraph.AddNode("sc2", "cb2")

	return callGraph
}

// CreateGraphTestAsyncCallsAsyncBothCallbacksFailCrossShard -
func CreateGraphTestAsyncCallsAsyncBothCallbacksFailCrossShard() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 1000, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncCrossShardEdge(sc1f1, sc2f2, "cb1", "").
		SetGasLimit(500).
		SetGasUsed(7).
		SetGasUsedByCallback(5).
		SetCallbackFail()

	callGraph.AddNode("sc1", "cb1")

	sc3f3 := callGraph.AddNode("sc3", "f3")
	callGraph.AddAsyncCrossShardEdge(sc2f2, sc3f3, "cb2", "").
		SetGasLimit(100).
		SetGasUsed(6).
		SetGasUsedByCallback(3).
		SetCallbackFail()

	callGraph.AddNode("sc2", "cb2")

	return callGraph
}

// CreateGraphTestCallbackCallsSync -
func CreateGraphTestCallbackCallsSync() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 2000, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncEdge(sc1f1, sc2f2, "cb1", "").
		SetGasLimit(1000).
		SetGasUsed(70).
		SetGasUsedByCallback(500)

	sc1cb1 := callGraph.AddNode("sc1", "cb1")

	sc2f3 := callGraph.AddNode("sc2", "f3")
	callGraph.AddSyncEdge(sc1cb1, sc2f3).
		SetGasLimit(200).
		SetGasUsed(60)

	callGraph.AddNode("sc1", "cb2")

	return callGraph
}

// CreateGraphTestCallbackCallsAsyncFailLocalLocal -
func CreateGraphTestCallbackCallsAsyncFailLocalLocal() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 2000, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncEdge(sc1f1, sc2f2, "cb1", "").
		SetGasLimit(1000).
		SetGasUsed(70).
		SetGasUsedByCallback(500)

	sc1cb1 := callGraph.AddNode("sc1", "cb1")

	sc3f3 := callGraph.AddNode("sc3", "f3")
	callGraph.AddAsyncEdge(sc1cb1, sc3f3, "cb2", "").
		SetGasLimit(200).
		SetGasUsed(60).
		SetGasUsedByCallback(30).
		SetFail()

	callGraph.AddNode("sc1", "cb2")

	return callGraph
}

// CreateGraphTestCallbackCallsAsyncCallbackFailLocalLocal -
func CreateGraphTestCallbackCallsAsyncCallbackFailLocalLocal() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 2000, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncEdge(sc1f1, sc2f2, "cb1", "").
		SetGasLimit(1000).
		SetGasUsed(70).
		SetGasUsedByCallback(500)

	sc1cb1 := callGraph.AddNode("sc1", "cb1")

	sc3f3 := callGraph.AddNode("sc3", "f3")
	callGraph.AddAsyncEdge(sc1cb1, sc3f3, "cb2", "").
		SetGasLimit(200).
		SetGasUsed(60).
		SetGasUsedByCallback(30).
		SetCallbackFail()

	callGraph.AddNode("sc1", "cb2")

	return callGraph
}

// CreateGraphTestCallbackCallsAsyncLocalLocal -
func CreateGraphTestCallbackCallsAsyncLocalLocal() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 2000, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncEdge(sc1f1, sc2f2, "cb1", "").
		SetGasLimit(1000).
		SetGasUsed(70).
		SetGasUsedByCallback(500)

	sc1cb1 := callGraph.AddNode("sc1", "cb1")

	sc3f3 := callGraph.AddNode("sc3", "f3")
	callGraph.AddAsyncEdge(sc1cb1, sc3f3, "cb2", "").
		SetGasLimit(200).
		SetGasUsed(60).
		SetGasUsedByCallback(30)

	callGraph.AddNode("sc1", "cb2")

	return callGraph
}

// CreateGraphTestCallbackCallsAsyncLocalCross -
func CreateGraphTestCallbackCallsAsyncLocalCross() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 2000, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncEdge(sc1f1, sc2f2, "cb1", "").
		SetGasLimit(1000).
		SetGasUsed(70).
		SetGasUsedByCallback(500)

	sc1cb1 := callGraph.AddNode("sc1", "cb1")

	sc3f3 := callGraph.AddNode("sc3", "f3")
	callGraph.AddAsyncCrossShardEdge(sc1cb1, sc3f3, "cb2", "").
		SetGasLimit(200).
		SetGasUsed(60).
		SetGasUsedByCallback(30)

	callGraph.AddNode("sc1", "cb2")

	return callGraph
}

// CreateGraphTestCallbackCallsAsyncFailLocalCross -
func CreateGraphTestCallbackCallsAsyncFailLocalCross() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 2000, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncEdge(sc1f1, sc2f2, "cb1", "").
		SetGasLimit(1000).
		SetGasUsed(70).
		SetGasUsedByCallback(500)

	sc1cb1 := callGraph.AddNode("sc1", "cb1")

	sc3f3 := callGraph.AddNode("sc3", "f3")
	callGraph.AddAsyncCrossShardEdge(sc1cb1, sc3f3, "cb2", "").
		SetGasLimit(200).
		SetGasUsed(60).
		SetGasUsedByCallback(30).
		SetFail()

	callGraph.AddNode("sc1", "cb2")

	return callGraph
}

// CreateGraphTestCallbackCallsAsyncCallbackFailLocalCross -
func CreateGraphTestCallbackCallsAsyncCallbackFailLocalCross() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 2000, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncEdge(sc1f1, sc2f2, "cb1", "").
		SetGasLimit(1000).
		SetGasUsed(70).
		SetGasUsedByCallback(500)

	sc1cb1 := callGraph.AddNode("sc1", "cb1")

	sc3f3 := callGraph.AddNode("sc3", "f3")
	callGraph.AddAsyncCrossShardEdge(sc1cb1, sc3f3, "cb2", "").
		SetGasLimit(200).
		SetGasUsed(60).
		SetGasUsedByCallback(30).
		SetCallbackFail()

	callGraph.AddNode("sc1", "cb2")

	return callGraph
}

// CreateGraphTestCallbackCallsAsyncCrossLocal -
func CreateGraphTestCallbackCallsAsyncCrossLocal() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 2000, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncCrossShardEdge(sc1f1, sc2f2, "cb1", "").
		SetGasLimit(1000).
		SetGasUsed(70).
		SetGasUsedByCallback(500)

	sc1cb1 := callGraph.AddNode("sc1", "cb1")

	sc3f3 := callGraph.AddNode("sc3", "f3")
	callGraph.AddAsyncEdge(sc1cb1, sc3f3, "cb2", "").
		SetGasLimit(200).
		SetGasUsed(60).
		SetGasUsedByCallback(30)

	callGraph.AddNode("sc1", "cb2")

	return callGraph
}

// CreateGraphTestCallbackCallsAsyncFailCrossLocal -
func CreateGraphTestCallbackCallsAsyncFailCrossLocal() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 2000, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncCrossShardEdge(sc1f1, sc2f2, "cb1", "").
		SetGasLimit(1000).
		SetGasUsed(70).
		SetGasUsedByCallback(500)

	sc1cb1 := callGraph.AddNode("sc1", "cb1")

	sc3f3 := callGraph.AddNode("sc3", "f3")
	callGraph.AddAsyncEdge(sc1cb1, sc3f3, "cb2", "").
		SetGasLimit(200).
		SetGasUsed(60).
		SetGasUsedByCallback(30).
		SetFail()

	callGraph.AddNode("sc1", "cb2")

	return callGraph
}

// CreateGraphTestCallbackCallsAsyncCallbackFailCrossLocal -
func CreateGraphTestCallbackCallsAsyncCallbackFailCrossLocal() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 2000, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncCrossShardEdge(sc1f1, sc2f2, "cb1", "").
		SetGasLimit(1000).
		SetGasUsed(70).
		SetGasUsedByCallback(500)

	sc1cb1 := callGraph.AddNode("sc1", "cb1")

	sc3f3 := callGraph.AddNode("sc3", "f3")
	callGraph.AddAsyncEdge(sc1cb1, sc3f3, "cb2", "").
		SetGasLimit(200).
		SetGasUsed(60).
		SetGasUsedByCallback(30).
		SetCallbackFail()

	callGraph.AddNode("sc1", "cb2")

	return callGraph
}

// CreateGraphTestCallbackCallsAsyncCrossCross -
func CreateGraphTestCallbackCallsAsyncCrossCross() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 2000, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncCrossShardEdge(sc1f1, sc2f2, "cb1", "").
		SetGasLimit(1000).
		SetGasUsed(70).
		SetGasUsedByCallback(500)

	sc1cb1 := callGraph.AddNode("sc1", "cb1")

	sc2f3 := callGraph.AddNode("sc2", "f3")
	callGraph.AddAsyncCrossShardEdge(sc1cb1, sc2f3, "cb2", "").
		SetGasLimit(200).
		SetGasUsed(60).
		SetGasUsedByCallback(30)

	callGraph.AddNode("sc1", "cb2")

	return callGraph
}

// CreateGraphTestCallbackCallsAsyncFailCrossCross -
func CreateGraphTestCallbackCallsAsyncFailCrossCross() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 2000, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncCrossShardEdge(sc1f1, sc2f2, "cb1", "").
		SetGasLimit(1000).
		SetGasUsed(70).
		SetGasUsedByCallback(500)

	sc1cb1 := callGraph.AddNode("sc1", "cb1")

	sc2f3 := callGraph.AddNode("sc2", "f3")
	callGraph.AddAsyncCrossShardEdge(sc1cb1, sc2f3, "cb2", "").
		SetGasLimit(200).
		SetGasUsed(60).
		SetGasUsedByCallback(30).
		SetFail()

	callGraph.AddNode("sc1", "cb2")

	return callGraph
}

// CreateGraphTestCallbackCallsAsyncCallbackFailCrossCross -
func CreateGraphTestCallbackCallsAsyncCallbackFailCrossCross() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 2000, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncCrossShardEdge(sc1f1, sc2f2, "cb1", "").
		SetGasLimit(1000).
		SetGasUsed(70).
		SetGasUsedByCallback(500)

	sc1cb1 := callGraph.AddNode("sc1", "cb1")

	sc2f3 := callGraph.AddNode("sc2", "f3")
	callGraph.AddAsyncCrossShardEdge(sc1cb1, sc2f3, "cb2", "").
		SetGasLimit(200).
		SetGasUsed(60).
		SetGasUsedByCallback(30).
		SetCallbackFail()

	callGraph.AddNode("sc1", "cb2")

	return callGraph
}

// CreateGraphTestSyncCalls -
func CreateGraphTestSyncCalls() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 500, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddSyncEdge(sc1f1, sc2f2).
		SetGasLimit(100).
		SetGasUsed(7)

	sc3f3 := callGraph.AddNode("sc3", "f3")
	callGraph.AddSyncEdge(sc1f1, sc3f3).
		SetGasLimit(100).
		SetGasUsed(7)

	sc4f4 := callGraph.AddNode("sc4", "f4")
	callGraph.AddSyncEdge(sc3f3, sc4f4).
		SetGasLimit(35).
		SetGasUsed(7)

	sc5f5 := callGraph.AddNode("sc5", "f5")
	callGraph.AddSyncEdge(sc3f3, sc5f5).
		SetGasLimit(35).
		SetGasUsed(7)

	return callGraph
}

// CreateGraphTestSyncCalls2 -
func CreateGraphTestSyncCalls2() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 500, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddSyncEdge(sc1f1, sc2f2).
		SetGasLimit(100).
		SetGasUsed(7)

	sc3f3 := callGraph.AddNode("sc3", "f3")
	callGraph.AddSyncEdge(sc2f2, sc3f3).
		SetGasLimit(50).
		SetGasUsed(7)

	return callGraph
}

// CreateGraphTestSyncCallsFailPropagation -
func CreateGraphTestSyncCallsFailPropagation() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 1000, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddSyncEdge(sc1f1, sc2f2).
		SetGasLimit(500).
		SetGasUsed(7)

	sc3f4 := callGraph.AddNode("sc3", "f4")
	callGraph.AddSyncEdge(sc2f2, sc3f4).
		SetGasLimit(10).
		SetGasUsed(3).
		SetFail()

	// callGraph.AddNode("sc1", "cb1")

	sc3f3 := callGraph.AddNode("sc3", "f3")
	callGraph.AddAsyncEdge(sc2f2, sc3f3, "cb2", "").
		SetGasLimit(100).
		SetGasUsed(6).
		SetGasUsedByCallback(3)

	callGraph.AddNode("sc2", "cb2")

	return callGraph
}

// CreateGraphTestOneAsyncCall -
func CreateGraphTestOneAsyncCall() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 500, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncEdge(sc1f1, sc2f2, "cb1", "").
		SetGasLimit(35).
		SetGasUsed(7).
		SetGasUsedByCallback(6)

	callGraph.AddNode("sc1", "cb1")

	return callGraph
}

// CreateGraphTestOneAsyncCallCustomGasLocked -
func CreateGraphTestOneAsyncCallCustomGasLocked() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 500, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncEdge(sc1f1, sc2f2, "cb1", "").
		SetGasLimit(35).
		SetGasUsed(7).
		SetGasLocked(100).
		SetGasUsedByCallback(6)

	callGraph.AddNode("sc1", "cb1")

	return callGraph
}

// CreateGraphTestOneAsyncCallNoCallback -
func CreateGraphTestOneAsyncCallNoCallback() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 500, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncEdge(sc1f1, sc2f2, "", "").
		SetGasLimit(35).
		SetGasUsed(7)

	return callGraph
}

// CreateGraphTestOneAsyncCallFail -
func CreateGraphTestOneAsyncCallFail() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 500, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncEdge(sc1f1, sc2f2, "cb1", "").
		SetGasLimit(35).
		SetGasUsed(7).
		SetGasUsedByCallback(6).
		SetFail()
	sc1cb1 := callGraph.AddNode("sc1", "cb1")

	sc3f3 := callGraph.AddNode("sc3", "f3")
	callGraph.AddSyncEdge(sc2f2, sc3f3).
		SetGasLimit(10).
		SetGasUsed(4)

	sc4f4 := callGraph.AddNode("sc4", "f4")
	callGraph.AddSyncEdge(sc1cb1, sc4f4).
		SetGasLimit(100).
		SetGasUsed(40)

	return callGraph
}

// CreateGraphTestOneAsyncCallNoCallbackFail -
func CreateGraphTestOneAsyncCallNoCallbackFail() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 500, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncEdge(sc1f1, sc2f2, "", "").
		SetGasLimit(35).
		SetGasUsed(7).
		SetGasUsedByCallback(6).
		SetFail()

	callGraph.AddNode("sc1", "cb1")

	return callGraph
}

// CreateGraphTestAsyncCallIndirectFailCrossShard -
func CreateGraphTestAsyncCallIndirectFailCrossShard() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 500, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncCrossShardEdge(sc1f1, sc2f2, "cb1", "").
		SetGasLimit(35).
		SetGasUsed(7).
		SetGasUsedByCallback(6)

	sc1cb1 := callGraph.AddNode("sc1", "cb1")

	sc6f6 := callGraph.AddNode("sc6", "f6")
	callGraph.AddSyncEdge(sc2f2, sc6f6).
		SetGasLimit(8).
		SetGasUsed(3)

	sc3f3 := callGraph.AddNode("sc3", "f3")
	callGraph.AddSyncEdge(sc2f2, sc3f3).
		SetGasLimit(10).
		SetGasUsed(4)

	sc5f5 := callGraph.AddNode("sc5", "f5")
	callGraph.AddSyncEdge(sc3f3, sc5f5).
		SetGasLimit(3).
		SetGasUsed(1).
		SetFail()

	sc7f7 := callGraph.AddNode("sc7", "f7")
	callGraph.AddSyncEdge(sc2f2, sc7f7).
		SetGasLimit(5).
		SetGasUsed(2)

	sc4f4 := callGraph.AddNode("sc4", "f4")
	callGraph.AddSyncEdge(sc1cb1, sc4f4).
		SetGasLimit(100).
		SetGasUsed(40)

	return callGraph
}

// CreateGraphTestAsyncCallIndirectFail -
func CreateGraphTestAsyncCallIndirectFail() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 500, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncEdge(sc1f1, sc2f2, "cb1", "").
		SetGasLimit(35).
		SetGasUsed(7).
		SetGasUsedByCallback(6)
	sc1cb1 := callGraph.AddNode("sc1", "cb1")

	sc6f6 := callGraph.AddNode("sc6", "f6")
	callGraph.AddSyncEdge(sc2f2, sc6f6).
		SetGasLimit(8).
		SetGasUsed(3)

	sc3f3 := callGraph.AddNode("sc3", "f3")
	callGraph.AddSyncEdge(sc2f2, sc3f3).
		SetGasLimit(10).
		SetGasUsed(4)

	sc5f5 := callGraph.AddNode("sc5", "f5")
	callGraph.AddSyncEdge(sc3f3, sc5f5).
		SetGasLimit(3).
		SetGasUsed(1).
		SetFail()

	sc7f7 := callGraph.AddNode("sc7", "f7")
	callGraph.AddSyncEdge(sc2f2, sc7f7).
		SetGasLimit(5).
		SetGasUsed(2)

	sc4f4 := callGraph.AddNode("sc4", "f4")
	callGraph.AddSyncEdge(sc1cb1, sc4f4).
		SetGasLimit(100).
		SetGasUsed(40)

	return callGraph
}

// CreateGraphTestOneAsyncCallbackFail -
func CreateGraphTestOneAsyncCallbackFail() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 500, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncEdge(sc1f1, sc2f2, "cb1", "").
		SetGasLimit(35).
		SetGasUsed(7).
		SetGasUsedByCallback(6).
		SetCallbackFail()
	sc1cb1 := callGraph.AddNode("sc1", "cb1")

	sc3f3 := callGraph.AddNode("sc3", "f3")
	callGraph.AddSyncEdge(sc2f2, sc3f3).
		SetGasLimit(10).
		SetGasUsed(4)

	sc4f4 := callGraph.AddNode("sc4", "f4")
	callGraph.AddSyncEdge(sc1cb1, sc4f4).
		SetGasLimit(100).
		SetGasUsed(40)

	return callGraph
}

// CreateGraphTestAsyncCallbackIndirectFail -
func CreateGraphTestAsyncCallbackIndirectFail() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 500, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncEdge(sc1f1, sc2f2, "cb1", "").
		SetGasLimit(35).
		SetGasUsed(7).
		SetGasUsedByCallback(6)
	sc1cb1 := callGraph.AddNode("sc1", "cb1")

	sc3f3 := callGraph.AddNode("sc3", "f3")
	callGraph.AddSyncEdge(sc2f2, sc3f3).
		SetGasLimit(10).
		SetGasUsed(4)

	sc4f4 := callGraph.AddNode("sc4", "f4")
	callGraph.AddSyncEdge(sc1cb1, sc4f4).
		SetGasLimit(100).
		SetGasUsed(40)

	sc5f5 := callGraph.AddNode("sc5", "f5")
	callGraph.AddSyncEdge(sc1cb1, sc5f5).
		SetGasLimit(10).
		SetGasUsed(4).
		SetFail()

	return callGraph
}

// CreateGraphTestOneAsyncCallbackFailCrossShard -
func CreateGraphTestOneAsyncCallbackFailCrossShard() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 500, 10)

	sc5f5 := callGraph.AddNode("sc5", "f5")
	callGraph.AddSyncEdge(sc1f1, sc5f5).
		SetGasLimit(400).
		SetGasUsed(6)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncCrossShardEdge(sc5f5, sc2f2, "cb1", "").
		SetGasLimit(35).
		SetGasUsed(7).
		SetGasUsedByCallback(6).
		SetCallbackFail()
	sc5cb1 := callGraph.AddNode("sc5", "cb1")

	sc3f3 := callGraph.AddNode("sc3", "f3")
	callGraph.AddSyncEdge(sc2f2, sc3f3).
		SetGasLimit(10).
		SetGasUsed(4)

	sc4f4 := callGraph.AddNode("sc4", "f4")
	callGraph.AddSyncEdge(sc5cb1, sc4f4).
		SetGasLimit(100).
		SetGasUsed(40)

	return callGraph
}

// CreateGraphTestAsyncCallbackIndirectFailCrossShard -
func CreateGraphTestAsyncCallbackIndirectFailCrossShard() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 500, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncCrossShardEdge(sc1f1, sc2f2, "cb1", "").
		SetGasLimit(35).
		SetGasUsed(7).
		SetGasUsedByCallback(6)
	sc1cb1 := callGraph.AddNode("sc1", "cb1")

	sc3f3 := callGraph.AddNode("sc3", "f3")
	callGraph.AddSyncEdge(sc2f2, sc3f3).
		SetGasLimit(10).
		SetGasUsed(4)

	sc6f6 := callGraph.AddNode("sc6", "f6")
	callGraph.AddSyncEdge(sc1cb1, sc6f6).
		SetGasLimit(10).
		SetGasUsed(4)

	sc4f4 := callGraph.AddNode("sc4", "f4")
	callGraph.AddSyncEdge(sc1cb1, sc4f4).
		SetGasLimit(100).
		SetGasUsed(40)

	sc5f5 := callGraph.AddNode("sc5", "f5")
	callGraph.AddSyncEdge(sc4f4, sc5f5).
		SetGasLimit(5).
		SetGasUsed(2).
		SetFail()

	sc7f7 := callGraph.AddNode("sc7", "f7")
	callGraph.AddSyncEdge(sc1cb1, sc7f7).
		SetGasLimit(12).
		SetGasUsed(2)

	return callGraph
}

// CreateGraphTestTwoAsyncCalls -
func CreateGraphTestTwoAsyncCalls() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 1000, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncEdge(sc1f1, sc2f2, "cb1", "").
		SetGasLimit(20).
		SetGasUsed(7).
		SetGasUsedByCallback(5)

	sc3f3 := callGraph.AddNode("sc3", "f3")
	callGraph.AddAsyncEdge(sc1f1, sc3f3, "cb2", "").
		SetGasLimit(30).
		SetGasUsed(6).
		SetGasUsedByCallback(3)

	callGraph.AddNode("sc1", "cb1")
	callGraph.AddNode("sc1", "cb2")

	return callGraph
}

// CreateGraphTestTwoAsyncCallsFirstNoCallback -
func CreateGraphTestTwoAsyncCallsFirstNoCallback() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 1000, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncEdge(sc1f1, sc2f2, "", "").
		SetGasLimit(20).
		SetGasUsed(7)

	sc3f3 := callGraph.AddNode("sc3", "f3")
	callGraph.AddAsyncEdge(sc1f1, sc3f3, "cb2", "").
		SetGasLimit(30).
		SetGasUsed(6).
		SetGasUsedByCallback(3)

	callGraph.AddNode("sc1", "cb1")
	callGraph.AddNode("sc1", "cb2")

	return callGraph
}

// CreateGraphTestTwoAsyncCallsSecondNoCallback -
func CreateGraphTestTwoAsyncCallsSecondNoCallback() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 1000, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncEdge(sc1f1, sc2f2, "cb1", "").
		SetGasLimit(20).
		SetGasUsed(7).
		SetGasUsedByCallback(5)

	sc3f3 := callGraph.AddNode("sc3", "f3")
	callGraph.AddAsyncEdge(sc1f1, sc3f3, "", "").
		SetGasLimit(30).
		SetGasUsed(6)

	callGraph.AddNode("sc1", "cb1")
	callGraph.AddNode("sc1", "cb2")

	return callGraph
}

// CreateGraphTestTwoAsyncCallsFirstFail -
func CreateGraphTestTwoAsyncCallsFirstFail() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 1000, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncEdge(sc1f1, sc2f2, "cb1", "").
		SetGasLimit(20).
		SetGasUsed(7).
		SetGasUsedByCallback(5).
		SetFail()

	sc4f4 := callGraph.AddNode("sc4", "f4")
	callGraph.AddSyncEdge(sc2f2, sc4f4).
		SetGasLimit(5).
		SetGasUsed(2)

	sc3f3 := callGraph.AddNode("sc3", "f3")
	callGraph.AddAsyncEdge(sc1f1, sc3f3, "cb2", "").
		SetGasLimit(30).
		SetGasUsed(6).
		SetGasUsedByCallback(3)

	sc5f5 := callGraph.AddNode("sc5", "f5")
	callGraph.AddSyncEdge(sc3f3, sc5f5).
		SetGasLimit(4).
		SetGasUsed(1)

	callGraph.AddNode("sc1", "cb1")
	callGraph.AddNode("sc1", "cb2")

	return callGraph
}

// CreateGraphTestTwoAsyncCallsSecondFail -
func CreateGraphTestTwoAsyncCallsSecondFail() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 1000, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncEdge(sc1f1, sc2f2, "cb1", "").
		SetGasLimit(20).
		SetGasUsed(7).
		SetGasUsedByCallback(5)

	sc4f4 := callGraph.AddNode("sc4", "f4")
	callGraph.AddSyncEdge(sc2f2, sc4f4).
		SetGasLimit(5).
		SetGasUsed(2)

	sc3f3 := callGraph.AddNode("sc3", "f3")
	callGraph.AddAsyncEdge(sc1f1, sc3f3, "cb2", "").
		SetGasLimit(30).
		SetGasUsed(6).
		SetGasUsedByCallback(3).
		SetFail()

	sc5f5 := callGraph.AddNode("sc5", "f5")
	callGraph.AddSyncEdge(sc3f3, sc5f5).
		SetGasLimit(4).
		SetGasUsed(1)

	callGraph.AddNode("sc1", "cb1")
	callGraph.AddNode("sc1", "cb2")

	return callGraph
}

// CreateGraphTestTwoAsyncCallsBothFail -
func CreateGraphTestTwoAsyncCallsBothFail() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 1000, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncEdge(sc1f1, sc2f2, "cb1", "").
		SetGasLimit(20).
		SetGasUsed(7).
		SetGasUsedByCallback(5).
		SetFail()

	sc4f4 := callGraph.AddNode("sc4", "f4")
	callGraph.AddSyncEdge(sc2f2, sc4f4).
		SetGasLimit(5).
		SetGasUsed(2)

	sc3f3 := callGraph.AddNode("sc3", "f3")
	callGraph.AddAsyncEdge(sc1f1, sc3f3, "cb2", "").
		SetGasLimit(30).
		SetGasUsed(6).
		SetGasUsedByCallback(3).
		SetFail()

	sc5f5 := callGraph.AddNode("sc5", "f5")
	callGraph.AddSyncEdge(sc3f3, sc5f5).
		SetGasLimit(4).
		SetGasUsed(1)

	callGraph.AddNode("sc1", "cb1")
	callGraph.AddNode("sc1", "cb2")

	return callGraph
}

// CreateGraphTestTwoAsyncCallsFirstCallbackFail -
func CreateGraphTestTwoAsyncCallsFirstCallbackFail() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 1000, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncEdge(sc1f1, sc2f2, "cb1", "").
		SetGasLimit(20).
		SetGasUsed(7).
		SetGasUsedByCallback(5).
		SetCallbackFail()

	sc4f4 := callGraph.AddNode("sc4", "f4")
	callGraph.AddSyncEdge(sc2f2, sc4f4).
		SetGasLimit(5).
		SetGasUsed(2)

	sc3f3 := callGraph.AddNode("sc3", "f3")
	callGraph.AddAsyncEdge(sc1f1, sc3f3, "cb2", "").
		SetGasLimit(30).
		SetGasUsed(6).
		SetGasUsedByCallback(3)

	sc5f5 := callGraph.AddNode("sc5", "f5")
	callGraph.AddSyncEdge(sc3f3, sc5f5).
		SetGasLimit(4).
		SetGasUsed(1)

	callGraph.AddNode("sc1", "cb1")
	callGraph.AddNode("sc1", "cb2")

	return callGraph
}

// CreateGraphTestTwoAsyncCallsSecondCallbackFail -
func CreateGraphTestTwoAsyncCallsSecondCallbackFail() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 1000, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncEdge(sc1f1, sc2f2, "cb1", "").
		SetGasLimit(20).
		SetGasUsed(7).
		SetGasUsedByCallback(5)

	sc4f4 := callGraph.AddNode("sc4", "f4")
	callGraph.AddSyncEdge(sc2f2, sc4f4).
		SetGasLimit(5).
		SetGasUsed(2)

	sc3f3 := callGraph.AddNode("sc3", "f3")
	callGraph.AddAsyncEdge(sc1f1, sc3f3, "cb2", "").
		SetGasLimit(30).
		SetGasUsed(6).
		SetGasUsedByCallback(3).
		SetCallbackFail()

	sc5f5 := callGraph.AddNode("sc5", "f5")
	callGraph.AddSyncEdge(sc3f3, sc5f5).
		SetGasLimit(4).
		SetGasUsed(1)

	callGraph.AddNode("sc1", "cb1")
	callGraph.AddNode("sc1", "cb2")

	return callGraph
}

// CreateGraphTestTwoAsyncCallsBothCallbacksFail -
func CreateGraphTestTwoAsyncCallsBothCallbacksFail() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 1000, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncEdge(sc1f1, sc2f2, "cb1", "").
		SetGasLimit(20).
		SetGasUsed(7).
		SetGasUsedByCallback(5).
		SetCallbackFail()

	sc4f4 := callGraph.AddNode("sc4", "f4")
	callGraph.AddSyncEdge(sc2f2, sc4f4).
		SetGasLimit(5).
		SetGasUsed(2)

	sc3f3 := callGraph.AddNode("sc3", "f3")
	callGraph.AddAsyncEdge(sc1f1, sc3f3, "cb2", "").
		SetGasLimit(30).
		SetGasUsed(6).
		SetGasUsedByCallback(3).
		SetCallbackFail()

	sc5f5 := callGraph.AddNode("sc5", "f5")
	callGraph.AddSyncEdge(sc3f3, sc5f5).
		SetGasLimit(4).
		SetGasUsed(1)

	callGraph.AddNode("sc1", "cb1")
	callGraph.AddNode("sc1", "cb2")

	return callGraph
}

// CreateGraphTestTwoAsyncCallsLocalCross -
func CreateGraphTestTwoAsyncCallsLocalCross() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 1000, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncEdge(sc1f1, sc2f2, "cb1", "").
		SetGasLimit(20).
		SetGasUsed(7).
		SetGasUsedByCallback(5)

	s3f3 := callGraph.AddNode("s3", "f3")
	callGraph.AddAsyncCrossShardEdge(sc1f1, s3f3, "cb2", "").
		SetGasLimit(30).
		SetGasUsed(6).
		SetGasUsedByCallback(3)

	callGraph.AddNode("sc1", "cb1")
	callGraph.AddNode("sc1", "cb2")

	return callGraph
}

// CreateGraphTestTwoAsyncFirstNoCallbackCallsLocalCross -
func CreateGraphTestTwoAsyncFirstNoCallbackCallsLocalCross() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 1000, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncEdge(sc1f1, sc2f2, "cb1", "").
		SetGasLimit(20).
		SetGasUsed(7).
		SetGasUsedByCallback(5)

	s3f3 := callGraph.AddNode("s3", "f3")
	callGraph.AddAsyncCrossShardEdge(sc1f1, s3f3, "cb2", "").
		SetGasLimit(30).
		SetGasUsed(6).
		SetGasUsedByCallback(3)

	callGraph.AddNode("sc1", "cb1")
	callGraph.AddNode("sc1", "cb2")

	return callGraph
}

// CreateGraphTestTwoAsyncSecondNoCallbackCallsLocalCross -
func CreateGraphTestTwoAsyncSecondNoCallbackCallsLocalCross() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 1000, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncEdge(sc1f1, sc2f2, "cb1", "").
		SetGasLimit(20).
		SetGasUsed(7).
		SetGasUsedByCallback(5)

	s3f3 := callGraph.AddNode("s3", "f3")
	callGraph.AddAsyncCrossShardEdge(sc1f1, s3f3, "cb2", "").
		SetGasLimit(30).
		SetGasUsed(6).
		SetGasUsedByCallback(3)

	callGraph.AddNode("sc1", "cb1")
	callGraph.AddNode("sc1", "cb2")

	return callGraph
}

// CreateGraphTestTwoAsyncCallsFirstFailLocalCross -
func CreateGraphTestTwoAsyncCallsFirstFailLocalCross() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 1000, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncEdge(sc1f1, sc2f2, "cb1", "").
		SetGasLimit(20).
		SetGasUsed(7).
		SetGasUsedByCallback(5).
		SetFail()

	s3f3 := callGraph.AddNode("s3", "f3")
	callGraph.AddAsyncCrossShardEdge(sc1f1, s3f3, "cb2", "").
		SetGasLimit(30).
		SetGasUsed(6).
		SetGasUsedByCallback(3)

	callGraph.AddNode("sc1", "cb1")
	callGraph.AddNode("sc1", "cb2")

	return callGraph
}

// CreateGraphTestTwoAsyncCallsSecondFailLocalCross -
func CreateGraphTestTwoAsyncCallsSecondFailLocalCross() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 1000, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncEdge(sc1f1, sc2f2, "cb1", "").
		SetGasLimit(20).
		SetGasUsed(7).
		SetGasUsedByCallback(5)

	s3f3 := callGraph.AddNode("s3", "f3")
	callGraph.AddAsyncCrossShardEdge(sc1f1, s3f3, "cb2", "").
		SetGasLimit(30).
		SetGasUsed(6).
		SetGasUsedByCallback(3).
		SetFail()

	callGraph.AddNode("sc1", "cb1")
	callGraph.AddNode("sc1", "cb2")

	return callGraph
}

// CreateGraphTestTwoAsyncCallsBothFailLocalCross -
func CreateGraphTestTwoAsyncCallsBothFailLocalCross() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 1000, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncEdge(sc1f1, sc2f2, "cb1", "").
		SetGasLimit(20).
		SetGasUsed(7).
		SetGasUsedByCallback(5).
		SetFail()

	s3f3 := callGraph.AddNode("s3", "f3")
	callGraph.AddAsyncCrossShardEdge(sc1f1, s3f3, "cb2", "").
		SetGasLimit(30).
		SetGasUsed(6).
		SetGasUsedByCallback(3).
		SetFail()

	callGraph.AddNode("sc1", "cb1")
	callGraph.AddNode("sc1", "cb2")

	return callGraph
}

// CreateGraphTestTwoAsyncCallsFirstCallbackFailLocalCross -
func CreateGraphTestTwoAsyncCallsFirstCallbackFailLocalCross() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 1000, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncEdge(sc1f1, sc2f2, "cb1", "").
		SetGasLimit(20).
		SetGasUsed(7).
		SetGasUsedByCallback(5).
		SetCallbackFail()

	s3f3 := callGraph.AddNode("s3", "f3")
	callGraph.AddAsyncCrossShardEdge(sc1f1, s3f3, "cb2", "").
		SetGasLimit(30).
		SetGasUsed(6).
		SetGasUsedByCallback(3)

	callGraph.AddNode("sc1", "cb1")
	callGraph.AddNode("sc1", "cb2")

	return callGraph
}

// CreateGraphTestTwoAsyncCallsSecondCallbackFailLocalCross -
func CreateGraphTestTwoAsyncCallsSecondCallbackFailLocalCross() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 1000, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncEdge(sc1f1, sc2f2, "cb1", "").
		SetGasLimit(20).
		SetGasUsed(7).
		SetGasUsedByCallback(5)

	s3f3 := callGraph.AddNode("s3", "f3")
	callGraph.AddAsyncCrossShardEdge(sc1f1, s3f3, "cb2", "").
		SetGasLimit(30).
		SetGasUsed(6).
		SetGasUsedByCallback(3).
		SetCallbackFail()

	callGraph.AddNode("sc1", "cb1")
	callGraph.AddNode("sc1", "cb2")

	return callGraph
}

// CreateGraphTestTwoAsyncCallsBothCallbacksFailLocalCross -
func CreateGraphTestTwoAsyncCallsBothCallbacksFailLocalCross() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 1000, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncEdge(sc1f1, sc2f2, "cb1", "").
		SetGasLimit(20).
		SetGasUsed(7).
		SetGasUsedByCallback(5).
		SetCallbackFail()

	s3f3 := callGraph.AddNode("s3", "f3")
	callGraph.AddAsyncCrossShardEdge(sc1f1, s3f3, "cb2", "").
		SetGasLimit(30).
		SetGasUsed(6).
		SetGasUsedByCallback(3).
		SetCallbackFail()

	callGraph.AddNode("sc1", "cb1")
	callGraph.AddNode("sc1", "cb2")

	return callGraph
}

// CreateGraphTestTwoAsyncCallsCrossLocal -
func CreateGraphTestTwoAsyncCallsCrossLocal() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 1000, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncCrossShardEdge(sc1f1, sc2f2, "cb1", "").
		SetGasLimit(20).
		SetGasUsed(7).
		SetGasUsedByCallback(5)

	s3f3 := callGraph.AddNode("s3", "f3")
	callGraph.AddAsyncEdge(sc1f1, s3f3, "cb2", "").
		SetGasLimit(30).
		SetGasUsed(6).
		SetGasUsedByCallback(3)

	callGraph.AddNode("sc1", "cb1")
	callGraph.AddNode("sc1", "cb2")

	return callGraph
}

// CreateGraphTestTwoAsyncCallsFirstNoCallbackCrossLocal -
func CreateGraphTestTwoAsyncCallsFirstNoCallbackCrossLocal() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 1000, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncCrossShardEdge(sc1f1, sc2f2, "cb1", "").
		SetGasLimit(20).
		SetGasUsed(7).
		SetGasUsedByCallback(5)

	s3f3 := callGraph.AddNode("s3", "f3")
	callGraph.AddAsyncEdge(sc1f1, s3f3, "cb2", "").
		SetGasLimit(30).
		SetGasUsed(6).
		SetGasUsedByCallback(3)

	callGraph.AddNode("sc1", "cb1")
	callGraph.AddNode("sc1", "cb2")

	return callGraph
}

// CreateGraphTestTwoAsyncCallsSecondNoCallbackCrossLocal -
func CreateGraphTestTwoAsyncCallsSecondNoCallbackCrossLocal() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 1000, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncCrossShardEdge(sc1f1, sc2f2, "", "").
		SetGasLimit(20).
		SetGasUsed(7)

	s3f3 := callGraph.AddNode("s3", "f3")
	callGraph.AddAsyncEdge(sc1f1, s3f3, "cb2", "").
		SetGasLimit(30).
		SetGasUsed(6).
		SetGasUsedByCallback(3)

	callGraph.AddNode("sc1", "cb1")
	callGraph.AddNode("sc1", "cb2")

	return callGraph
}

// CreateGraphTestTwoAsyncCallsFirstFailCrossLocal -
func CreateGraphTestTwoAsyncCallsFirstFailCrossLocal() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 1000, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncCrossShardEdge(sc1f1, sc2f2, "cb1", "").
		SetGasLimit(20).
		SetGasUsed(7).
		SetGasUsedByCallback(5).
		SetFail()

	s3f3 := callGraph.AddNode("s3", "f3")
	callGraph.AddAsyncEdge(sc1f1, s3f3, "", "").
		SetGasLimit(30).
		SetGasUsed(6)

	callGraph.AddNode("sc1", "cb1")
	callGraph.AddNode("sc1", "cb2")

	return callGraph
}

// CreateGraphTestTwoAsyncCallsSecondFailCrossLocal -
func CreateGraphTestTwoAsyncCallsSecondFailCrossLocal() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 1000, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncCrossShardEdge(sc1f1, sc2f2, "cb1", "").
		SetGasLimit(20).
		SetGasUsed(7).
		SetGasUsedByCallback(5)

	s3f3 := callGraph.AddNode("s3", "f3")
	callGraph.AddAsyncEdge(sc1f1, s3f3, "cb2", "").
		SetGasLimit(30).
		SetGasUsed(6).
		SetGasUsedByCallback(3).
		SetFail()

	callGraph.AddNode("sc1", "cb1")
	callGraph.AddNode("sc1", "cb2")

	return callGraph
}

// CreateGraphTestTwoAsyncCallsBothFailCrossLocal -
func CreateGraphTestTwoAsyncCallsBothFailCrossLocal() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 1000, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncCrossShardEdge(sc1f1, sc2f2, "cb1", "").
		SetGasLimit(20).
		SetGasUsed(7).
		SetGasUsedByCallback(5).
		SetFail()

	s3f3 := callGraph.AddNode("s3", "f3")
	callGraph.AddAsyncEdge(sc1f1, s3f3, "cb2", "").
		SetGasLimit(30).
		SetGasUsed(6).
		SetGasUsedByCallback(3).
		SetFail()

	callGraph.AddNode("sc1", "cb1")
	callGraph.AddNode("sc1", "cb2")

	return callGraph
}

// CreateGraphTestTwoAsyncCallsFirstCallbackFailCrossLocal -
func CreateGraphTestTwoAsyncCallsFirstCallbackFailCrossLocal() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 1000, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncCrossShardEdge(sc1f1, sc2f2, "cb1", "").
		SetGasLimit(20).
		SetGasUsed(7).
		SetGasUsedByCallback(5).
		SetCallbackFail()

	s3f3 := callGraph.AddNode("s3", "f3")
	callGraph.AddAsyncEdge(sc1f1, s3f3, "cb2", "").
		SetGasLimit(30).
		SetGasUsed(6).
		SetGasUsedByCallback(3)

	callGraph.AddNode("sc1", "cb1")
	callGraph.AddNode("sc1", "cb2")

	return callGraph
}

// CreateGraphTestTwoAsyncCallsSecondCallbackFailCrossLocal -
func CreateGraphTestTwoAsyncCallsSecondCallbackFailCrossLocal() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 1000, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncCrossShardEdge(sc1f1, sc2f2, "cb1", "").
		SetGasLimit(20).
		SetGasUsed(7).
		SetGasUsedByCallback(5)

	s3f3 := callGraph.AddNode("s3", "f3")
	callGraph.AddAsyncEdge(sc1f1, s3f3, "cb2", "").
		SetGasLimit(30).
		SetGasUsed(6).
		SetGasUsedByCallback(3).
		SetCallbackFail()

	callGraph.AddNode("sc1", "cb1")
	callGraph.AddNode("sc1", "cb2")

	return callGraph
}

// CreateGraphTestTwoAsyncCallsBothCallbacksFailCrossLocal -
func CreateGraphTestTwoAsyncCallsBothCallbacksFailCrossLocal() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 1000, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncCrossShardEdge(sc1f1, sc2f2, "cb1", "").
		SetGasLimit(20).
		SetGasUsed(7).
		SetGasUsedByCallback(5).
		SetCallbackFail()

	s3f3 := callGraph.AddNode("s3", "f3")
	callGraph.AddAsyncEdge(sc1f1, s3f3, "cb2", "").
		SetGasLimit(30).
		SetGasUsed(6).
		SetGasUsedByCallback(3).
		SetCallbackFail()

	callGraph.AddNode("sc1", "cb1")
	callGraph.AddNode("sc1", "cb2")

	return callGraph
}

// CreateGraphTestTwoAsyncCallsCrossShard -
func CreateGraphTestTwoAsyncCallsCrossShard() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 1000, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncCrossShardEdge(sc1f1, sc2f2, "cb1", "").
		SetGasLimit(20).
		SetGasUsed(7).
		SetGasUsedByCallback(5)

	s3f3 := callGraph.AddNode("s3", "f3")
	callGraph.AddAsyncCrossShardEdge(sc1f1, s3f3, "cb2", "").
		SetGasLimit(30).
		SetGasUsed(6).
		SetGasUsedByCallback(3)

	callGraph.AddNode("sc1", "cb1")
	callGraph.AddNode("sc1", "cb2")

	return callGraph
}

// CreateGraphTestTwoAsyncCallsFirstNoCallbackCrossShard -
func CreateGraphTestTwoAsyncCallsFirstNoCallbackCrossShard() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 1000, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncCrossShardEdge(sc1f1, sc2f2, "", "").
		SetGasLimit(20).
		SetGasUsed(7)

	s3f3 := callGraph.AddNode("s3", "f3")
	callGraph.AddAsyncCrossShardEdge(sc1f1, s3f3, "cb2", "").
		SetGasLimit(30).
		SetGasUsed(6).
		SetGasUsedByCallback(3)

	callGraph.AddNode("sc1", "cb1")
	callGraph.AddNode("sc1", "cb2")

	return callGraph
}

// CreateGraphTestTwoAsyncCallsSecondNoCallbackCrossShard -
func CreateGraphTestTwoAsyncCallsSecondNoCallbackCrossShard() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 1000, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncCrossShardEdge(sc1f1, sc2f2, "cb1", "").
		SetGasLimit(20).
		SetGasUsed(7).
		SetGasUsedByCallback(5)

	s3f3 := callGraph.AddNode("s3", "f3")
	callGraph.AddAsyncCrossShardEdge(sc1f1, s3f3, "", "").
		SetGasLimit(30).
		SetGasUsed(6)

	callGraph.AddNode("sc1", "cb1")
	callGraph.AddNode("sc1", "cb2")

	return callGraph
}

// CreateGraphTestTwoAsyncCallsFirstFailCrossShard -
func CreateGraphTestTwoAsyncCallsFirstFailCrossShard() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 1000, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncCrossShardEdge(sc1f1, sc2f2, "cb1", "").
		SetGasLimit(20).
		SetGasUsed(7).
		SetGasUsedByCallback(5).
		SetFail()

	s3f3 := callGraph.AddNode("s3", "f3")
	callGraph.AddAsyncCrossShardEdge(sc1f1, s3f3, "cb2", "").
		SetGasLimit(30).
		SetGasUsed(6).
		SetGasUsedByCallback(3)

	callGraph.AddNode("sc1", "cb1")
	callGraph.AddNode("sc1", "cb2")

	return callGraph
}

// CreateGraphTestTwoAsyncCallsSecondFailCrossShard -
func CreateGraphTestTwoAsyncCallsSecondFailCrossShard() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 1000, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncCrossShardEdge(sc1f1, sc2f2, "cb1", "").
		SetGasLimit(20).
		SetGasUsed(7).
		SetGasUsedByCallback(5)

	s3f3 := callGraph.AddNode("s3", "f3")
	callGraph.AddAsyncCrossShardEdge(sc1f1, s3f3, "cb2", "").
		SetGasLimit(30).
		SetGasUsed(6).
		SetGasUsedByCallback(3).
		SetFail()

	callGraph.AddNode("sc1", "cb1")
	callGraph.AddNode("sc1", "cb2")

	return callGraph
}

// CreateGraphTestTwoAsyncCallsBothFailCrossShard -
func CreateGraphTestTwoAsyncCallsBothFailCrossShard() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 1000, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncCrossShardEdge(sc1f1, sc2f2, "cb1", "").
		SetGasLimit(20).
		SetGasUsed(7).
		SetGasUsedByCallback(5).
		SetFail()

	s3f3 := callGraph.AddNode("s3", "f3")
	callGraph.AddAsyncCrossShardEdge(sc1f1, s3f3, "cb2", "").
		SetGasLimit(30).
		SetGasUsed(6).
		SetGasUsedByCallback(3).
		SetFail()

	callGraph.AddNode("sc1", "cb1")
	callGraph.AddNode("sc1", "cb2")

	return callGraph
}

// CreateGraphTestTwoAsyncCallsFirstCallbackFailCrossShard -
func CreateGraphTestTwoAsyncCallsFirstCallbackFailCrossShard() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 1000, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncCrossShardEdge(sc1f1, sc2f2, "cb1", "").
		SetGasLimit(20).
		SetGasUsed(7).
		SetGasUsedByCallback(5).
		SetCallbackFail()

	s3f3 := callGraph.AddNode("s3", "f3")
	callGraph.AddAsyncCrossShardEdge(sc1f1, s3f3, "cb2", "").
		SetGasLimit(30).
		SetGasUsed(6).
		SetGasUsedByCallback(3)

	callGraph.AddNode("sc1", "cb1")
	callGraph.AddNode("sc1", "cb2")

	return callGraph
}

// CreateGraphTestTwoAsyncCallsSecondCallbackFailCrossShard -
func CreateGraphTestTwoAsyncCallsSecondCallbackFailCrossShard() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 1000, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncCrossShardEdge(sc1f1, sc2f2, "cb1", "").
		SetGasLimit(20).
		SetGasUsed(7).
		SetGasUsedByCallback(5)

	s3f3 := callGraph.AddNode("s3", "f3")
	callGraph.AddAsyncCrossShardEdge(sc1f1, s3f3, "cb2", "").
		SetGasLimit(30).
		SetGasUsed(6).
		SetGasUsedByCallback(3).
		SetCallbackFail()

	callGraph.AddNode("sc1", "cb1")
	callGraph.AddNode("sc1", "cb2")

	return callGraph
}

// CreateGraphTestTwoAsyncCallsBothCallbacksFailCrossShard -
func CreateGraphTestTwoAsyncCallsBothCallbacksFailCrossShard() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 1000, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncCrossShardEdge(sc1f1, sc2f2, "cb1", "").
		SetGasLimit(20).
		SetGasUsed(7).
		SetGasUsedByCallback(5).
		SetCallbackFail()

	s3f3 := callGraph.AddNode("s3", "f3")
	callGraph.AddAsyncCrossShardEdge(sc1f1, s3f3, "cb2", "").
		SetGasLimit(30).
		SetGasUsed(6).
		SetGasUsedByCallback(3).
		SetCallbackFail()

	callGraph.AddNode("sc1", "cb1")
	callGraph.AddNode("sc1", "cb2")

	return callGraph
}

// CreateGraphTestSyncAndAsync1 -
func CreateGraphTestSyncAndAsync1() *TestCallGraph {
	callGraph := CreateTestCallGraph()
	sc1f1 := callGraph.AddStartNode("sc1", "f1", 2000, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncEdge(sc1f1, sc2f2, "cb1", "").
		SetGasLimit(400).
		SetGasUsed(60).
		SetGasUsedByCallback(100)

	callGraph.AddNode("sc1", "cb1")

	sc3f3 := callGraph.AddNode("sc3", "f3")
	callGraph.AddSyncEdge(sc2f2, sc3f3).
		SetGasLimit(200).
		SetGasUsed(10)

	return callGraph
}

// CreateGraphTestSyncAndAsync2 -
func CreateGraphTestSyncAndAsync2() *TestCallGraph {
	callGraph := CreateTestCallGraph()
	sc1f1 := callGraph.AddStartNode("sc1", "f1", 2000, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddSyncEdge(sc1f1, sc2f2).
		SetGasLimit(700).
		SetGasUsed(70)

	sc3f3 := callGraph.AddNode("sc3", "f3")
	callGraph.AddAsyncEdge(sc2f2, sc3f3, "cb1", "").
		SetGasLimit(400).
		SetGasUsed(60).
		SetGasUsedByCallback(100)

	sc4f4 := callGraph.AddNode("sc4", "f4")
	callGraph.AddSyncEdge(sc1f1, sc4f4).
		SetGasLimit(200).
		SetGasUsed(10)

	sc2cb1 := callGraph.AddNode("sc2", "cb1")
	callGraph.AddSyncEdge(sc4f4, sc2cb1).
		SetGasLimit(80).
		SetGasUsed(50)

	sc5f5 := callGraph.AddNode("sc5", "f5")
	callGraph.AddSyncEdge(sc2cb1, sc5f5).
		SetGasLimit(20).
		SetGasUsed(10)

	return callGraph
}

// CreateGraphTestSyncAndAsync3 -
func CreateGraphTestSyncAndAsync3() *TestCallGraph {
	callGraph := CreateTestCallGraph()
	sc1f1 := callGraph.AddStartNode("sc1", "f1", 1000, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncEdge(sc1f1, sc2f2, "cb1", "").
		SetGasLimit(55).
		SetGasUsed(7).
		SetGasUsedByCallback(6)

	sc5f5 := callGraph.AddNode("sc5", "f5")
	callGraph.AddSyncEdge(sc2f2, sc5f5).
		SetGasLimit(10).
		SetGasUsed(4)

	sc3f3 := callGraph.AddNode("sc3", "f3")

	sc1cb1 := callGraph.AddNode("sc1", "cb1")
	callGraph.AddSyncEdge(sc1cb1, sc3f3).
		SetGasLimit(20).
		SetGasUsed(3)

	sc4f4 := callGraph.AddNode("sc4", "f4")
	callGraph.AddSyncEdge(sc3f3, sc4f4).
		SetGasLimit(10).
		SetGasUsed(5)

	return callGraph
}

// CreateGraphTestSyncAndAsync4 -
func CreateGraphTestSyncAndAsync4() *TestCallGraph {
	callGraph := CreateTestCallGraph()
	sc1f1 := callGraph.AddStartNode("sc1", "f1", 2000, 10)

	sc4f4 := callGraph.AddNode("sc4", "f4")
	callGraph.AddSyncEdge(sc1f1, sc4f4).
		SetGasLimit(400).
		SetGasUsed(40)

	sc5f5 := callGraph.AddNode("sc5", "f5")
	callGraph.AddAsyncEdge(sc4f4, sc5f5, "cb2", "").
		SetGasLimit(200).
		SetGasUsed(20).
		SetGasUsedByCallback(100)

	callGraph.AddNode("sc4", "cb2")

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncEdge(sc1f1, sc2f2, "cb1", "").
		SetGasLimit(400).
		SetGasUsed(60).
		SetGasUsedByCallback(100)

	sc1cb1 := callGraph.AddNode("sc1", "cb1")

	sc3f3 := callGraph.AddNode("sc3", "f3")
	callGraph.AddSyncEdge(sc1cb1, sc3f3).
		SetGasLimit(10).
		SetGasUsed(5)

	return callGraph
}

// CreateGraphTestSyncAndAsync5 -
func CreateGraphTestSyncAndAsync5() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 1000, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncEdge(sc1f1, sc2f2, "cb1", "").
		SetGasLimit(800).
		SetGasUsed(50).
		SetGasUsedByCallback(20)

	sc3f3 := callGraph.AddNode("sc3", "f3")
	callGraph.AddSyncEdge(sc2f2, sc3f3).
		SetGasLimit(500).
		SetGasUsed(20)

	sc4f4 := callGraph.AddNode("sc4", "f4")
	callGraph.AddSyncEdge(sc3f3, sc4f4).
		SetGasLimit(300).
		SetGasUsed(15)

	sc5f5 := callGraph.AddNode("sc5", "f5")
	callGraph.AddNode("sc2", "cb2")
	callGraph.AddAsyncEdge(sc4f4, sc5f5, "cb4", "").
		SetGasLimit(100).
		SetGasUsed(50).
		SetGasUsedByCallback(5)

	callGraph.AddNode("sc1", "cb1")
	callGraph.AddNode("sc4", "cb4")

	return callGraph
}

// CreateGraphTestSyncAndAsync6 -
func CreateGraphTestSyncAndAsync6() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc0f0 := callGraph.AddStartNode("sc0", "f0", 3000, 10)

	sc1f1 := callGraph.AddNode("sc1", "f1")

	callGraph.AddAsyncEdge(sc0f0, sc1f1, "cb0", "").
		SetGasLimit(1300).
		SetGasUsed(60).
		SetGasUsedByCallback(10)

	callGraph.AddNode("sc0", "cb0")

	sc6f6 := callGraph.AddNode("sc6", "f6")
	callGraph.AddSyncEdge(sc1f1, sc6f6).
		SetGasLimit(1000).
		SetGasUsed(5)

	return callGraph
}

// CreateGraphTestSyncAndAsync7 -
func CreateGraphTestSyncAndAsync7() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc0f0 := callGraph.AddStartNode("sc0", "f0", 3000, 10)

	sc1f1 := callGraph.AddNode("sc1", "f1")
	callGraph.AddSyncEdge(sc0f0, sc1f1).
		SetGasLimit(1000).
		SetGasUsed(100)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddSyncEdge(sc1f1, sc2f2).
		SetGasLimit(500).
		SetGasUsed(40)

	sc3f3 := callGraph.AddNode("sc3", "f3")
	callGraph.AddSyncEdge(sc1f1, sc3f3).
		SetGasLimit(100).
		SetGasUsed(10)

	sc4f4 := callGraph.AddNode("sc4", "f4")
	callGraph.AddAsyncEdge(sc2f2, sc4f4, "cb2", "").
		SetGasLimit(300).
		SetGasUsed(60).
		SetGasUsedByCallback(10)

	callGraph.AddNode("sc2", "cb2")

	return callGraph
}

// CreateGraphTestDifferentTypeOfCallsToSameFunction -
func CreateGraphTestDifferentTypeOfCallsToSameFunction() *TestCallGraph {
	callGraph := CreateTestCallGraph()
	sc1f1 := callGraph.AddStartNode("sc1", "f1", 2000, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddSyncEdge(sc1f1, sc2f2).
		SetGasLimit(20).
		SetGasUsed(5)

	callGraph.AddAsyncEdge(sc1f1, sc2f2, "cb1", "").
		SetGasLimit(35).
		SetGasUsed(7).
		SetGasUsedByCallback(5)

	callGraph.AddAsyncEdge(sc1f1, sc2f2, "cb2", "").
		SetGasLimit(30).
		SetGasUsed(6).
		SetGasUsedByCallback(3)

	sc3f3 := callGraph.AddNode("sc3", "f3")
	callGraph.AddSyncEdge(sc2f2, sc3f3).
		SetGasLimit(6).
		SetGasUsed(6)

	callGraph.AddNode("sc1", "cb1")
	callGraph.AddNode("sc1", "cb2")

	return callGraph
}

// CreateGraphTestSameContractWithDifferentSubCallsLocalLocal -
func CreateGraphTestSameContractWithDifferentSubCallsLocalLocal() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 3000, 10)

	sc4f4 := callGraph.AddNode("sc4", "f4")
	callGraph.AddSyncEdge(sc1f1, sc4f4).
		SetGasLimit(300).
		SetGasUsed(15)

	sc5f5 := callGraph.AddNode("sc5", "f5")
	callGraph.AddNode("sc2", "cb2")
	callGraph.AddAsyncEdge(sc4f4, sc5f5, "cb4", "").
		SetGasLimit(100).
		SetGasUsed(50).
		SetGasUsedByCallback(10)

	callGraph.AddNode("sc1", "cb1")
	callGraph.AddNode("sc4", "cb4")

	sc4f7 := callGraph.AddNode("sc4", "f7")
	callGraph.AddSyncEdge(sc1f1, sc4f7).
		SetGasLimit(300).
		SetGasUsed(15)

	sc5f8 := callGraph.AddNode("sc5", "f8")
	callGraph.AddNode("sc2", "cb2")
	callGraph.AddAsyncEdge(sc4f7, sc5f8, "cb5", "").
		SetGasLimit(100).
		SetGasUsed(50).
		SetGasUsedByCallback(10)

	callGraph.AddNode("sc4", "cb5")

	return callGraph
}

// CreateGraphTestSameContractWithDifferentSubCallsLocalCross -
func CreateGraphTestSameContractWithDifferentSubCallsLocalCross() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 3000, 10)

	sc4f4 := callGraph.AddNode("sc4", "f4")
	callGraph.AddSyncEdge(sc1f1, sc4f4).
		SetGasLimit(300).
		SetGasUsed(15)

	sc5f5 := callGraph.AddNode("sc5", "f5")
	callGraph.AddNode("sc2", "cb2")
	callGraph.AddAsyncEdge(sc4f4, sc5f5, "cb4", "").
		SetGasLimit(100).
		SetGasUsed(50).
		SetGasUsedByCallback(10)

	callGraph.AddNode("sc1", "cb1")
	callGraph.AddNode("sc4", "cb4")

	sc4f7 := callGraph.AddNode("sc4", "f7")
	callGraph.AddSyncEdge(sc1f1, sc4f7).
		SetGasLimit(300).
		SetGasUsed(15)

	sc6f8 := callGraph.AddNode("sc6", "f8")
	callGraph.AddNode("sc2", "cb2")
	callGraph.AddAsyncCrossShardEdge(sc4f7, sc6f8, "cb5", "").
		SetGasLimit(100).
		SetGasUsed(50).
		SetGasUsedByCallback(10)

	callGraph.AddNode("sc4", "cb5")

	return callGraph
}

// CreateGraphTestSameContractWithDifferentSubCallsCrossLocal -
func CreateGraphTestSameContractWithDifferentSubCallsCrossLocal() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 3000, 10)

	sc4f4 := callGraph.AddNode("sc4", "f4")
	callGraph.AddSyncEdge(sc1f1, sc4f4).
		SetGasLimit(300).
		SetGasUsed(15)

	sc5f5 := callGraph.AddNode("sc5", "f5")
	callGraph.AddNode("sc2", "cb2")
	callGraph.AddAsyncCrossShardEdge(sc4f4, sc5f5, "cb4", "").
		SetGasLimit(100).
		SetGasUsed(50).
		SetGasUsedByCallback(10)

	callGraph.AddNode("sc1", "cb1")
	callGraph.AddNode("sc4", "cb4")

	sc4f7 := callGraph.AddNode("sc4", "f7")
	callGraph.AddSyncEdge(sc1f1, sc4f7).
		SetGasLimit(300).
		SetGasUsed(15)

	sc6f8 := callGraph.AddNode("sc6", "f8")
	callGraph.AddNode("sc2", "cb2")
	callGraph.AddAsyncEdge(sc4f7, sc6f8, "cb5", "").
		SetGasLimit(100).
		SetGasUsed(50).
		SetGasUsedByCallback(10)

	callGraph.AddNode("sc4", "cb5")

	return callGraph
}

// CreateGraphTestSameContractWithDifferentSubCallsCrossCross -
func CreateGraphTestSameContractWithDifferentSubCallsCrossCross() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 3000, 10)

	sc4f4 := callGraph.AddNode("sc4", "f4")
	callGraph.AddSyncEdge(sc1f1, sc4f4).
		SetGasLimit(300).
		SetGasUsed(15)

	sc5f5 := callGraph.AddNode("sc5", "f5")
	callGraph.AddNode("sc2", "cb2")
	callGraph.AddAsyncCrossShardEdge(sc4f4, sc5f5, "cb4", "").
		SetGasLimit(100).
		SetGasUsed(50).
		SetGasUsedByCallback(10)

	callGraph.AddNode("sc1", "cb1")
	callGraph.AddNode("sc4", "cb4")

	sc4f7 := callGraph.AddNode("sc4", "f7")
	callGraph.AddSyncEdge(sc1f1, sc4f7).
		SetGasLimit(300).
		SetGasUsed(15)

	sc5f8 := callGraph.AddNode("sc5", "f8")
	callGraph.AddNode("sc2", "cb2")
	callGraph.AddAsyncCrossShardEdge(sc4f7, sc5f8, "cb5", "").
		SetGasLimit(100).
		SetGasUsed(50).
		SetGasUsedByCallback(10)

	callGraph.AddNode("sc4", "cb5")

	return callGraph
}

// CreateGraphTestSyncAndAsync9 -
func CreateGraphTestSyncAndAsync9() *TestCallGraph {
	callGraph := CreateTestCallGraph()
	sc1f1 := callGraph.AddStartNode("sc1", "f1", 5000, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddSyncEdge(sc1f1, sc2f2).
		SetGasLimit(20).
		SetGasUsed(5)

	callGraph.AddAsyncEdge(sc1f1, sc2f2, "cb1", "").
		SetGasLimit(500).
		SetGasUsed(7).
		SetGasUsedByCallback(5)

	callGraph.AddAsyncEdge(sc1f1, sc2f2, "cb2", "").
		SetGasLimit(600).
		SetGasUsed(6).
		SetGasUsedByCallback(400)

	sc3f3 := callGraph.AddNode("sc3", "f3")
	callGraph.AddSyncEdge(sc2f2, sc3f3).
		SetGasLimit(6).
		SetGasUsed(6)

	sc4f4 := callGraph.AddNode("sc4", "f4")
	sc1cb1 := callGraph.AddNode("sc1", "cb1")
	callGraph.AddSyncEdge(sc1cb1, sc4f4).
		SetGasLimit(10).
		SetGasUsed(5)

	sc1cb2 := callGraph.AddNode("sc1", "cb2")
	sc5f5 := callGraph.AddNode("sc5", "f5")

	callGraph.AddAsyncEdge(sc1cb2, sc5f5, "cb1", "").
		SetGasLimit(20).
		SetGasUsed(4).
		SetGasUsedByCallback(3)

	return callGraph
}

// CreateGraphTestSyncAndAsync10 -
func CreateGraphTestSyncAndAsync10() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 5500, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddSyncEdge(sc1f1, sc2f2).
		SetGasLimit(1500).
		SetGasUsed(20)

	callGraph.AddSyncEdge(sc1f1, sc2f2).
		SetGasLimit(1500).
		SetGasUsed(20)

	callGraph.AddNode("sc2", "cb1")

	sc3f3 := callGraph.AddNode("sc3", "f3")
	callGraph.AddAsyncEdge(sc2f2, sc3f3, "cb1", "").
		SetGasLimit(500).
		SetGasUsed(50).
		SetGasUsedByCallback(35)

	sc4f4 := callGraph.AddNode("sc4", "f4")
	callGraph.AddAsyncEdge(sc2f2, sc4f4, "cb1", "").
		SetGasLimit(500).
		SetGasUsed(50).
		SetGasUsedByCallback(35)

	return callGraph
}

// CreateGraphTestSyncAndAsync11 -
func CreateGraphTestSyncAndAsync11() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 1000, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddSyncEdge(sc1f1, sc2f2).
		SetGasLimit(500).
		SetGasUsed(7)

	sc3f4 := callGraph.AddNode("sc3", "f4")
	callGraph.AddSyncEdge(sc2f2, sc3f4).
		SetGasLimit(10).
		SetGasUsed(3)

	sc4f4 := callGraph.AddNode("sc4", "f4")
	callGraph.AddAsyncEdge(sc1f1, sc4f4, "cb1", "").
		SetGasLimit(40).
		SetGasUsed(12).
		SetGasUsedByCallback(5)

	callGraph.AddNode("sc1", "cb1")

	sc5f3 := callGraph.AddNode("sc5", "f3")
	callGraph.AddAsyncCrossShardEdge(sc2f2, sc5f3, "cb2", "").
		SetGasLimit(100).
		SetGasUsed(6).
		SetGasUsedByCallback(3)

	callGraph.AddNode("sc2", "cb2")

	return callGraph
}

// CreateGraphTestOneAsyncCallCrossShard -
func CreateGraphTestOneAsyncCallCrossShard() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 500, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")

	callGraph.AddAsyncCrossShardEdge(sc1f1, sc2f2, "cb1", "").
		SetGasLimit(35).
		SetGasUsed(7).
		SetGasUsedByCallback(6)

	callGraph.AddNode("sc1", "cb1")

	return callGraph
}

// CreateGraphTestOneAsyncCallNoCallbackCrossShard -
func CreateGraphTestOneAsyncCallNoCallbackCrossShard() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 500, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")

	callGraph.AddAsyncCrossShardEdge(sc1f1, sc2f2, "", "").
		SetGasLimit(35).
		SetGasUsed(7)

	callGraph.AddNode("sc1", "cb1")

	return callGraph
}

// CreateGraphTestOneAsyncCallCrossShard2 -
func CreateGraphTestOneAsyncCallCrossShard2() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 500, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddSyncEdge(sc1f1, sc2f2).
		SetGasLimit(300).
		SetGasUsed(20)

	sc3f3 := callGraph.AddNode("sc3", "f3")

	callGraph.AddAsyncCrossShardEdge(sc2f2, sc3f3, "cb1", "").
		SetGasLimit(35).
		SetGasUsed(7).
		SetGasUsedByCallback(6)

	callGraph.AddNode("sc2", "cb1")

	return callGraph
}

// CreateGraphTestOneAsyncCallFailCrossShard -
func CreateGraphTestOneAsyncCallFailCrossShard() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 500, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")

	callGraph.AddAsyncCrossShardEdge(sc1f1, sc2f2, "cb1", "").
		SetGasLimit(35).
		SetGasUsed(7).
		SetGasUsedByCallback(6).
		SetFail()

	callGraph.AddNode("sc1", "cb1")

	return callGraph
}

// CreateGraphTestOneAsyncCallFailNoCallbackCrossShard -
func CreateGraphTestOneAsyncCallFailNoCallbackCrossShard() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 500, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")

	callGraph.AddAsyncCrossShardEdge(sc1f1, sc2f2, "", "").
		SetGasLimit(35).
		SetGasUsed(7).
		SetFail()

	callGraph.AddNode("sc1", "cb1")

	return callGraph
}

// CreateGraphTestAsyncCallsCrossShard2 -
func CreateGraphTestAsyncCallsCrossShard2() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 800, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncCrossShardEdge(sc1f1, sc2f2, "cb1", "").
		SetGasLimit(220).
		SetGasUsed(27).
		SetGasUsedByCallback(23)

	sc3f6 := callGraph.AddNode("sc3", "f6")
	callGraph.AddNode("sc2", "cb2")
	callGraph.AddAsyncCrossShardEdge(sc2f2, sc3f6, "cb2", "").
		SetGasLimit(30).
		SetGasUsed(10).
		SetGasUsedByCallback(6)

	callGraph.AddNode("sc1", "cb1")

	return callGraph
}

// CreateGraphTestAsyncCallsCrossShard3 -
func CreateGraphTestAsyncCallsCrossShard3() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 800, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncEdge(sc1f1, sc2f2, "cb1", "").
		SetGasLimit(220).
		SetGasUsed(27).
		SetGasUsedByCallback(23)

	sc3f6 := callGraph.AddNode("sc3", "f6")
	callGraph.AddNode("sc2", "cb2")
	callGraph.AddAsyncCrossShardEdge(sc2f2, sc3f6, "cb2", "").
		SetGasLimit(30).
		SetGasUsed(10).
		SetGasUsedByCallback(6)

	callGraph.AddNode("sc1", "cb1")

	return callGraph
}

// CreateGraphTestAsyncCallsCrossShard4 -
func CreateGraphTestAsyncCallsCrossShard4() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 1000, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	//callGraph.AddAsyncCrossShardEdge(sc1f1, sc2f2, "cb1", "").
	callGraph.AddAsyncEdge(sc1f1, sc2f2, "cb1", "").
		SetGasLimit(800).
		SetGasUsed(50).
		SetGasUsedByCallback(20)

	sc3f3 := callGraph.AddNode("sc3", "f3")
	callGraph.AddSyncEdge(sc2f2, sc3f3).
		SetGasLimit(500).
		SetGasUsed(20)

	sc4f4 := callGraph.AddNode("sc4", "f4")
	callGraph.AddSyncEdge(sc3f3, sc4f4).
		SetGasLimit(300).
		SetGasUsed(15)

	sc5f5 := callGraph.AddNode("sc5", "f5")
	callGraph.AddNode("sc2", "cb2")
	callGraph.AddAsyncCrossShardEdge(sc4f4, sc5f5, "cb4", "").
		SetGasLimit(100).
		SetGasUsed(50).
		SetGasUsedByCallback(5)

	callGraph.AddNode("sc1", "cb1")
	callGraph.AddNode("sc4", "cb4")

	return callGraph
}

// CreateGraphTestAsyncCallsCrossShard5 -
func CreateGraphTestAsyncCallsCrossShard5() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc1f1 := callGraph.AddStartNode("sc1", "f1", 3000, 10)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncEdge(sc1f1, sc2f2, "cb1", "").
		SetGasLimit(2000).
		SetGasUsed(50).
		SetGasUsedByCallback(20)

	sc3f3 := callGraph.AddNode("sc3", "f3")
	callGraph.AddSyncEdge(sc2f2, sc3f3).
		SetGasLimit(500).
		SetGasUsed(20)

	sc4f4 := callGraph.AddNode("sc4", "f4")
	callGraph.AddSyncEdge(sc3f3, sc4f4).
		SetGasLimit(300).
		SetGasUsed(15)

	sc5f5 := callGraph.AddNode("sc5", "f5")
	callGraph.AddNode("sc2", "cb2")
	callGraph.AddAsyncCrossShardEdge(sc4f4, sc5f5, "cb4", "").
		SetGasLimit(100).
		SetGasUsed(50).
		SetGasUsedByCallback(10)

	callGraph.AddNode("sc1", "cb1")
	callGraph.AddNode("sc4", "cb4")

	sc3f6 := callGraph.AddNode("sc3", "f6")
	callGraph.AddSyncEdge(sc2f2, sc3f6).
		SetGasLimit(500).
		SetGasUsed(20)

	sc4f7 := callGraph.AddNode("sc4", "f7")
	callGraph.AddSyncEdge(sc3f6, sc4f7).
		SetGasLimit(300).
		SetGasUsed(15)

	sc5f8 := callGraph.AddNode("sc5", "f8")
	callGraph.AddNode("sc2", "cb2")
	callGraph.AddAsyncCrossShardEdge(sc4f7, sc5f8, "cb5", "").
		SetGasLimit(100).
		SetGasUsed(50).
		SetGasUsedByCallback(10)

	callGraph.AddNode("sc4", "cb5")

	return callGraph
}

// CreateGraphTestAsyncCallsCrossShard6 -
func CreateGraphTestAsyncCallsCrossShard6() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc0f0 := callGraph.AddStartNode("sc0", "f0", 3000, 10)

	sc1f1 := callGraph.AddNode("sc1", "f1")

	callGraph.AddAsyncEdge(sc0f0, sc1f1, "cb0", "").
		SetGasLimit(1300).
		SetGasUsed(60).
		SetGasUsedByCallback(10)

	callGraph.AddNode("sc0", "cb0")

	sc6f6 := callGraph.AddNode("sc6", "f6")
	callGraph.AddSyncEdge(sc1f1, sc6f6).
		SetGasLimit(1000).
		SetGasUsed(5)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncEdge(sc6f6, sc2f2, "cb1", "").
		SetGasLimit(800).
		SetGasUsed(50).
		SetGasUsedByCallback(20)

	callGraph.AddNode("sc6", "cb1")

	sc3f3 := callGraph.AddNode("sc3", "f3")
	callGraph.AddSyncEdge(sc2f2, sc3f3).
		SetGasLimit(500).
		SetGasUsed(20)

	sc4f4 := callGraph.AddNode("sc4", "f4")
	callGraph.AddSyncEdge(sc3f3, sc4f4).
		SetGasLimit(300).
		SetGasUsed(15)

	sc5f5 := callGraph.AddNode("sc5", "f5")
	callGraph.AddNode("sc2", "cb2")
	callGraph.AddAsyncCrossShardEdge(sc4f4, sc5f5, "cb4", "").
		SetGasLimit(100).
		SetGasUsed(50).
		SetGasUsedByCallback(5)

	callGraph.AddNode("sc4", "cb4")

	return callGraph
}

// CreateGraphTestAsyncCallsCrossShard7 -
func CreateGraphTestAsyncCallsCrossShard7() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc0f0 := callGraph.AddStartNode("sc0", "f0", 3000, 10)

	sc1f1 := callGraph.AddNode("sc1", "f1")

	callGraph.AddAsyncEdge(sc0f0, sc1f1, "cb0", "").
		SetGasLimit(1300).
		SetGasUsed(60).
		SetGasUsedByCallback(10)

	callGraph.AddNode("sc0", "cb0")

	sc6f6 := callGraph.AddNode("sc6", "f6")
	callGraph.AddSyncEdge(sc1f1, sc6f6).
		SetGasLimit(1000).
		SetGasUsed(5)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncEdge(sc6f6, sc2f2, "cb1", "").
		SetGasLimit(800).
		SetGasUsed(50).
		SetGasUsedByCallback(20)

	callGraph.AddNode("sc6", "cb1")

	sc3f3 := callGraph.AddNode("sc3", "f3")
	callGraph.AddSyncEdge(sc2f2, sc3f3).
		SetGasLimit(500).
		SetGasUsed(20)

	sc4f4 := callGraph.AddNode("sc4", "f4")
	callGraph.AddSyncEdge(sc3f3, sc4f4).
		SetGasLimit(300).
		SetGasUsed(15)

	sc5f5 := callGraph.AddNode("sc5", "f5")
	callGraph.AddNode("sc2", "cb2")
	callGraph.AddAsyncEdge(sc4f4, sc5f5, "cb4", "").
		SetGasLimit(100).
		SetGasUsed(50).
		SetGasUsedByCallback(5)

	callGraph.AddNode("sc4", "cb4")

	return callGraph
}

// CreateGraphTestAsyncCallsCrossShard8 -
func CreateGraphTestAsyncCallsCrossShard8() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc0f0 := callGraph.AddStartNode("sc0", "f0", 3000, 10)

	sc1f1 := callGraph.AddNode("sc1", "f1")

	callGraph.AddAsyncEdge(sc0f0, sc1f1, "cb0", "").
		SetGasLimit(1300).
		SetGasUsed(60).
		SetGasUsedByCallback(10)

	callGraph.AddNode("sc0", "cb0")

	sc6f6 := callGraph.AddNode("sc6", "f6")
	callGraph.AddSyncEdge(sc1f1, sc6f6).
		SetGasLimit(1000).
		SetGasUsed(5)

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddAsyncEdge(sc6f6, sc2f2, "cb1", "").
		SetGasLimit(800).
		SetGasUsed(50).
		SetGasUsedByCallback(20)

	callGraph.AddNode("sc6", "cb1")

	sc3f3 := callGraph.AddNode("sc3", "f3")
	callGraph.AddSyncEdge(sc2f2, sc3f3).
		SetGasLimit(600).
		SetGasUsed(20)

	sc4f4 := callGraph.AddNode("sc4", "f4")
	callGraph.AddSyncEdge(sc3f3, sc4f4).
		SetGasLimit(500).
		SetGasUsed(15)

	callGraph.AddNode("sc2", "cb2")

	sc5f5 := callGraph.AddNode("sc5", "f5")
	callGraph.AddAsyncCrossShardEdge(sc4f4, sc5f5, "cb4", "").
		SetGasLimit(70).
		SetGasUsed(50).
		SetGasUsedByCallback(5)

	callGraph.AddNode("sc4", "cb4")

	sc7f7 := callGraph.AddNode("sc7", "f7")
	callGraph.AddAsyncCrossShardEdge(sc4f4, sc7f7, "cb7", "").
		SetGasLimit(30).
		SetGasUsed(20).
		SetGasUsedByCallback(5)

	callGraph.AddNode("sc4", "cb7")

	return callGraph
}

// CreateGraphTestAsyncCallsCrossShard9 -
func CreateGraphTestAsyncCallsCrossShard9() *TestCallGraph {
	callGraph := CreateTestCallGraph()

	sc0f0 := callGraph.AddStartNode("sc0", "f0", 5500, 10)

	sc1f1 := callGraph.AddNode("sc1", "f1")
	callGraph.AddAsyncEdge(sc0f0, sc1f1, "cb0", "").
		SetGasLimit(5000).
		SetGasUsed(60).
		SetGasUsedByCallback(10)

	callGraph.AddNode("sc0", "cb0")

	sc4f4 := callGraph.AddNode("sc4", "f4")
	callGraph.AddSyncEdge(sc1f1, sc4f4).
		SetGasLimit(3000).
		SetGasUsed(25)

	sc5f5 := callGraph.AddNode("sc5", "f5")
	callGraph.AddAsyncCrossShardEdge(sc4f4, sc5f5, "cb4", "").
		SetGasLimit(2800).
		SetGasUsed(50).
		SetGasUsedByCallback(35)

	sc4cb4 := callGraph.AddNode("sc4", "cb4")

	sc6f6 := callGraph.AddNode("sc6", "f6")
	callGraph.AddAsyncCrossShardEdge(sc5f5, sc6f6, "cb5", "").
		SetGasLimit(100).
		SetGasUsed(10).
		SetGasUsedByCallback(2)

	callGraph.AddNode("sc5", "cb5")

	sc7f7 := callGraph.AddNode("sc7", "f7")
	callGraph.AddAsyncEdge(sc4cb4, sc7f7, "cb44", "").
		SetGasLimit(1500).
		SetGasUsed(45).
		SetGasUsedByCallback(5)

	callGraph.AddNode("sc4", "cb44")

	sc8f8 := callGraph.AddNode("sc8", "f8")
	callGraph.AddAsyncCrossShardEdge(sc7f7, sc8f8, "cb71", "").
		SetGasLimit(500).
		SetGasUsed(20).
		SetGasUsedByCallback(45)

	callGraph.AddNode("sc7", "cb71")

	sc9f9 := callGraph.AddNode("sc9", "f9")
	callGraph.AddAsyncCrossShardEdge(sc7f7, sc9f9, "cb72", "").
		SetGasLimit(600).
		SetGasUsed(10).
		SetGasUsedByCallback(55)

	callGraph.AddNode("sc7", "cb72")

	sc2f2 := callGraph.AddNode("sc2", "f2")
	callGraph.AddSyncEdge(sc1f1, sc2f2).
		SetGasLimit(1000).
		SetGasUsed(15)

	sc3f3 := callGraph.AddNode("sc3", "f3")
	callGraph.AddAsyncEdge(sc2f2, sc3f3, "cb2", "").
		SetGasLimit(700).
		SetGasUsed(40).
		SetGasUsedByCallback(5)

	callGraph.AddNode("sc2", "cb2")

	sc3f33 := callGraph.AddNode("sc3", "f33")
	callGraph.AddSyncEdge(sc3f3, sc3f33).
		SetGasLimit(300).
		SetGasUsed(100)

	return callGraph
}
