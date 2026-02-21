package websocket

import "time"

const (
	pingPeriod     = time.Duration(15) * time.Second
	outChannelSize = 500
	maxWorkers     = 4
)
