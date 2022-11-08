package utils

import "encoding/json"

type MsgEvmBase struct {
	Type    string        `json:"type"`
	Content MsgEvmContent `json:"content"`
}

type MsgEvmContent struct {
	Name    string       `json:"name"`
	Version int          `json:"version"`
	Params  MsgEvmParams `json:"params"`
}

type MsgEvmParams struct {
	Hash string `json:"hash"`
}

func ParseMsgEvmTx(tx_message string) MsgEvmBase {
	var result MsgEvmBase

	if err := json.Unmarshal([]byte(tx_message), &result); err != nil {
		return MsgEvmBase{}
	}

	return result
}
