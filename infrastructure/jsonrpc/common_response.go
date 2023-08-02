package jsonrpc

type CommonResp struct {
	Jsonrpc string      `json:"jsonrpc"`
	Id      uint        `json:"id"`
	Result  interface{} `json:"result"`
}
