package cosmosapp

type BlockInfo struct {
	BlockId interface{} `json:"block_id"`
	Block   BlockData   `json:"block"`
}

type BlockData struct {
	Header     interface{} `json:"header"`
	Data       RawTxs      `json:"data"`
	Evidence   interface{} `json:"evidence"`
	LastCommit interface{} `json:"last_commit"`
}

type RawTxs struct {
	Txs []string `json:"txs"`
}
