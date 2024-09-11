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

// TestReturnDataSuffix -
var TestReturnDataSuffix = "_returnData"

// ErrSyncCallFail -
var ErrSyncCallFail = fmt.Errorf("sync call fail")

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
	callGraph.AddSyncEdge(sc2f2, sc3f3).
		SetGasLimit(100).
		SetGasUsed(6)
	callGraph.AddNode("sc2", "cb2")

	return callGraph
}
