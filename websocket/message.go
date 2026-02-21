package websocket

import "encoding/json"

const (
	MethodGetTransaction = "get_transaction"
	MethodGetBlock       = "get_block"
	MethodSubscribe      = "subscribe"
	MethodUnsubscribe    = "unsubscribe"
)

type WSRequest struct {
	ID     string          `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

type WSResponse struct {
	ID    string      `json:"id"`
	Data  interface{} `json:"data,omitempty"`
	Error string      `json:"error,omitempty"`
}

type GetTransactionParams struct {
	Hash        string `json:"hash"`
	WithResults bool   `json:"withResults"`
}

type GetBlockParams struct {
	Nonce   *uint64 `json:"nonce,omitempty"`
	Hash    string  `json:"hash,omitempty"`
	WithTxs bool    `json:"withTxs"`
}

type SubscribeParams struct {
	Addresses []string `json:"addresses"`
	Types     []string `json:"types"`
}

type UnsubscribeParams struct {
	Addresses []string `json:"addresses"`
	Types     []string `json:"types"`
}
