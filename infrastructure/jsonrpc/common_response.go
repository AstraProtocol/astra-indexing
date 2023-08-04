package jsonrpc

type CommonResp struct {
	Jsonrpc string      `json:"jsonrpc"`
	Id      uint        `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   interface{} `json:"error,omitempty"`
}
