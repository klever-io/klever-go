package websocket

import "time"

const (
	// Ping interval to check client connection
	pingPeriod = time.Duration(15) * time.Second

	// chanel of client messages size
	outChannelSize = 500
)
