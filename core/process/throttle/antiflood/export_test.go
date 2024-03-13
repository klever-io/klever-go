package antiflood

import "github.com/klever-io/klever-go/core/process"

func (af *p2pAntiflood) Debugger() process.AntifloodDebugger {
	return af.debugger
}
