package vm

// CallType specifies the type of SC invocation (in terms of asynchronicity)
type CallType int

const (
	// DirectCall means that the call is an explicit SC invocation originating from a user Transaction
	DirectCall CallType = iota

	// KDATransferAndExecute means that there is a smart contract execution after the KDA transfer
	// this is needed in order to skip the check whether a contract is payable or not
	KDATransferAndExecute

	// ExecOnDestByCaller means that the call is an invocation of a built in function / smart contract from
	// another smart contract but the caller is from the previous caller
	ExecOnDestByCaller
)

const (
	DirectCallStr            = "directCall"
	KDATransferAndExecuteStr = "kdaTransferAndExecute"
	ExecOnDestByCallerStr    = "execOnDestByCaller"
	UnknownStr               = "unknown"
)

func (ct CallType) ToString() string {
	switch ct {
	case DirectCall:
		return DirectCallStr
	case KDATransferAndExecute:
		return KDATransferAndExecuteStr
	case ExecOnDestByCaller:
		return ExecOnDestByCallerStr
	default:
		return UnknownStr
	}
}
